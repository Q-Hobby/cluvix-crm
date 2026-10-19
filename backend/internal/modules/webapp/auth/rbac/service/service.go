package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/token"

	"gorm.io/gorm"
)

type RbacService struct {
	DB       *gorm.DB
	Cache    *cache.RedisCache
	TokenSvc *token.Service
}

const (
	cacheKeyUserPerm = "rbac:user:%d"
	cacheKeyUserRole = "rbac:role:%d"
	cacheTTLRbac     = 1 * time.Hour
)

// ErrRbacNameExists — name (PK) đã tồn tại khi tạo role/permission. Handler map → 422.
var ErrRbacNameExists = errors.New("rbac_name_exists")

// GetUserRole lookup role của user (1 user = 1 role). Cache key rbac:role:<uid>.
func (s *RbacService) GetUserRole(userID uint) (string, error) {
	key := fmt.Sprintf(cacheKeyUserRole, userID)
	if s.Cache != nil {
		var cached string
		if err := s.Cache.Get(key, &cached); err == nil && cached != "" {
			return cached, nil
		}
	}

	var role string
	err := s.DB.Table("rbac_auth_assignment a").
		Joins("JOIN rbac_auth_item i ON a.item_name = i.name AND i.type = 1").
		Where("a.user_id = ?", userID).
		Limit(1).
		Pluck("a.item_name", &role).Error
	if err != nil {
		return "", err
	}
	if role != "" && s.Cache != nil {
		_ = s.Cache.Set(key, role, cacheTTLRbac)
	}
	return role, nil
}

// IsSuperUser true nếu user có role thuộc define.SuperRoles.
func (s *RbacService) IsSuperUser(userID uint) bool {
	role, err := s.GetUserRole(userID)
	if err != nil {
		return false
	}
	return define.IsSuperRole(role)
}

// GetUserPermissions trả permission name list của user.
// Super role → return all rbac_auth_item.type=2.
// Normal → JOIN rbac_auth_assignment ⋈ rbac_auth_item_child ⋈ rbac_auth_item.
// Cache key rbac:user:<uid>, TTL 1h.
func (s *RbacService) GetUserPermissions(userID uint) ([]string, error) {
	key := fmt.Sprintf(cacheKeyUserPerm, userID)
	if s.Cache != nil {
		var cached []string
		if err := s.Cache.Get(key, &cached); err == nil && cached != nil {
			return cached, nil
		}
	}

	var perms []string
	if s.IsSuperUser(userID) {
		if err := s.DB.Table("rbac_auth_item").
			Where("type = 2").
			Pluck("name", &perms).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.DB.Table("rbac_auth_assignment a").
			Joins("JOIN rbac_auth_item_child c ON a.item_name = c.parent").
			Joins("JOIN rbac_auth_item i ON c.child = i.name AND i.type = 2").
			Where("a.user_id = ?", userID).
			Distinct("c.child").
			Pluck("c.child", &perms).Error; err != nil {
			return nil, err
		}
	}

	if perms == nil {
		perms = []string{}
	}
	if s.Cache != nil {
		_ = s.Cache.Set(key, perms, cacheTTLRbac)
	}
	return perms, nil
}

// InvalidateUserCache xoá cache role + perm của 1 user.
func (s *RbacService) InvalidateUserCache(userID uint) {
	if s.Cache == nil {
		return
	}
	s.Cache.Delete(fmt.Sprintf(cacheKeyUserPerm, userID))
	s.Cache.Delete(fmt.Sprintf(cacheKeyUserRole, userID))
}

// InvalidateRoleCache flush cache cho tất cả user gắn roleName.
func (s *RbacService) InvalidateRoleCache(roleName string) error {
	if s.Cache == nil {
		return nil
	}
	var userIDs []uint
	if err := s.DB.Table("rbac_auth_assignment").
		Where("item_name = ?", roleName).
		Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	for _, uid := range userIDs {
		s.InvalidateUserCache(uid)
	}
	return nil
}

// InvalidateAllSuperUsers flush cache cho tất cả user thuộc SuperRoles.
// Gọi sau khi thêm/xoá permission (super tự động có hết → cần refresh).
func (s *RbacService) InvalidateAllSuperUsers() error {
	if s.Cache == nil {
		return nil
	}
	var userIDs []uint
	if err := s.DB.Table("rbac_auth_assignment").
		Where("item_name IN ?", define.SuperRoles).
		Pluck("user_id", &userIDs).Error; err != nil {
		return err
	}
	for _, uid := range userIDs {
		s.InvalidateUserCache(uid)
	}
	return nil
}

// GetRolesDropdown returns roles for dropdown (exclude admin/develop/partner)
func (s *RbacService) GetRolesDropdown() ([]map[string]interface{}, error) {
	var roles []map[string]interface{}
	tx := s.DB.Table("rbac_auth_item").
		Select("name as value, description as label").
		Where("type = 1 AND name NOT IN ('admin', 'develop', 'partner')").
		Find(&roles)
	if pagination.IsQueryError(tx.Error) {
		return nil, tx.Error
	}
	return roles, nil
}

// GetPermissionsByRole returns permission names directly mapped to a role (editor list).
// CHỈ lấy child trực tiếp là permission (type=2) — KHỚP enforcement ở GetUserPermissions:
// runtime KHÔNG propagate quyền qua child-role, nên editor cũng chỉ hiển thị quyền trực tiếp,
// tránh tick nhầm quyền user thực tế không được cấp.
func (s *RbacService) GetPermissionsByRole(roleName string) []string {
	perms := make([]string, 0)
	s.DB.Table("rbac_auth_item_child c").
		Joins("JOIN rbac_auth_item i ON c.child = i.name AND i.type = 2").
		Where("c.parent = ?", roleName).
		Distinct("c.child").
		Pluck("c.child", &perms)
	return perms
}

// ErrUnknownPermissions — FE gửi quyền không có trong catalog `rbac_auth_item` (type=2).
// Handler map sang 422 kèm tên quyền; KHÔNG phải lỗi hệ thống nên không trả 500.
type ErrUnknownPermissions struct {
	Names []string
}

func (e *ErrUnknownPermissions) Error() string {
	return "quyền không có trong catalog: " + strings.Join(e.Names, ", ")
}

// UpdatePermissionWithRole batch update permissions for a role.
//
// ALL-OR-NOTHING: validate toàn bộ `toInsert` với catalog TRƯỚC khi chạm DB. Có bất kỳ quyền lạ
// nào → trả *ErrUnknownPermissions, KHÔNG apply phần nào (kể cả toDelete).
//
// Trước đây hàm này lặng lẽ `continue` bỏ qua quyền không hợp lệ rồi vẫn commit + trả nil ⇒ handler
// trả 200 "updated". FE (`user-role.ts` submit → subscribe next) khi đó set luôn baseline = lựa
// chọn hiện tại nên checkbox vẫn tick, `hasDiff` về false: admin tin là đã gán quyền trong khi DB
// không có gì. Kịch bản thật gặp mỗi lần FE deploy trước BE (matrix có quyền mới, catalog chưa seed
// — sự cố mobile/plan/41 MOB-109). Thà 422 nói rõ tên quyền còn hơn "thành công" giả.
func (s *RbacService) UpdatePermissionWithRole(roleName string, toDelete, toInsert []string) error {
	dedup := dedupStrings(toInsert)
	if len(dedup) > 0 {
		var valid []string
		if err := s.DB.Table("rbac_auth_item").
			Where("type = 2 AND name IN ?", dedup).
			Pluck("name", &valid).Error; err != nil {
			return err
		}
		if unknown := diffUnknown(dedup, valid); len(unknown) > 0 {
			return &ErrUnknownPermissions{Names: unknown}
		}
	}

	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if len(toDelete) > 0 {
		if err := tx.Exec("DELETE FROM rbac_auth_item_child WHERE parent = ? AND child IN ?", roleName, toDelete).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, child := range dedup {
		if err := tx.Exec("INSERT IGNORE INTO rbac_auth_item_child (parent, child) VALUES (?, ?)", roleName, child).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Load affected userIDs once — reused for revoke, cache invalidation, and auth version bump
	s.notifyRoleUsers(roleName)

	return nil
}

// diffUnknown trả phần tử của `want` không có trong `have`, giữ nguyên thứ tự `want`.
// Thuần (không DB) để test được nhánh "quyền lạ" mà không cần MySQL.
func diffUnknown(want, have []string) []string {
	set := make(map[string]bool, len(have))
	for _, v := range have {
		set[v] = true
	}
	var out []string
	for _, v := range want {
		if !set[v] {
			out = append(out, v)
		}
	}
	return out
}

// dedupStrings giữ nguyên thứ tự, bỏ phần tử rỗng và trùng lặp.
func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func (s *RbacService) notifyRoleUsers(roleName string) {
	var userIDs []uint
	s.DB.Table("rbac_auth_assignment").Where("item_name = ?", roleName).Pluck("user_id", &userIDs)
	for _, uid := range userIDs {
		if s.TokenSvc != nil {
			s.TokenSvc.RevokeAllSessions(uid)
		}
		s.InvalidateUserCache(uid)
		// user_scope cache giữ IsSuper — đổi role phải xóa, nếu không super-user vừa
		// bị hạ quyền vẫn bypass IP allowlist tới 5 phút (stale IsSuper).
		middleware.InvalidateUserScope(s.Cache, uid)
		middleware.BumpUserAuthVersion(s.Cache, uid)
	}
}

// invalidateCache wraps InvalidateRoleCache (giữ tên cũ cho backwards compat).
// BỎ s.Cache.DeletePattern("rbac*") cũ — quá aggressive, thay bằng targeted invalidate.
func (s *RbacService) invalidateCache(roleName string) {
	_ = s.InvalidateRoleCache(roleName)
}

// ListRoles returns paginated roles (type=1)
func (s *RbacService) ListRoles(keyword string) ([]map[string]interface{}, error) {
	return s.listItems(1, keyword)
}

// ListPermissions returns paginated permissions (type=2)
func (s *RbacService) ListPermissions(keyword string) ([]map[string]interface{}, error) {
	return s.listItems(2, keyword)
}

func (s *RbacService) listItems(itemType int, keyword string) ([]map[string]interface{}, error) {
	q := s.DB.Table("rbac_auth_item").Where("type = ?", itemType)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR description LIKE ? OR rule_name LIKE ?", like, like, like)
	}
	q = q.Order("created_at DESC")
	var rows []map[string]interface{}
	tx := q.Find(&rows)
	if pagination.IsQueryError(tx.Error) {
		return nil, tx.Error
	}
	return rows, nil
}

// GetRole returns a role by name
func (s *RbacService) GetRole(name string) (map[string]interface{}, error) {
	return s.getItem(name, 1)
}

// GetPermission returns a permission by name
func (s *RbacService) GetPermission(name string) (map[string]interface{}, error) {
	return s.getItem(name, 2)
}

func (s *RbacService) getItem(name string, itemType int) (map[string]interface{}, error) {
	var item map[string]interface{}
	if err := s.DB.Table("rbac_auth_item").Select("name, description").Where("name = ? AND type = ?", name, itemType).Limit(1).Scan(&item).Error; err != nil {
		return nil, errors.New("not_found")
	}
	if item == nil {
		return nil, errors.New("not_found")
	}
	return item, nil
}

// CreateRole creates a new role
func (s *RbacService) CreateRole(name, description string) error {
	return s.createItem(name, description, 1)
}

// CreatePermission creates a new permission. Sau khi tạo → invalidate cache super user
// (super có thêm permission mới ngay lập tức không cần thêm rbac_auth_item_child).
func (s *RbacService) CreatePermission(name, description string) error {
	// Best-effort cache invalidation — failure không ảnh hưởng business path.
	defer func() { _ = s.InvalidateAllSuperUsers() }()
	return s.createItem(name, description, 2)
}

func (s *RbacService) createItem(name, description string, itemType int) error {
	// PK là name (bất kể type) — pre-check để trả 422 thay vì 500 (MySQL 1062).
	var cnt int64
	s.DB.Table("rbac_auth_item").Where("name = ?", name).Count(&cnt)
	if cnt > 0 {
		return ErrRbacNameExists
	}
	now := fmt.Sprintf("%d", time.Now().Unix())
	return s.DB.Exec(
		"INSERT INTO rbac_auth_item (name, type, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		name, itemType, description, now, now,
	).Error
}

// UpdateRole updates a role's description
func (s *RbacService) UpdateRole(name string, updates map[string]interface{}) error {
	return s.updateItem(name, 1, updates)
}

// UpdatePermission updates a permission
func (s *RbacService) UpdatePermission(name string, updates map[string]interface{}) error {
	return s.updateItem(name, 2, updates)
}

func (s *RbacService) updateItem(name string, itemType int, updates map[string]interface{}) error {
	// Pre-check existence riêng — tránh "RowsAffected=0 do value không đổi" thành false NotFound.
	var cnt int64
	s.DB.Table("rbac_auth_item").Where("name = ? AND type = ?", name, itemType).Count(&cnt)
	if cnt == 0 {
		return errors.New("not_found")
	}
	return s.DB.Table("rbac_auth_item").Where("name = ? AND type = ?", name, itemType).Updates(updates).Error
}

// DeleteRole deletes a role + its children + assignments.
// notifyRoleUsers TRƯỚC khi xoá — revoke session/token của user đang gắn role này,
// tránh JWT/refresh còn sống sau khi role (và quyền) đã bị xoá khỏi DB.
func (s *RbacService) DeleteRole(name string) error {
	defer func() { _ = s.InvalidateRoleCache(name) }()
	s.notifyRoleUsers(name)
	return s.deleteItem(name, 1)
}

// DeletePermission deletes a permission + its children. Sau khi xoá → invalidate cache super.
func (s *RbacService) DeletePermission(name string) error {
	// Best-effort cache invalidation — failure không ảnh hưởng business path.
	defer func() { _ = s.InvalidateAllSuperUsers() }()
	return s.deleteItem(name, 2)
}

func (s *RbacService) deleteItem(name string, itemType int) error {
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM rbac_auth_item_child WHERE parent = ? OR child = ?", name, name).Error; err != nil {
			return fmt.Errorf("delete rbac child links: %w", err)
		}
		if itemType == 1 {
			if err := tx.Exec("DELETE FROM rbac_auth_assignment WHERE item_name = ?", name).Error; err != nil {
				return fmt.Errorf("delete rbac assignments: %w", err)
			}
		}
		if err := tx.Exec("DELETE FROM rbac_auth_item WHERE name = ? AND type = ?", name, itemType).Error; err != nil {
			return fmt.Errorf("delete rbac item: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.invalidateCache(name)
	return nil
}

// ── Sync ──

type SyncPermInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type SyncResult struct {
	Added   []string `json:"added"`
	Updated []string `json:"updated"`
	Total   int      `json:"total"`
}

// SyncPermissions upsert permission từ FE PERMISSION_MATRIX.
// KHÔNG xóa permission cũ — tránh mất role mapping nếu FE rename.
// Sau commit → invalidate cache super user (super tự có thêm perm mới).
func (s *RbacService) SyncPermissions(items []SyncPermInput) (*SyncResult, error) {
	result := &SyncResult{Added: []string{}, Updated: []string{}}
	if len(items) == 0 {
		return result, nil
	}

	// Pluck tất cả name bất kể type — PRIMARY KEY là name, nếu trùng dù khác type cũng fail INSERT
	var existing []string
	if err := s.DB.Table("rbac_auth_item").
		Pluck("name", &existing).Error; err != nil {
		return nil, err
	}
	existingSet := make(map[string]bool, len(existing))
	for _, n := range existing {
		existingSet[n] = true
	}

	now := time.Now().Unix()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		// Dedup items theo name (FE có thể gửi trùng) để tránh INSERT 2 lần cùng record.
		seen := make(map[string]bool, len(items))
		for _, p := range items {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true

			if existingSet[p.Name] {
				// Name đã tồn tại (bất kể type). Chỉ update khi type=2; type khác → no-op.
				if err := tx.Exec(
					"UPDATE rbac_auth_item SET description = ?, updated_at = ? WHERE name = ? AND type = 2",
					p.Description, now, p.Name,
				).Error; err != nil {
					return err
				}
				result.Updated = append(result.Updated, p.Name)
			} else {
				// INSERT IGNORE: tránh 1062 nếu name vừa được thread khác chèn (race) hoặc existingSet lệch.
				if err := tx.Exec(
					"INSERT IGNORE INTO rbac_auth_item (name, type, description, created_at, updated_at) VALUES (?, 2, ?, ?, ?)",
					p.Name, p.Description, now, now,
				).Error; err != nil {
					return err
				}
				result.Added = append(result.Added, p.Name)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Số record thực xử lý (đã dedup), khớp Added+Updated — không đếm item trùng trong input.
	result.Total = len(result.Added) + len(result.Updated)
	_ = s.InvalidateAllSuperUsers()
	return result, nil
}

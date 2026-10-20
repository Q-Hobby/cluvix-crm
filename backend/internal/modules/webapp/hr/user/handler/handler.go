package handler

import (
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"

	"gorm.io/gorm"

	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	hrUserDao "erpCluvix/internal/modules/webapp/hr/user/dao"
	svc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

// Deps — full Deps (DB + UserSvc + DownloadURL) cho HR modules. user module
// dùng UserSvc, payroll module dùng DownloadURL, các module khác chỉ DB —
// giữ chung Deps để consistency.
type Deps struct {
	DB          *gorm.DB
	UserSvc     *hrSvc.UserService
	RbacSvc     *authSvc.RbacService
	DownloadURL string
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// hasPermission check user hiện tại có permission code không (Redis-cached qua RbacSvc; super-role
// tự có toàn bộ perm).
//
// Guard này KHÔNG thừa dù đã có PermissionEnforcer: middleware đang chạy audit-mode
// (PERM_ENFORCE=false trong .env) nên chỉ ghi log would-deny chứ không trả 403. Cho tới khi bật
// enforce thật, chặn ở FE (*hasPerm) là chặn ở phía client — gọi thẳng API vẫn qua.
func (h *Handler) hasPermission(userID uint, code string) bool {
	if h.RbacSvc == nil {
		return false
	}
	perms, err := h.RbacSvc.GetUserPermissions(userID)
	if err != nil {
		return false
	}
	for _, p := range perms {
		if p == code {
			return true
		}
	}
	return false
}

// BuildUserQuery builds the user list query with all filters applied
func (h *Handler) BuildUserQuery(c *gin.Context) *gorm.DB {
	q := h.DB.
		Table("user u").
		Select(`u.id, u.id_old, u.username, u.status, u.branchId, u.allowed_ips,
			u.created_at, u.updated_at, u.created_by, u.updated_by,
			p.user_id, p.fullname, p.nickname, p.phone, p.avatar, p.email,
			p.code, p.birthday, p.gender, p.departmentList, p.about, p.address,
			p.id_pancake, p.facebook, p.telegram, p.cover, p.locale,
			p.branchOffice_id, p.branchActive_id, p.updated_at as profile_updated_at,
			MIN(i.name) as role_name, MIN(i.description) as role_description`).
		Joins("LEFT JOIN user_profile p ON p.user_id = u.id").
		Joins("LEFT JOIN rbac_auth_assignment a ON a.user_id = u.id").
		Joins("LEFT JOIN rbac_auth_item i ON i.name = a.item_name").
		Where("u.id <> 1")

	// Story-58 (E6/D15, 1 thương hiệu/DB): bỏ lọc JSON `u.companyId` theo company đang active —
	// mọi nhân viên trong DB thuộc thương hiệu duy nhất. Guard IDOR của Update/Delete/đổi mật khẩu
	// (đi qua chính query này) nay là: tồn tại + `u.id <> 1` + targetIsProtectedSuper (xem dưới).

	q = query.ApplyKeywordFilter(c, q, "u.username", "p.fullname", "p.phone")
	q = query.ApplyFilter(c, q, "status", "u.status")
	q = query.ApplyJSONContainsFilter(c, q, "branch_id", "u.branchId")
	q = query.ApplyFilter(c, q, "_role", "a.item_name")
	q = query.ApplyJSONContainsFilter(c, q, "_department", "p.departmentList")

	// GROUP BY u.id — BẮT BUỘC, không phải tối ưu.
	//
	// PK của `rbac_auth_assignment` là (user_id, item_name) ⇒ schema CHO PHÉP 1 user nhiều vai trò.
	// Khi đó LEFT JOIN nhân dòng: user hiện 2 lần trong danh sách, totalCount đếm sai, và file export
	// ra 2 dòng CÙNG id — import ngược thì dòng sau ghi đè dòng trước, mất im lặng 1 vai trò.
	// Gom về 1 dòng/user + MIN(i.name) cho giá trị xác định (response `userRole` vốn là 1 object,
	// toàn hệ thống — form, import, mobile DTO — đều là mô hình 1 vai trò).
	//
	// Filter `_role` vẫn đúng ngữ nghĩa "user có vai trò trong danh sách" vì WHERE lọc trên dòng đã
	// JOIN, TRƯỚC khi gom nhóm.
	//
	// An toàn với ONLY_FULL_GROUP_BY (đã chạy thử trên MySQL 9.7): các cột p.* phụ thuộc hàm vào u.id
	// do `p.user_id` là PRIMARY KEY của user_profile và join bằng đẳng thức p.user_id = u.id.
	// `Paginate` và `pkg/export` đều bọc subquery (`SELECT COUNT(*) FROM (?)`) nên đếm đúng số NHÓM.
	return q.Group("u.id").Order("u.id DESC")
}

// ListUser GET /hr/user
func (h *Handler) ListUser(c *gin.Context) {
	q := h.BuildUserQuery(c)

	if c.Query("_export") == "1" {
		// File export chở trọn danh sách nhân sự (tài khoản, SĐT, email, vai trò) — cùng quyền với
		// nút Export trên FE (*hasPerm="extUser"), chặn thật ở đây vì middleware đang audit-mode.
		if !h.hasPermission(middleware.GetUserID(c), define.PermExtUser) {
			response.Forbidden(c)
			return
		}
		exportHandlers.ExportUsers(c, q)
		return
	}
	var items []map[string]interface{}
	result := pagination.Paginate(c, q, &items)
	storage.ResolveRows(items, "avatar") // avatar lưu KEY kho public ⇒ ghép URL

	// Nest userProfile + userRole to match Yii2 response
	nested := make([]gin.H, 0, len(items))
	for _, row := range items {
		user := gin.H{
			"id":         row["id"],
			"username":   row["username"],
			"status":     row["status"],
			"branchId":   row["branchId"],
			"created_at": row["created_at"],
			"updated_at": row["updated_at"],
			"created_by": row["created_by"],
			"updated_by": row["updated_by"],
			"userProfile": gin.H{
				"user_id":         row["user_id"],
				"fullname":        row["fullname"],
				"nickname":        row["nickname"],
				"code":            row["code"],
				"phone":           row["phone"],
				"email":           row["email"],
				"avatar":          row["avatar"],
				"cover":           row["cover"],
				"birthday":        row["birthday"],
				"gender":          row["gender"],
				"about":           row["about"],
				"address":         row["address"],
				"locale":          row["locale"],
				"id_pancake":      row["id_pancake"],
				"facebook":        row["facebook"],
				"telegram":        row["telegram"],
				"departmentList":  row["departmentList"],
				"branchOffice_id": row["branchOffice_id"],
				"branchActive_id": row["branchActive_id"],
				"updated_at":      row["profile_updated_at"],
			},
		}
		if rn, ok := row["role_name"]; ok && rn != nil {
			user["userRole"] = gin.H{"name": rn, "description": row["role_description"]}
		}
		nested = append(nested, user)
	}
	result.Items = nested
	parseJSONInItems(result.Items)
	c.JSON(http.StatusOK, result)
}

// GetUser GET /hr/user/:id
func (h *Handler) GetUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// Use BuildUserQuery to enforce tenant scope (same as list)
	var items []map[string]interface{}
	tx := h.BuildUserQuery(c).Where("u.id = ?", uint(id)).Limit(1).Find(&items)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	if len(items) == 0 {
		response.NotFound(c)
		return
	}
	helpers.ParseJSONFieldsForTables(items, "user")
	storage.ResolveRows(items, "avatar") // avatar lưu KEY kho public ⇒ ghép URL

	row := items[0]
	user := gin.H{
		"id":          row["id"],
		"username":    row["username"],
		"status":      row["status"],
		"branchId":    row["branchId"],
		"allowed_ips": row["allowed_ips"],
		"created_at":  row["created_at"],
		"updated_at":  row["updated_at"],
		"created_by":  row["created_by"],
		"updated_by":  row["updated_by"],
		"userProfile": gin.H{
			"user_id":         row["user_id"],
			"fullname":        row["fullname"],
			"nickname":        row["nickname"],
			"code":            row["code"],
			"phone":           row["phone"],
			"email":           row["email"],
			"avatar":          row["avatar"],
			"cover":           row["cover"],
			"birthday":        row["birthday"],
			"gender":          row["gender"],
			"about":           row["about"],
			"address":         row["address"],
			"locale":          row["locale"],
			"id_pancake":      row["id_pancake"],
			"facebook":        row["facebook"],
			"telegram":        row["telegram"],
			"departmentList":  row["departmentList"],
			"branchOffice_id": row["branchOffice_id"],
			"branchActive_id": row["branchActive_id"],
			"updated_at":      row["profile_updated_at"],
		},
	}
	if rn, ok := row["role_name"]; ok && rn != nil {
		user["userRole"] = gin.H{"name": rn, "description": row["role_description"]}
	}
	parseGinH(user)
	if profile, ok := user["userProfile"].(gin.H); ok {
		parseGinH(profile)
	}
	c.JSON(http.StatusOK, user)
}

// sameAsCurrent — giá trị file có TRÙNG giá trị đang lưu không.
//
// Cột mảng JSON (branchId/departmentList) so sau khi bỏ khoảng trắng vì MySQL trả
// "[5, 3, 2]" còn người gõ tay ra "[5,3,2]" — cùng một giá trị.
func sameAsCurrent(col string, fileVal interface{}, current map[string]string) bool {
	if current == nil {
		return false
	}
	cur, ok := current[col]
	if !ok {
		return false
	}
	f := fmt.Sprintf("%v", fileVal)
	switch col {
	case "branchId", "departmentList":
		return normalizeJSONIDs(f) == normalizeJSONIDs(cur)
	default:
		return f == cur
	}
}

// normalizeJSONIDs bỏ mọi khoảng trắng để so sánh mảng id dạng JSON giữa file và DB.
// MySQL trả `[5, 3, 2]` (có space), file export chở nguyên chuỗi đó, còn người dùng gõ tay thì
// không space — so chuỗi thô sẽ báo "đã đổi" một cách giả tạo.
func normalizeJSONIDs(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// jsonIDsChanged — file có THẬT SỰ đổi mảng id so với giá trị đang lưu không.
//
// Đảo thứ tự tính là đổi — cố ý chặt tay: nghi ngờ thì validate, không bỏ qua.
func jsonIDsChanged(fileVal, currentVal string) bool {
	return normalizeJSONIDs(fileVal) != normalizeJSONIDs(currentVal)
}

// jsonIDsAllowed kiểm tra mảng JSON id (branchId) nhận từ payload chỉ chứa phần tử ⊆ allowed
// (scope caller). allowed rỗng = caller không bị giới hạn. raw rỗng/nil = không đổi field → cho qua.
func jsonIDsAllowed(raw json.RawMessage, allowed []uint) bool {
	if len(raw) == 0 || len(allowed) == 0 {
		return true
	}
	var ids []uint64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return false
	}
	allowedSet := make(map[uint64]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[uint64(a)] = true
	}
	for _, id := range ids {
		if !allowedSet[id] {
			return false
		}
	}
	return true
}

// targetIsProtectedSuper — [S2-3] thay `targetHasNullCompany` (story-58).
//
// Trước: user `companyId` NULL = tài khoản cấp nền tảng (thấy được ở MỌI thương hiệu qua
// NULL-fallback) ⇒ write nhạy cảm (update/xoá/đổi-reset mật khẩu/import) chỉ super-user làm được —
// chặn chiếm tài khoản nền tảng qua reset mật khẩu. Bỏ company thì "NULL company" không còn nghĩa;
// điều kiện TƯƠNG ĐƯƠNG theo QUYỀN: user đích mang vai trò super (define.SuperRoles) ⇒ chỉ
// super-user mới được sửa. Fail-closed: lỗi DB khi kiểm ⇒ coi như được bảo vệ (chặn).
func (h *Handler) targetIsProtectedSuper(id uint) bool {
	isSuper, err := helpers.IsSuperUserE(h.DB, id)
	if err != nil {
		return true
	}
	return isSuper
}

// sameAllowedIPs: `raw` client gửi có cùng danh sách với allowed_ips đang lưu của user không
// (null/rỗng/[] coi như nhau). Lỗi đọc/parse ⇒ false (fail-closed: coi như đổi).
func (h *Handler) sameAllowedIPs(userID uint, raw []byte) bool {
	var current *string
	if err := h.DB.Table("user").Select("allowed_ips").Where("id = ?", userID).Scan(&current).Error; err != nil {
		return false
	}
	parse := func(b []byte) ([]string, bool) {
		s := strings.TrimSpace(string(b))
		if s == "" || s == "null" {
			return nil, true
		}
		var list []string
		if err := json.Unmarshal(b, &list); err != nil {
			return nil, false
		}
		return list, true
	}
	var cur []byte
	if current != nil {
		cur = []byte(*current)
	}
	a, okA := parse(cur)
	b, okB := parse(raw)
	if !okA || !okB || len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}

// roleEscalationBlocked trả true nếu role nằm trong define.SuperRoles (admin/develop) và
// caller không phải super-user — chặn tự nâng quyền qua Create/Update/Import (xem [H2]).
func roleEscalationBlocked(role string, callerIsSuper bool) bool {
	return role != "" && define.IsSuperRole(role) && !callerIsSuper
}

// CreateUser POST /hr/user
func (h *Handler) CreateUser(c *gin.Context) {
	var input svc.CreateUserInput
	if err := c.ShouldBind(&input); err != nil {
		response.ValidateErr(c, err)
		return
	}
	// `companyId` trong payload (FE cũ) bị BỎ QUA — DTO không có field, bảng không còn cột.
	if !jsonIDsAllowed(input.BranchId, middleware.GetBranchIDs(c)) {
		response.Validate(c, i18n.T(c, "branch_id_not_allowed"))
		return
	}
	callerID := middleware.GetUserID(c)
	if roleEscalationBlocked(input.Role, helpers.IsSuperUser(h.DB, callerID)) {
		response.Validate(c, i18n.T(c, "role_not_allowed"))
		return
	}
	if !helpers.ValidateIPOrCIDRList(input.AllowedIps) {
		response.Validate(c, i18n.T(c, "allowed_ips_invalid"))
		return
	}

	user, err := h.UserSvc.CreateUser(input, callerID)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	response.Created(c, user)
}

// UpdateUser PUT /hr/user/:id
func (h *Handler) UpdateUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// Verify the target user is within the caller's tenant scope
	var count int64
	h.BuildUserQuery(c).Where("u.id = ?", uint(id)).Count(&count)
	if count == 0 {
		response.NotFound(c)
		return
	}

	callerID := middleware.GetUserID(c)
	callerIsSuper := helpers.IsSuperUser(h.DB, callerID)
	// [S2-3] write nhạy cảm trên tài khoản super chỉ super-user được phép.
	if h.targetIsProtectedSuper(uint(id)) && !callerIsSuper {
		response.Forbidden(c)
		return
	}

	var input svc.UpdateUserInput
	if err := c.ShouldBind(&input); err != nil {
		response.ValidateErr(c, err)
		return
	}
	// Story-58: `companyId` trong payload bị BỎ QUA (DTO không còn field) — không đổi cột.
	if !jsonIDsAllowed(input.BranchId, middleware.GetBranchIDs(c)) {
		response.Validate(c, i18n.T(c, "branch_id_not_allowed"))
		return
	}
	if input.Role != nil && roleEscalationBlocked(*input.Role, callerIsSuper) {
		response.Validate(c, i18n.T(c, "role_not_allowed"))
		return
	}
	if !helpers.ValidateIPOrCIDRList(input.AllowedIps) {
		response.Validate(c, i18n.T(c, "allowed_ips_invalid"))
		return
	}
	// Allowlist IP riêng của user GHI ĐÈ allowlist thương hiệu ⇒ tự sửa của chính mình = tự gỡ
	// giới hạn IP. Chỉ super-user được; FE gửi lại nguyên giá trị cũ thì vẫn cho qua.
	if uint(id) == callerID && !callerIsSuper && input.AllowedIps != nil && !h.sameAllowedIPs(uint(id), input.AllowedIps) {
		response.Validate(c, i18n.T(c, "self_allowed_ips_not_allowed"))
		return
	}

	user, err := h.UserSvc.UpdateUser(uint(id), input, callerID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.DataWithMessage(c, user, i18n.T(c, "user_updated"))
}

// DeleteUser DELETE /hr/user/:id
func (h *Handler) DeleteUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)

	// Verify the target user is within the caller's tenant scope
	var count int64
	h.BuildUserQuery(c).Where("u.id = ?", uint(id)).Count(&count)
	if count == 0 {
		response.NotFound(c)
		return
	}
	// [S2-3] write nhạy cảm trên tài khoản super chỉ super-user được phép.
	if h.targetIsProtectedSuper(uint(id)) && !helpers.IsSuperUser(h.DB, middleware.GetUserID(c)) {
		response.Forbidden(c)
		return
	}

	if err := h.UserSvc.DeleteUser(uint(id), middleware.GetUserID(c)); err != nil {
		response.HandleError(c, err)
		return
	}
	response.Deleted(c)
}

// ChangePassword POST /hr/user/change-password
func (h *Handler) ChangePassword(c *gin.Context) {
	var body struct {
		ID       uint   `json:"id"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidateErr(c, err)
		return
	}
	if body.Password == "" || len(body.Password) < 6 {
		response.Validate(c, i18n.T(c, "password_min"))
		return
	}
	// Verify target user belongs to caller's tenant scope (prevent IDOR)
	var exists int64
	h.BuildUserQuery(c).Where("u.id = ?", body.ID).Count(&exists)
	if exists == 0 {
		response.NotFound(c)
		return
	}
	// [S2-3] write nhạy cảm trên tài khoản super chỉ super-user được phép.
	if h.targetIsProtectedSuper(body.ID) && !helpers.IsSuperUser(h.DB, middleware.GetUserID(c)) {
		response.Forbidden(c)
		return
	}
	// Đổi hộ người khác ⇒ chủ tài khoản phải đổi lại ở lần đăng nhập kế (S-4/D-1); tự đổi cho
	// chính mình qua endpoint này ⇒ không ép.
	if err := h.UserSvc.ChangePassword(body.ID, body.Password, body.ID != middleware.GetUserID(c)); err != nil {
		response.ServerError(c, err)
		return
	}
	response.DataWithMessage(c, nil, i18n.T(c, "password_changed"))
}

// UpdatePassword POST /hr/user/update-password – user tự đổi mật khẩu
func (h *Handler) UpdatePassword(c *gin.Context) {
	var body struct {
		OldPassword       string `json:"oldpassword"`
		NewPassword       string `json:"newpassword"`
		NewPasswordRepeat string `json:"newpasswordrepeat"`
		IsSecurity        bool   `json:"isSecurity"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidateErr(c, err)
		return
	}
	if body.NewPassword == "" || len(body.NewPassword) < 6 {
		response.Validate(c, i18n.T(c, "password_min"))
		return
	}
	if body.NewPassword != body.NewPasswordRepeat {
		response.Validate(c, i18n.T(c, "password_confirm_mismatch"))
		return
	}

	userID := middleware.GetUserID(c)
	// Verify mật khẩu cũ
	var user hrUserDao.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.OldPassword)); err != nil {
		response.Validate(c, i18n.T(c, "password_old_wrong"))
		return
	}
	// Mật khẩu mới trùng mật khẩu hiện tại ⇒ từ chối: nếu không, user bị ép đổi (S-4/D-1, vd admin
	// seed mật khẩu mặc định công khai) "đổi" sang chính nó là gỡ được cờ mà mật khẩu vẫn y nguyên.
	if body.NewPassword == body.OldPassword {
		response.Validate(c, i18n.T(c, "password_same_as_old"))
		return
	}

	// ChangePassword hash + update DB + RevokeAllSessions (bắt buộc revoke tất cả session cũ).
	// Tự đổi ⇒ gỡ cờ must_change_password.
	if err := h.UserSvc.ChangePassword(userID, body.NewPassword, false); err != nil {
		response.ServerError(c, err)
		return
	}
	response.DataWithMessage(c, nil, i18n.T(c, "password_changed"))
}

// ResetPassword PUT /hr/user/reset-password
func (h *Handler) ResetPassword(c *gin.Context) {
	var body struct {
		ID       uint   `json:"id"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ID == 0 {
		response.Validate(c, i18n.TF(c, "required", i18n.FieldLabel(c, "id")))
		return
	}
	if len(body.Password) < 6 {
		response.Validate(c, i18n.T(c, "password_min"))
		return
	}
	// Verify target user belongs to caller's tenant scope (prevent IDOR)
	var exists int64
	h.BuildUserQuery(c).Where("u.id = ?", body.ID).Count(&exists)
	if exists == 0 {
		response.NotFound(c)
		return
	}
	// [S2-3] write nhạy cảm trên tài khoản super chỉ super-user được phép.
	if h.targetIsProtectedSuper(body.ID) && !helpers.IsSuperUser(h.DB, middleware.GetUserID(c)) {
		response.Forbidden(c)
		return
	}
	// Đặt lại bởi người khác ⇒ ép chủ tài khoản đổi ở lần đăng nhập kế (S-4/D-1, thông lệ);
	// tự đặt lại cho chính mình ⇒ không ép.
	if err := h.UserSvc.ChangePassword(body.ID, body.Password, body.ID != middleware.GetUserID(c)); err != nil {
		response.HandleError(c, err)
		return
	}
	response.Updated(c, nil)
}

// GetAllUsers GET /hr/user/get-all-users
func (h *Handler) GetAllUsers(c *gin.Context) {
	data, err := h.UserSvc.GetAllUsers()
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

// RolesDropdown GET /hr/user/roles-dropdown
func (h *Handler) RolesDropdown(c *gin.Context) {
	data, err := h.UserSvc.GetRolesDropdown()
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

// parseJSONInItems converts JSON string fields to json.RawMessage in nested gin.H
func parseJSONInItems(items interface{}) {
	rows, ok := items.([]gin.H)
	if !ok {
		return
	}
	for _, row := range rows {
		parseGinH(row)
		if profile, ok := row["userProfile"].(gin.H); ok {
			parseGinH(profile)
		}
	}
}

func parseGinH(m gin.H) {
	for k, v := range m {
		s, ok := v.(string)
		if !ok || len(s) < 2 {
			continue
		}
		if (s[0] == '[' && s[len(s)-1] == ']') || (s[0] == '{' && s[len(s)-1] == '}') {
			m[k] = json.RawMessage(s)
		}
	}
}

// ImportUsers POST /hr/user/import-users
//
// File nhận vào là chính file bấm Export ra: 2 dòng header (dòng 1 nhãn tiếng Việt, dòng 2 tên cột
// DB), dữ liệu từ dòng 3. File cũ chỉ có dòng nhãn vẫn chạy — xem resolveUserImportColumns.
//
// Map nhãn tiếng Việt → tên field DB. "Ngày sinh" (birthday) chủ động KHÔNG map vào đây dù có ở
// form nhân sự: cần thêm thì phải parse bằng pkg/exceldate — xem
// skill/skills/debugging-backend-queries/reference/patterns-bulk-import.md §6
// (bug ngày ≤12 bị Excel serialize).
var userHeaderToCol = map[string]string{
	// "id_old" giữ lại cho file export đời cũ (nhãn dòng 1 từng chính là tên cột DB).
	"id": "id", "id_old": "id_old", "id cũ": "id_old",
	"Mã chấm công": "code", "Nhân viên": "fullname", "Bí danh": "nickname",
	"Tài khoản": "username", "Mật khẩu": "password", "Giới tính": "gender", "Trạng thái": "status",
	"Số điện thoại": "phone", "Email": "email", "Vai trò": "role",
	"List phòng ban": "departmentList", "List chi nhánh": "branchId",
	// Story-58 (D23): bỏ cột "List thương hiệu" — file cũ còn cột này thì bị bỏ qua như cột lạ.
}

// userImportValidCols — tập tên cột DB hợp lệ ở dòng header thứ 2, suy từ chính userHeaderToCol để
// không phải nuôi 2 danh sách song song.
var userImportValidCols = func() map[string]bool {
	m := make(map[string]bool, len(userHeaderToCol))
	for _, col := range userHeaderToCol {
		m[col] = true
	}
	return m
}()

// resolveUserImportColumns nhận cột từ header, trả về (index cột → tên field DB), số dòng header
// phải bỏ qua, và tên cột bị lặp (nếu có).
//
// Ưu tiên dòng 2 (tên cột DB) vì đó là dòng máy đọc — user hay sửa/dịch nhãn dòng 1. File export
// đời cũ chỉ có 1 dòng nhãn thì rơi về dòng 1. Trả cột lặp thay vì im lặng lấy cột cuối: 2 cột cùng
// trỏ 1 field là file hỏng, ghi bừa vào DB nguy hiểm hơn nhiều so với báo lỗi.
func resolveUserImportColumns(rows [][]string) (colIndex map[int]string, dataStart int, dupCol string) {
	byLabel := func(line []string) map[int]string {
		out := map[int]string{}
		for j, h := range line {
			if col, ok := userHeaderToCol[strings.TrimSpace(h)]; ok {
				out[j] = col
			}
		}
		return out
	}
	byKey := func(line []string) map[int]string {
		out := map[int]string{}
		for j, h := range line {
			if col := strings.TrimSpace(h); userImportValidCols[col] {
				out[j] = col
			}
		}
		return out
	}

	labelCols := byLabel(rows[0])
	var keyCols map[int]string
	if len(rows) > 1 {
		keyCols = byKey(rows[1])
	}
	switch {
	case len(keyCols) >= 2 && len(keyCols) >= len(labelCols):
		colIndex, dataStart = keyCols, 2
	default:
		// Dòng 1 có thể chính là dòng field key (user xoá dòng nhãn). Chỉ nhận khi nó khớp field key
		// nhiều hơn hẳn cách đọc theo nhãn — tránh cướp mất file template cũ hợp lệ.
		if row0Keys := byKey(rows[0]); len(row0Keys) >= 2 && len(row0Keys) > len(labelCols) {
			colIndex, dataStart = row0Keys, 1
		} else {
			colIndex, dataStart = labelCols, 1
		}
	}

	idxs := make([]int, 0, len(colIndex))
	for j := range colIndex {
		idxs = append(idxs, j)
	}
	sort.Ints(idxs) // báo lỗi phải ổn định: cùng file luôn ra cùng tên cột lặp
	seen := map[string]bool{}
	for _, j := range idxs {
		if seen[colIndex[j]] {
			return colIndex, dataStart, colIndex[j]
		}
		seen[colIndex[j]] = true
	}
	return colIndex, dataStart, ""
}

// userRowData dựng map field → giá trị của 1 dòng, chỉ lấy ô không rỗng.
//
// Unsanitize là BẮT BUỘC: export chèn dấu nháy đơn vào đầu chuỗi bắt đầu bằng = + - @ (chống công
// thức Excel). Không bóc thì bí danh "-Anh" quay lại DB thành "'-Anh", mỗi vòng export→import chồng
// thêm một dấu.
func userRowData(row []string, colIndex map[int]string) map[string]interface{} {
	data := map[string]interface{}{}
	for j, col := range colIndex {
		if j >= len(row) {
			continue
		}
		// TrimSpace TRƯỚC: Unsanitize chỉ bóc khi ký tự ĐẦU TIÊN là nháy đơn, ô " '-Anh" (Excel/user
		// chèn khoảng trắng) mà làm ngược thứ tự thì dấu nháy chui thẳng vào DB.
		if v := export.Unsanitize(strings.TrimSpace(row[j])); v != "" {
			data[col] = v
		}
	}
	return data
}

// maxUserImportRows — trần số dòng dữ liệu 1 file import.
// Không có trần thì 1 file lỡ tay (hoặc cố ý) đủ để giữ transaction hàng chục phút. Cùng giá trị với
// import dịch vụ để user gặp một luật duy nhất.
const maxUserImportRows = 5000

// newUserPassword — mật khẩu của 1 dòng tạo mới, băm sẵn ngoài transaction.
// `plain` chỉ sống trong 1 request để trả về cho người import chép lại, KHÔNG ghi xuống đâu cả.
type newUserPassword struct {
	plain string
	hash  string
}

func (h *Handler) ImportUsers(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Validate(c, i18n.T(c, "file_required"))
		return
	}
	// Đo thời gian theo PHA. Endpoint bulk chạy hàng nghìn dòng nên "chậm" là chuyện sẽ tái diễn;
	// không có mốc theo pha thì mỗi lần lại phải suy đoán lại từ đầu. Chỉ in khi thật sự chậm
	// (>2s) nên bình thường im lặng. stderr chứ không logger.Warn (zap pin ErrorLevel, nuốt câm)
	// và không logger.Error (bắn Telegram — đây là số liệu, không phải sự cố).
	tStart := time.Now()
	var tRead, tPrescan, tPreload, tLoop, tCommit time.Duration
	defer func() {
		total := time.Since(tStart)
		if total < 2*time.Second {
			return
		}
		fmt.Fprintf(os.Stderr,
			"[import-users] TỔNG %v | đọc file %v | pre-scan(+bcrypt) %v | pre-load %v | vòng ghi %v | commit+cache %v\n",
			total.Round(time.Millisecond), tRead.Round(time.Millisecond), tPrescan.Round(time.Millisecond),
			tPreload.Round(time.Millisecond), tLoop.Round(time.Millisecond), tCommit.Round(time.Millisecond))
	}()

	// ReadImportRows: đọc RawCellValue (mã chấm công 0012 không bị đọc thành "12") và nhận cả .csv —
	// export >= CSVThreshold dòng ra .csv, thiếu nhánh này thì round-trip đứt đúng lúc danh sách
	// nhân sự lớn nhất, mà thông báo lỗi lại là "đọc file Excel thất bại".
	rows, err := export.ReadImportRows(file)
	tRead = time.Since(tStart)
	if err != nil {
		response.Validate(c, i18n.TF(c, "excel_read_failed", err.Error()))
		return
	}
	if len(rows) < 2 {
		response.Validate(c, i18n.T(c, "no_data"))
		return
	}

	colIndex, dataStart, dupCol := resolveUserImportColumns(rows)
	if len(colIndex) == 0 {
		response.Validate(c, i18n.T(c, "import_no_recognized_column"))
		return
	}
	if dupCol != "" {
		response.Validate(c, i18n.TF(c, "import_duplicate_column", dupCol))
		return
	}
	if len(rows) <= dataStart {
		response.Validate(c, i18n.T(c, "no_data"))
		return
	}
	if len(rows)-dataStart > maxUserImportRows {
		response.Validate(c, i18n.TF(c, "import_too_many_rows", maxUserImportRows))
		return
	}

	userID := middleware.GetUserID(c)

	// Guard quyền theo đúng thao tác file yêu cầu (cùng khuôn ImportServiceFromExcel).
	// `extUser` = quyền bấm nút Import/Export nhân sự trên FE; thêm create/update tuỳ file có dòng
	// tạo mới hay dòng cập nhật, để user chỉ có quyền sửa không dùng file mà tạo được tài khoản.
	// Bắt buộc chặn ở đây: middleware permission đang audit-mode, không tự trả 403.
	if !h.hasPermission(userID, define.PermExtUser) {
		response.Forbidden(c)
		return
	}
	callerIsSuper := helpers.IsSuperUser(h.DB, userID)
	now := time.Now().Unix()
	uid := int(userID)

	type createdUserInfo struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var createdUsers []createdUserInfo

	// Pre-scan rows to batch-load phone ownership and profile existence (avoid N+1),
	// đồng thời BĂM MẬT KHẨU TRƯỚC KHI MỞ TRANSACTION.
	//
	// bcrypt cost 12 ≈ 300ms/dòng. Băm bên trong `tx` nghĩa là file 1000 nhân viên mới giữ lock ghi
	// trên user/user_profile/rbac_auth_assignment suốt ~5 phút — cả hệ thống không tạo/sửa nhân sự
	// được, và proxy timeout thì rollback sạch, user không hiểu vì sao. CPU vẫn tốn như cũ, chỉ là
	// tốn ngoài transaction.
	var phonesInFile []string
	var updateIDsInFile []uint
	newPasswords := make(map[int]newUserPassword, len(rows)-dataStart)
	for i, row := range rows[dataStart:] {
		if allEmptyRow(row) {
			continue
		}
		data := userRowData(row, colIndex)
		if idVal, hasID := data["id"]; hasID {
			var pid uint
			if _, e := fmt.Sscanf(fmt.Sprintf("%v", idVal), "%d", &pid); e == nil && pid > 0 {
				updateIDsInFile = append(updateIDsInFile, pid)
			}
		} else if len(data) > 0 {
			// Dòng tạo mới: sinh + băm ngay tại đây. Dòng lỗi sau này có băm thừa cũng chấp nhận —
			// đổi lại transaction chỉ còn thuần I/O.
			plain, _ := data["password"].(string)
			if plain == "" {
				plain = helpers.RandomString(8)
			}
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(plain), 12)
			if hashErr != nil {
				response.ServerError(c, hashErr)
				return
			}
			newPasswords[i] = newUserPassword{plain: plain, hash: string(hash)}
		}
		if ph, ok := data["phone"].(string); ok && ph != "" {
			phonesInFile = append(phonesInFile, ph)
		}
	}

	tPrescan = time.Since(tStart) - tRead

	var phonePreRows []struct {
		UserID uint   `gorm:"column:user_id"`
		Phone  string `gorm:"column:phone"`
	}
	if len(phonesInFile) > 0 {
		// Lỗi ở đây mà bỏ qua thì phoneOwner rỗng ⇒ SĐT đang thuộc người khác vẫn bị ghi đè.
		tx := h.DB.Table("user_profile").Select("user_id, phone").Where("phone IN ?", phonesInFile).Find(&phonePreRows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
	}
	phoneOwner := make(map[string]uint, len(phonePreRows))
	for _, pr := range phonePreRows {
		phoneOwner[pr.Phone] = pr.UserID
	}

	var pidPreRows []struct {
		UserID uint `gorm:"column:user_id"`
	}
	if len(updateIDsInFile) > 0 {
		// Lỗi ở đây mà bỏ qua thì profileExists rỗng ⇒ nhánh Create chạy trên hồ sơ đã tồn tại.
		tx := h.DB.Table("user_profile").Select("user_id").Where("user_id IN ?", updateIDsInFile).Find(&pidPreRows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
	}
	profileExists := make(map[uint]bool, len(pidPreRows))
	for _, p := range pidPreRows {
		profileExists[p.UserID] = true
	}

	// Giá trị hồ sơ đang lưu — để bỏ qua UPDATE khi file không đổi gì (xem sameAsCurrent).
	currentProfileRow := make(map[uint]map[string]string, len(updateIDsInFile))
	if len(updateIDsInFile) > 0 {
		var curProf []struct {
			UserID         uint   `gorm:"column:user_id"`
			Code           string `gorm:"column:code"`
			Nickname       string `gorm:"column:nickname"`
			Gender         string `gorm:"column:gender"`
			Email          string `gorm:"column:email"`
			Phone          string `gorm:"column:phone"`
			Fullname       string `gorm:"column:fullname"`
			DepartmentList string `gorm:"column:departmentList"`
		}
		tx := h.DB.Table("user_profile").
			Select(`user_id, COALESCE(code,'') code, COALESCE(nickname,'') nickname, COALESCE(gender,'') gender,
				COALESCE(email,'') email, COALESCE(phone,'') phone, COALESCE(fullname,'') fullname,
				COALESCE(departmentList,'') departmentList`).
			Where("user_id IN ?", updateIDsInFile).Find(&curProf)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		for _, r := range curProf {
			currentProfileRow[r.UserID] = map[string]string{
				"code": r.Code, "nickname": r.Nickname, "gender": r.Gender, "email": r.Email,
				"phone": r.Phone, "fullname": r.Fullname, "departmentList": r.DepartmentList,
			}
		}
	}

	// Quyền theo thao tác: file có dòng cập nhật cần updateUser, có dòng tạo mới cần createUser.
	if len(updateIDsInFile) > 0 && !h.hasPermission(userID, define.PermUpdateUser) {
		response.Forbidden(c)
		return
	}
	if len(newPasswords) > 0 && !h.hasPermission(userID, define.PermCreateUser) {
		response.Forbidden(c)
		return
	}
	// Batch pre-load branchId ĐANG LƯU của các dòng cập nhật (1 query, không N+1).
	// Dùng để chỉ validate scope khi file THẬT SỰ đổi giá trị — xem chỗ gọi jsonIDsChanged.
	currentBranchJSON := make(map[uint]string, len(updateIDsInFile))
	currentUserRow := make(map[uint]map[string]string, len(updateIDsInFile))
	if len(updateIDsInFile) > 0 {
		var curRows []struct {
			ID       uint   `gorm:"column:id"`
			BranchId string `gorm:"column:branchId"`
			Username string `gorm:"column:username"`
			Status   string `gorm:"column:status"`
			IdOld    string `gorm:"column:id_old"`
		}
		tx := h.DB.Table("user").Select("id, branchId, username, status, COALESCE(id_old,'') as id_old").
			Where("id IN ?", updateIDsInFile).Find(&curRows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		for _, r := range curRows {
			currentBranchJSON[r.ID] = r.BranchId
			currentUserRow[r.ID] = map[string]string{
				"username": r.Username, "status": r.Status, "id_old": r.IdOld,
				"branchId": r.BranchId,
			}
		}
	}

	// Batch pre-load các user ĐANG có nhiều hơn 1 vai trò. Nhánh UPDATE vai trò là DELETE-all rồi
	// INSERT đúng 1 dòng, nên với những user này việc import sẽ XOÁ các vai trò còn lại — mà file
	// export chỉ chở được 1 vai trò (MIN) nên user không hề thấy mình đang xoá cái gì.
	// Chặn thành lỗi dòng tường minh thay vì để mất im lặng; sửa vai trò cho họ phải làm trên form.
	// MỘT query lấy cả 2 thứ: user đang nhiều vai trò, và vai trò hiện tại của từng user.
	// MIN(item_name) khớp đúng với MIN(i.name) mà export ghi ra, nên so sánh "có đổi không" là so
	// đúng thứ file đang chở.
	multiRoleIDs := make(map[uint]bool)
	currentRole := make(map[uint]string, len(updateIDsInFile))
	if len(updateIDsInFile) > 0 {
		var roleRows []struct {
			UserID uint   `gorm:"column:user_id"`
			Role   string `gorm:"column:role"`
			N      int    `gorm:"column:n"`
		}
		tx := h.DB.Table("rbac_auth_assignment").
			Select("user_id, MIN(item_name) as role, COUNT(*) as n").
			Where("user_id IN ?", updateIDsInFile).Group("user_id").Find(&roleRows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		for _, r := range roleRows {
			currentRole[r.UserID] = r.Role
			if r.N > 1 {
				multiRoleIDs[r.UserID] = true
			}
		}
	}

	// [C1] Batch pre-load tập id ∈ scope caller (chống N+1) — nhánh UPDATE-by-id chỉ được
	// chạm user mà caller thấy qua BuildUserQuery (đúng IDOR guard C2/ResetPassword). Không
	// scope-check target-id → file Excel đổi username/role của user ngoài phạm vi.
	inScopeUpdateIDs := make(map[uint]bool, len(updateIDsInFile))
	if len(updateIDsInFile) > 0 {
		var scopedIDs []uint
		// `.Select("u.id")` BẮT BUỘC: GORM `Pluck` chỉ tự đặt SELECT khi `Statement.Selects` KHÁC 1
		// phần tử (finisher_api.go: `if len(tx.Statement.Selects) != 1`). BuildUserQuery gọi
		// `.Select("<chuỗi 36 cột>")` → Selects = []string{1 phần tử} → Pluck giữ nguyên 36 cột rồi
		// scan vào []uint: "Scan error on column index 0: destination not a pointer", trả về toàn 0.
		// Không có dòng này thì guard rỗng và MỌI dòng cập nhật báo "Không tìm thấy nhân viên".
		tx := h.BuildUserQuery(c).Where("u.id IN ?", updateIDsInFile).Select("u.id").Pluck("u.id", &scopedIDs)
		// Nuốt lỗi ở đây chính là thứ đã che bug trên suốt thời gian qua: query hỏng → tập scope rỗng
		// → guard IDOR từ chối tất cả, mà thông báo lại là "không tìm thấy nhân viên".
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		for _, id := range scopedIDs {
			inScopeUpdateIDs[id] = true
		}
	}

	var updatedCount, skippedCount int

	tPreload = time.Since(tStart) - tRead - tPrescan
	tLoopStart := time.Now()

	tx := h.DB.Begin()
	for i, row := range rows[dataStart:] {
		if allEmptyRow(row) {
			skippedCount++
			continue
		}
		rowIdx := i + dataStart + 1

		// Build data map từ colIndex — chỉ lấy các cột có trong file
		data := userRowData(row, colIndex)
		if len(data) == 0 {
			skippedCount++
			continue
		}

		idVal, hasID := data["id"]
		var currentUserID uint

		if hasID {
			// id có giá trị → UPDATE user đó
			var parsedID uint
			if _, err := fmt.Sscanf(fmt.Sprintf("%v", idVal), "%d", &parsedID); err != nil || parsedID == 0 {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: ID không hợp lệ '%v'", rowIdx, idVal))
				return
			}
			currentUserID = parsedID

			// [C1] Chặn cứng: KHÔNG cho import sửa tài khoản quản trị gốc (id=1), kể cả super-user.
			if currentUserID == 1 {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "user_import_root_forbidden")))
				return
			}
			// [C1] IDOR guard target-id (Y HỆT C2): target phải ∈ scope BuildUserQuery của caller.
			if !inScopeUpdateIDs[currentUserID] {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "user_not_found")))
				return
			}
			// [C1] Super bypass giống C2 (S2-3): tài khoản super chỉ super-user mới được sửa (giống
			// hệt UpdateUser — story-58 thay điều kiện "NULL-company"). Đây KHÔNG phải kiểu "giá trị
			// đổi mới kiểm" như role bên dưới: là ranh giới phân quyền trên chính bản ghi đích. Nói
			// rõ lý do thay vì "forbidden" trống trơn — họ vẫn nằm trong file export.
			if h.targetIsProtectedSuper(currentUserID) && !callerIsSuper {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "user_import_super_target")))
				return
			}

			// CHỈ đưa vào UPDATE những cột thực sự đổi giá trị.
			//
			// File export chở đủ cột, nên re-import một file chưa sửa gì sẽ ghi đè lại y nguyên từng
			// cột của TẤT CẢ nhân viên. Hai cái hại, cái sau nặng hơn nhiều:
			//   1. `updated_at`/`updated_by` bị bump cho người không hề bị sửa → sai dấu vết audit.
			//   2. Đo thật trên DB dev: mỗi `UPDATE user` một dòng theo khoá chính tốn 3–19 GIÂY
			//      (28 dòng ≈ 90s, đúng triệu chứng "import mãi không xong"). Bảng `user` có 351 FK
			//      trỏ vào; `UPDATE user_profile` cùng vòng lặp thì không hề chậm.
			// Bỏ qua UPDATE khi không có gì đổi là đúng nghiệp vụ, đồng thời né luôn chi phí đó.
			userUpdates := map[string]interface{}{}
			for _, k := range []string{"id_old", "username", "status", "branchId"} {
				if v, ok := data[k]; ok && !sameAsCurrent(k, v, currentUserRow[currentUserID]) {
					userUpdates[k] = v
				}
			}
			// Chỉ validate scope khi file THẬT SỰ đổi giá trị: file export luôn chở ĐỦ cột, validate cả
			// giá trị KHÔNG đổi ⇒ export rồi import lại y nguyên cũng bị chặn nhầm.
			if v, ok := data["branchId"].(string); ok && v != "" &&
				jsonIDsChanged(v, currentBranchJSON[currentUserID]) &&
				!jsonIDsAllowed(json.RawMessage(normalizeJSONIDs(v)), middleware.GetBranchIDs(c)) {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "branch_id_not_allowed")))
				return
			}
			if len(userUpdates) > 0 {
				userUpdates["updated_at"] = now
				userUpdates["updated_by"] = uid
				if err := tx.Table("user").Where("id = ?", currentUserID).Updates(userUpdates).Error; err != nil {
					tx.Rollback()
					response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, err.Error()))
					return
				}
			}

			profileUpdates := map[string]interface{}{}
			for _, k := range []string{"code", "nickname", "gender", "email", "departmentList"} {
				if v, ok := data[k]; ok && !sameAsCurrent(k, v, currentProfileRow[currentUserID]) {
					profileUpdates[k] = v
				}
			}
			// phone: chỉ include nếu free hoặc đang thuộc chính user này
			if phone, ok := data["phone"].(string); ok && phone != "" && !sameAsCurrent("phone", phone, currentProfileRow[currentUserID]) {
				ownerID := phoneOwner[phone]
				if ownerID == 0 || ownerID == currentUserID {
					profileUpdates["phone"] = phone
				}
			}
			if fn, ok := data["fullname"].(string); ok && fn != "" {
				if titled := helpers.ConvertToTitleCase(fn); !sameAsCurrent("fullname", titled, currentProfileRow[currentUserID]) {
					profileUpdates["fullname"] = titled
				}
			}
			if len(profileUpdates) > 0 {
				if profileExists[currentUserID] {
					if err := tx.Table("user_profile").Where("user_id = ?", currentUserID).Updates(profileUpdates).Error; err != nil {
						tx.Rollback()
						response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %v", rowIdx, err))
						return
					}
				} else {
					profileUpdates["user_id"] = currentUserID
					if err := tx.Table("user_profile").Create(profileUpdates).Error; err != nil {
						tx.Rollback()
						response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %v", rowIdx, err))
						return
					}
					profileExists[currentUserID] = true // cập nhật để tránh duplicate nếu cùng ID xuất hiện nhiều lần
				}
			}

			// Vai trò KHÔNG đổi → bỏ qua CẢ khối, không đụng gì tới rbac_auth_assignment.
			//
			// Cùng lý do với companyId/branchId ở trên: DB có sẵn nhiều nhân viên mang vai trò super
			// (admin/develop), file export chở lại đúng vai trò đó — kiểm leo thang trên cả giá trị
			// KHÔNG đổi sẽ chặn người import không-phải-super ngay dòng đầu dù họ chẳng leo thang gì.
			//
			// Điều kiện đặt ngay ở đầu khối, KHÔNG dùng `continue`: phía dưới còn
			// updatedCount++ của CHÍNH dòng này — `continue` sẽ nhảy mất cả hai.
			if role, ok := data["role"].(string); ok && role != "" && role != currentRole[currentUserID] {
				if multiRoleIDs[currentUserID] {
					tx.Rollback()
					response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "user_import_multi_role")))
					return
				}
				if roleEscalationBlocked(role, callerIsSuper) {
					tx.Rollback()
					response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "role_not_allowed")))
					return
				}
				if err := tx.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", currentUserID).Error; err != nil {
					tx.Rollback()
					response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %v", rowIdx, err))
					return
				}
				if err := tx.Exec("INSERT IGNORE INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, ?)",
					role, currentUserID, now).Error; err != nil {
					tx.Rollback()
					response.ServerError(c, err)
					return
				}
			}
			updatedCount++
		} else {
			// id trống → CREATE user mới
			username, _ := data["username"].(string)
			if username == "" {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: Tài khoản không được để trống khi tạo mới", rowIdx))
				return
			}

			// Mật khẩu đã băm ở vòng pre-scan (ngoài transaction) — tra theo đúng chỉ số dòng.
			pw, ok := newPasswords[i]
			if !ok {
				tx.Rollback()
				response.ServerError(c, fmt.Errorf("thiếu mật khẩu đã băm cho dòng %d", rowIdx))
				return
			}

			status := 10
			if s, ok := data["status"].(string); ok && s != "" {
				if sv, parseErr := strconv.Atoi(s); parseErr == nil {
					status = sv
				}
			}

			branchIdJSON := datatypes.JSON([]byte("[]"))
			if v, ok := data["branchId"].(string); ok && v != "" {
				branchIdJSON = datatypes.JSON([]byte(strings.ReplaceAll(v, " ", "")))
			}
			if !jsonIDsAllowed(json.RawMessage(branchIdJSON), middleware.GetBranchIDs(c)) {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "branch_id_not_allowed")))
				return
			}

			newUser := hrUserDao.User{
				Username:     username,
				PasswordHash: pw.hash,
				// R-7: mật khẩu do người import đặt/sinh ⇒ bắt buộc đổi ở lần đăng nhập đầu.
				MustChangePassword: define.UserMustChangePasswordYes,
				Status:             status,
				BranchId:           branchIdJSON,

				CreatedAt: now,
				UpdatedAt: now,
				CreatedBy: &uid,
				UpdatedBy: &uid,
			}
			if err := tx.Create(&newUser).Error; err != nil {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, err.Error()))
				return
			}
			currentUserID = newUser.ID
			createdUsers = append(createdUsers, createdUserInfo{Username: username, Password: pw.plain})

			profileData := map[string]interface{}{"user_id": currentUserID}
			for _, k := range []string{"code", "nickname", "phone", "gender", "email", "departmentList"} {
				if v, ok := data[k]; ok {
					profileData[k] = v
				}
			}
			if fn, ok := data["fullname"].(string); ok && fn != "" {
				profileData["fullname"] = helpers.ConvertToTitleCase(fn)
			}
			if err := tx.Table("user_profile").Create(profileData).Error; err != nil {
				tx.Rollback()
				response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %v", rowIdx, err))
				return
			}

			if role, ok := data["role"].(string); ok && role != "" {
				if roleEscalationBlocked(role, callerIsSuper) {
					tx.Rollback()
					response.Validate(c, fmt.Sprintf("Lỗi dòng %d: %s", rowIdx, i18n.T(c, "role_not_allowed")))
					return
				}
				if err := tx.Exec("INSERT IGNORE INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, ?)",
					role, currentUserID, now).Error; err != nil {
					tx.Rollback()
					response.ServerError(c, err)
					return
				}
			}
		}
	}

	tLoop = time.Since(tLoopStart)
	tCommitStart := time.Now()

	if err := tx.Commit().Error; err != nil {
		response.ServerError(c, err)
		return
	}
	h.UserSvc.InvalidateAllUserCache()
	tCommit = time.Since(tCommitStart)
	// `created` là DANH SÁCH tài khoản mới kèm mật khẩu sinh tự động (FE hiện bảng để copy), 3 con số
	// còn lại chỉ để dựng ô tổng kết — giữ nguyên `created` dạng mảng để không phá dialog đang dùng.
	response.DataWithMessage(c, gin.H{
		"created":      createdUsers,
		"createdCount": len(createdUsers),
		"updated":      updatedCount,
		"skipped":      skippedCount,
	}, i18n.T(c, "import_success"))
}

func allEmptyRow(row []string) bool {
	for _, v := range row {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

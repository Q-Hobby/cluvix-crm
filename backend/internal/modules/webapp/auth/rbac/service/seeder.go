package service

import (
	"fmt"
	"math"

	"gorm.io/gorm"
)

// pruneMaxRatio — safety-guard: nếu prune sẽ xoá > tỉ lệ này số quyền type=2 hiện có
// → nghi artifact hỏng/thiếu, ABORT prune (không xoá gì). Xem BMAD perm-codegen-seeder FR6.
const pruneMaxRatio = 0.30

// SeedResult tóm tắt kết quả 1 lần seed.
type SeedResult struct {
	Added      []string
	Updated    []string
	Pruned     []string // đã xoá thật (chỉ khi prune=true)
	WouldPrune []string // quyền thừa SẼ bị prune nếu bật SEED_PRUNE (khi prune=false, dry-run)
}

// SeedPermissions đồng bộ catalog permission (rbac_auth_item type=2) theo `items` (nguồn duy nhất
// từ artifact codegen). Thay cơ chế Sync-from-FE thủ công. Idempotent:
//  1. upsert (reuse SyncPermissions: INSERT IGNORE + UPDATE description) — LUÔN chạy.
//  2. prune=true (SEED_PRUNE): xoá quyền không còn trong `items` (rbac_auth_item + child), có
//     safety-guard tỉ lệ + invalidate cache/bump auth version cho user role bị ảnh hưởng (FR6b).
//  3. prune=false (MẶC ĐỊNH): KHÔNG xoá — chỉ trả `WouldPrune` để bootstrap log cảnh báo.
//
// Prune là destructive (xoá grant role thật) → phải bật tường minh qua SEED_PRUNE, tránh xoá nhầm
// quyền legacy đang gắn role trong giai đoạn matrix FE chưa phải superset của catalog DB.
// Guard (FR6): items rỗng → error, KHÔNG upsert/prune. Prune > pruneMaxRatio → error, KHÔNG prune.
func (s *RbacService) SeedPermissions(items []SyncPermInput, prune bool) (*SeedResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("seed permissions: artifact rỗng — ABORT (không upsert/prune)")
	}

	// 1. Upsert (tự có transaction bên trong).
	syncRes, err := s.SyncPermissions(items)
	if err != nil {
		return nil, fmt.Errorf("seed upsert: %w", err)
	}
	res := &SeedResult{Added: syncRes.Added, Updated: syncRes.Updated, Pruned: []string{}, WouldPrune: []string{}}

	// 2. Quét quyền thừa (existing type=2 \ desired).
	var existing []string
	if err := s.DB.Table("rbac_auth_item").Where("type = ?", 2).Pluck("name", &existing).Error; err != nil {
		return res, fmt.Errorf("seed prune scan: %w", err)
	}
	desiredNames := make([]string, len(items))
	for i, p := range items {
		desiredNames[i] = p.Name
	}

	// 3. MẶC ĐỊNH (SEED_PRUNE=false): dry-run — chỉ report quyền thừa, KHÔNG xoá.
	if !prune {
		res.WouldPrune = diffPrune(existing, desiredNames)
		return res, nil
	}

	// 4. SEED_PRUNE=true: prune thật (planPrune kèm safety-guard tỉ lệ + guard rỗng).
	toPrune, err := planPrune(existing, desiredNames)
	if err != nil {
		return res, err
	}
	if len(toPrune) == 0 {
		return res, nil // idempotent: không có gì để prune
	}

	// 5. Gom role bị ảnh hưởng TRƯỚC khi xoá (sau xoá, mapping child→role mất).
	var affectedRoles []string
	if err := s.DB.Table("rbac_auth_item_child").Where("child IN ?", toPrune).
		Distinct().Pluck("parent", &affectedRoles).Error; err != nil {
		return res, fmt.Errorf("seed prune affected-roles: %w", err)
	}

	// 6. Xoá tường minh child trước, rồi item (KHÔNG phụ thuộc FK cascade — robust mọi cấu hình).
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM rbac_auth_item_child WHERE child IN ?", toPrune).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM rbac_auth_item WHERE type = 2 AND name IN ?", toPrune).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return res, fmt.Errorf("seed prune delete: %w", err)
	}
	res.Pruned = toPrune

	// 7. FR6b: invalidate cache + bump auth version cho user thường của role bị ảnh hưởng
	// (reuse notifyRoleUsers — cũng revoke session để quyền đã xoá không còn hiệu lực tới hết TTL 1h).
	for _, role := range affectedRoles {
		s.notifyRoleUsers(role)
	}
	return res, nil
}

// diffPrune trả quyền thừa: existing (type=2) \ desiredNames. Thuần (không DB, không guard).
func diffPrune(existing, desiredNames []string) []string {
	desired := make(map[string]bool, len(desiredNames))
	for _, n := range desiredNames {
		desired[n] = true
	}
	var toPrune []string
	for _, n := range existing {
		if !desired[n] {
			toPrune = append(toPrune, n)
		}
	}
	return toPrune
}

// planPrune = diffPrune + safety-guard (FR6). Chỉ dùng cho path prune THẬT. Trả:
//   - err nếu desired rỗng (chống artifact rỗng) hoặc prune > pruneMaxRatio×existing (chống artifact hỏng).
//   - (nil, nil) nếu không có gì để prune.
func planPrune(existing, desiredNames []string) ([]string, error) {
	if len(desiredNames) == 0 {
		return nil, fmt.Errorf("planPrune: desired rỗng — ABORT (không xoá)")
	}
	toPrune := diffPrune(existing, desiredNames)
	if len(toPrune) == 0 {
		return nil, nil
	}
	maxPrune := int(math.Ceil(float64(len(existing)) * pruneMaxRatio))
	if len(toPrune) > maxPrune {
		return nil, fmt.Errorf("prune ABORT: sẽ xoá %d/%d quyền (> ngưỡng %d = %.0f%%) — nghi artifact hỏng",
			len(toPrune), len(existing), maxPrune, pruneMaxRatio*100)
	}
	return toPrune, nil
}

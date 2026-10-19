package tenant

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Bounds — phạm vi tenant TƯỜNG MINH, dùng ở nơi KHÔNG có *gin.Context (cron, worker,
// background job). Đây là bản sao dữ liệu của đúng 2 khoá mà session đặt vào context:
// branchActiveId / branchIds. (Story-41 E6/D15: 2 khoá company đã bỏ — 1 thương hiệu/DB.)
//
// Vì sao cần: `ScopedDBNullBranch` đọc phạm vi từ SESSION qua *gin.Context. Cron không có
// session — nếu để cron tự chế điều kiện WHERE thì sớm muộn sẽ có một chỗ quên lọc và query
// chạy xuyên phạm vi. Scope buộc caller nói rõ mình đang chạy trong phạm vi nào, và dùng
// CHUNG một thân hàm với đường có context (xem scopedDBNullBranch) nên hai đường không lệch.
//
// Giữ nguyên 2 tầng active/list y như context: ActiveID > 0 thắng, không thì dùng IDs.
// KHÔNG gộp ActiveID vào IDs — gộp làm đổi SQL sinh ra (`= ?` thành `IN ?`) trên 848 call site
// đang chạy, đổi thứ không cần đổi.
type Bounds struct {
	BranchActiveID uint
	BranchIDs      []uint
}

// BoundsFromContext trích phạm vi từ session. Đây là cầu nối DUY NHẤT giữa 2 đường —
// mọi chỗ khác chỉ làm việc với Bounds.
func BoundsFromContext(c *gin.Context) Bounds {
	return Bounds{
		BranchActiveID: getUintFromCtx(c, "branchActiveId"),
		BranchIDs:      getUintSliceFromCtx(c, "branchIds"),
	}
}

// Story-41 (E6/D15): `Empty()` (= "không có company") ĐÃ XOÁ cùng 2 field company — 0 caller
// non-test. Bounds không có chi nhánh = mọi chi nhánh, hợp lệ với 1 thương hiệu/DB (cron chạy toàn
// DB); không còn trạng thái "phạm vi rỗng đọc xuyên thương hiệu" để chặn.

// ScopedDBNullBranchBounds — bản không-context của ScopedDBNullBranch.
// Cùng thân hàm, cùng luật NULL-branch (record branch_id IS NULL luôn được lấy).
func ScopedDBNullBranchBounds(db *gorm.DB, s Bounds, table string, alias ...string) *gorm.DB {
	return applyNullBranchScope(db, s, table, false, alias...)
}

// ScopedDBNullBranchBoundsSkipBranch — như trên, bỏ qua tầng lọc chi nhánh.
func ScopedDBNullBranchBoundsSkipBranch(db *gorm.DB, s Bounds, table string, skipBranch bool, alias ...string) *gorm.DB {
	return applyNullBranchScope(db, s, table, skipBranch, alias...)
}

// applyNullBranchScope — THÂN HÀM DUY NHẤT của scope null-branch. Cả đường có context
// (scopedDBNullBranch) lẫn đường tường minh đều đi qua đây; sửa luật scope chỉ sửa ở một chỗ.
func applyNullBranchScope(db *gorm.DB, s Bounds, table string, skipBranch bool, alias ...string) *gorm.DB {
	q := db

	colPrefix := table
	if len(alias) > 0 && alias[0] != "" {
		colPrefix = alias[0]
	}

	// Story-35 (D15 — 1 thương hiệu, DB-per-customer): KHÔNG còn vế company. Mỗi DB chỉ phục vụ
	// đúng 1 thương hiệu nên lọc company_id không phân tách được gì; bỏ nó không mở rò dữ liệu.
	// Story-41: field company của Bounds + registry bảng company đã xoá hẳn.

	// Branch scope: include unassigned (NULL) rows.
	// user_option id=18 (cross-branch) không override được scope này —
	// phân quyền chi nhánh per-user phải được tôn trọng ở mọi list/get/report/EMR endpoint.
	// Chỉ SearchGlobal (customer search) tự gọi IsCrossBranchEnabled để bypass.
	if !skipBranch && branchTables[table] {
		col := colPrefix + ".branch_id"
		if s.BranchActiveID > 0 {
			q = q.Where(col+" = ? OR "+col+" IS NULL", s.BranchActiveID)
		} else if len(s.BranchIDs) > 0 {
			q = q.Where(col+" IN ? OR "+col+" IS NULL", s.BranchIDs)
		}
	}

	return q
}

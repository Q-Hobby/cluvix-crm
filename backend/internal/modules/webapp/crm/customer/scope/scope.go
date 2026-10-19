// Package scope áp giới hạn "khách mình phụ trách" cho nhân viên phòng Telesale trên
// danh sách/tìm kiếm/chi tiết khách hàng CRM (quyết định D8 — crm-standalone).
//
// Logic:
//   - user_option OPTION_VIEW_LEAD_OF_TELE (define.UserOptionViewLeadOfTele) = 1 (ON):
//     không lọc — ai cũng thấy hết.
//   - OFF: user có phòng ban Telesale trong user_profile.departmentList chỉ thấy khách
//     có customer.payment_user_id = mình — KỂ CẢ khi user thuộc thêm phòng ban khác.
//   - Phòng ban khác: không lọc.
//
// Dùng phòng ban (hr_department.id) thay vì role vì role có thể bị xoá/đổi tên.
// Nhánh phòng Convert của bản cũ (lọc theo người convert) đã bỏ cùng module sale/omnichat.
package scope

import (
	"strconv"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Restriction trả userID mà phạm vi telesale giới hạn theo (customer.payment_user_id = userID).
// restricted=false ⇒ user không bị giới hạn (option ON hoặc không thuộc phòng Telesale).
//
// FAIL-CLOSED (retro lần 2, D2): đọc user_option/user_profile lỗi ⇒ trả error, caller PHẢI
// dừng bằng response lỗi — KHÔNG được coi như "không phải Telesale" rồi bỏ lọc (trước đây
// telesale thấy toàn bộ khách khi DB lỗi). Chỉ trường hợp không có row user_profile mới
// được coi là "không thuộc Telesale".
func Restriction(c *gin.Context, db *gorm.DB) (userID uint, restricted bool, err error) {
	// Cache theo request: một màn (câu tổng + câu trang + export) gọi nhiều lần trong cùng request.
	// Chỉ cache kết quả THÀNH CÔNG — lỗi không cache để không biến lỗi tạm thành quyết định.
	if c != nil {
		if v, ok := c.Get(restrictionCtxKey); ok {
			if r, ok := v.(cachedRestriction); ok {
				return r.userID, r.restricted, nil
			}
		}
	}
	userID, restricted, err = resolveRestriction(c, db)
	if err == nil && c != nil {
		c.Set(restrictionCtxKey, cachedRestriction{userID: userID, restricted: restricted})
	}
	return userID, restricted, err
}

const restrictionCtxKey = "crm_customer_scope_restriction"

type cachedRestriction struct {
	userID     uint
	restricted bool
}

func resolveRestriction(c *gin.Context, db *gorm.DB) (userID uint, restricted bool, err error) {
	var optionVal int
	if err := db.Table("user_option").Where("id = ?", define.UserOptionViewLeadOfTele).Limit(1).
		Pluck("options", &optionVal).Error; err != nil {
		return 0, false, err
	}
	if optionVal == 1 {
		return 0, false, nil // option ON → không giới hạn
	}

	userID = middleware.GetUserID(c)

	var row struct {
		IsTelesale int `gorm:"column:is_telesale"`
	}
	if err := db.Raw(`SELECT JSON_CONTAINS(departmentList, ?) AS is_telesale
		FROM user_profile WHERE user_id = ? LIMIT 1`,
		strconv.Itoa(define.HrDepartmentTelesale),
		userID,
	).Scan(&row).Error; err != nil {
		return 0, false, err
	}
	if row.IsTelesale > 0 {
		return userID, true, nil
	}
	return 0, false, nil // phòng ban khác → thấy hết
}

// Apply scope q (FROM/JOIN bảng `customer` không alias) theo phòng ban của user hiện tại.
// db là raw DB (không scoped) — chỉ dùng để đọc user_option + user_profile.
// Lỗi ⇒ trả nguyên q + error (fail-closed, xem Restriction).
func Apply(c *gin.Context, db *gorm.DB, q *gorm.DB) (*gorm.DB, error) {
	uid, restricted, err := Restriction(c, db)
	if err != nil {
		return q, err
	}
	if restricted {
		return q.Where("customer.payment_user_id = ?", uid), nil
	}
	return q, nil
}

// CustomerIDCond — mảnh WHERE (đã bọc ngoặc) giới hạn cột customer_id `col` vào tập khách trong
// phạm vi telesale: `(col IN (SELECT id FROM customer WHERE payment_user_id = ?))`. Dùng cho MỌI
// bề mặt khác màn khách (đơn hàng, phiếu thu, lịch hẹn, báo cáo, lịch sử...) — cả GORM lẫn raw SQL.
// restricted=false ⇒ sql rỗng, caller không thêm gì (hành vi cũ). Dòng có col NULL bị loại khi
// restricted (không xác định được khách ⇒ không cho telesale xem — fail-closed).
// `col` là hằng trong code, KHÔNG nhận input người dùng.
func CustomerIDCond(c *gin.Context, db *gorm.DB, col string) (sql string, args []any, err error) {
	uid, restricted, err := Restriction(c, db)
	if err != nil || !restricted {
		return "", nil, err
	}
	return "(" + col + " IN (SELECT id FROM customer WHERE payment_user_id = ?))", []any{uid}, nil
}

// ApplyCustomerID áp CustomerIDCond lên q. Lỗi ⇒ trả nguyên q + error (fail-closed).
func ApplyCustomerID(c *gin.Context, db *gorm.DB, q *gorm.DB, col string) (*gorm.DB, error) {
	cond, args, err := CustomerIDCond(c, db, col)
	if err != nil {
		return q, err
	}
	if cond != "" {
		return q.Where(cond, args...), nil
	}
	return q, nil
}

// WhereCustomerID — như ApplyCustomerID nhưng cho các hàm dựng query KHÔNG trả error (builder dùng
// chung cho câu tổng/câu trang/export): đọc phạm vi lỗi ⇒ gắn lỗi vào q (q.AddError) để MỌI lệnh
// Find/Scan/Count sau đó KHÔNG chạy và trả lỗi ⇒ không bao giờ trả dữ liệu ngoài phạm vi (fail-closed).
func WhereCustomerID(c *gin.Context, db *gorm.DB, q *gorm.DB, col string) *gorm.DB {
	cond, args, err := CustomerIDCond(c, db, col)
	if err != nil {
		_ = q.AddError(err)
		return q
	}
	if cond != "" {
		return q.Where(cond, args...)
	}
	return q
}

package phoneutil

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/define"
)

// CanSeePhone trả true nếu user được xem số điện thoại đầy đủ.
// options=0 (OFF) = hiện toàn bộ, options=1 (ON) = ẩn (chỉ 4 số cuối).
//
// Query order tối ưu: option check trước — nếu options=0 (common case, không ẩn)
// → return true ngay, tiết kiệm dept query. BOD check chỉ chạy khi option ON.
func CanSeePhone(c *gin.Context, db *gorm.DB) bool {
	var optionVal int
	db.Table("user_option").Where("id = 14").Limit(1).Pluck("options", &optionVal)
	if optionVal == 0 {
		return true // option OFF → mọi người thấy số thật
	}

	// option ON (ẩn số): BOD luôn được xem số thật dù option bật
	userID := middleware.GetUserID(c)
	var deptCount int64
	db.Table("user_profile").
		Where("user_id = ? AND JSON_CONTAINS(departmentList, ?)", userID, fmt.Sprintf("%d", define.HrDepartmentBoardOfDirectors)).
		Count(&deptCount)
	return deptCount > 0
}

// CanSeePhoneExport trả true nếu user được xem số điện thoại đầy đủ trong file export.
// Độc lập với CanSeePhone (option 14 — list API).
// option 20 ON → tất cả user thấy; OFF (default) → chỉ BGĐ (dept=9) thấy.
func CanSeePhoneExport(c *gin.Context, db *gorm.DB) bool {
	var exportOpt int
	db.Table("user_option").Where("id = ?", 20).Limit(1).Pluck("options", &exportOpt)
	if exportOpt == 1 {
		return true
	}
	userID := middleware.GetUserID(c)
	var count int64
	db.Table("user_profile").
		Where("user_id = ? AND JSON_CONTAINS(departmentList, ?)", userID, fmt.Sprintf("%d", define.HrDepartmentBoardOfDirectors)).
		Count(&count)
	return count > 0
}

// MaskPhone ẩn tất cả ngoại trừ 4 số cuối (dùng 'x' để che, khớp format Yii2).
func MaskPhone(phone string) string {
	if len(phone) <= 4 {
		return phone
	}
	return strings.Repeat("x", len(phone)-4) + phone[len(phone)-4:]
}

// MaskNestedCustomerPhone masks phone/other_phone trong map con nested tại item[key]
// (dùng khi customer được attach qua AttachRelation, nằm
// lồng trong item[key] thay vì top-level của item — MaskPhoneInItems không xử lý được
// trường hợp này vì nó chỉ mask field top-level của mỗi map trong slice).
func MaskNestedCustomerPhone(c *gin.Context, db *gorm.DB, items []map[string]interface{}, key string) {
	if CanSeePhone(c, db) {
		return
	}
	for _, item := range items {
		nested, ok := item[key].(map[string]interface{})
		if !ok {
			continue
		}
		if phone, ok := nested["phone"].(string); ok {
			nested["phone"] = MaskPhone(phone)
		}
		if op, ok := nested["other_phone"].(string); ok {
			nested["other_phone"] = MaskPhone(op)
		}
	}
}

// MaskPhoneInItems masks phone và other_phone trong danh sách items nếu không có quyền xem.
func MaskPhoneInItems(show bool, items []map[string]interface{}) {
	if show {
		return
	}
	for _, item := range items {
		if v, ok := item["phone"]; ok {
			if s, ok2 := v.(string); ok2 {
				item["phone"] = MaskPhone(s)
			}
		}
		if v, ok := item["other_phone"]; ok {
			if s, ok2 := v.(string); ok2 {
				item["other_phone"] = MaskPhone(s)
			}
		}
	}
}

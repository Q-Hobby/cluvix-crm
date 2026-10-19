package helpers

import (
	"strconv"

	"gorm.io/gorm"
)

// InDepartment kiểm tra userID có thuộc phòng ban deptID không.
// Nguồn sự thật: cột JSON `user_profile.departmentList` (mảng id phòng ban) —
// cùng dữ liệu mà /auth/profile trả về cho FE/mobile dưới key `departmentList`.
//
// Dùng chung cho các gate nghiệp vụ theo phòng ban (khác permission RBAC).
// Pattern JSON_CONTAINS giống crm/customer/service.inDepartment và
// internal/phoneutil/phone.go — gom về đây để không nhân bản query.
func InDepartment(db *gorm.DB, userID uint, deptID int) bool {
	if userID == 0 {
		return false
	}
	var n int64
	db.Table("user_profile").
		Where("user_id = ? AND JSON_CONTAINS(departmentList, ?)", userID, strconv.Itoa(deptID)).
		Count(&n)
	return n > 0
}

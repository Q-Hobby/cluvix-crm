package helpers

import (
	"erpCluvix/pkg/define"

	"gorm.io/gorm"
)

// IsSuperUser kiểm tra user có role thuộc define.SuperRoles không, qua raw query
// (KHÔNG cần inject RbacService — dùng ở handler chưa có RbacSvc trong Deps).
// Pattern giống internal/middleware/response_headers.go (Super-user bypass).
func IsSuperUser(db *gorm.DB, userID uint) bool {
	var isSuper bool
	db.Raw("SELECT EXISTS(SELECT 1 FROM rbac_auth_assignment a "+
		"JOIN rbac_auth_item i ON a.item_name = i.name "+
		"WHERE a.user_id = ? AND i.type = 1 AND i.name IN ?)", userID, define.SuperRoles).
		Scan(&isSuper)
	return isSuper
}

// IsSuperUserE — cùng truy vấn với IsSuperUser nhưng TRẢ lỗi SQL thay vì che thành `false`.
// Dùng ở cửa quyền mà lỗi DB phải lộ ra 500 (vd `config/onboarding` bootstrap), không phải 403 sai
// ngữ nghĩa. IsSuperUser giữ nguyên để không đổi hành vi 20 call site cũ — sửa SQL thì sửa CẢ HAI.
func IsSuperUserE(db *gorm.DB, userID uint) (bool, error) {
	var isSuper bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM rbac_auth_assignment a "+
		"JOIN rbac_auth_item i ON a.item_name = i.name "+
		"WHERE a.user_id = ? AND i.type = 1 AND i.name IN ?)", userID, define.SuperRoles).
		Scan(&isSuper).Error; err != nil {
		return false, err
	}
	return isSuper, nil
}

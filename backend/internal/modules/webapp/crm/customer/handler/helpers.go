package handler

import (
	"errors"
	"fmt"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// validatePaymentUserIDChange kiểm tra telesale có được phép đổi payment_user_id không.
//
// Logic (mirror Yii2 Customer::validateTelesalePermission, dùng phòng ban thay role):
//  1. payment_user_id nil/không có trong body → skip
//  2. Đổi thành chính mình → OK
//  3. option id=12 = 1 (ON) → cho phép cập nhật chéo toàn bộ → OK (early exit)
//  4. Không thuộc phòng Telesale (id=3) → OK (admin/manager không bị chặn)
//  5. Thuộc phòng Telesale + option OFF:
//     a. Target user đang inactive/deleted → OK (claim lead từ nhân sự nghỉ việc)
//     b. Còn lại → BLOCK
//
// Query order tối ưu: option check trước (early exit nếu ON, bỏ qua dept+target query).
func validatePaymentUserIDChange(c *gin.Context, db *gorm.DB, body map[string]interface{}) error {
	raw, exists := body["payment_user_id"]
	if !exists || raw == nil {
		return nil
	}

	newUID := int(helpers.ToInt64(raw))
	if newUID <= 0 {
		return nil
	}

	currentUID := int(middleware.GetUserID(c))
	if newUID == currentUID {
		return nil // gán cho chính mình → luôn OK
	}

	// Option check trước: nếu ON → bỏ qua dept + target queries
	var optionVal int
	db.Table("user_option").Where("id = 12").Limit(1).Pluck("options", &optionVal)
	if optionVal == 1 {
		return nil
	}

	// Chỉ chặn phòng Telesale
	var telesaleCount int64
	db.Table("user_profile").
		Where("user_id = ? AND JSON_CONTAINS(departmentList, ?)", currentUID,
			fmt.Sprintf("%d", define.HrDepartmentTelesale)).
		Count(&telesaleCount)
	if telesaleCount == 0 {
		return nil // không phải telesale → OK
	}

	// Telesale + option OFF: cho phép claim lead từ nhân sự nghỉ việc
	var targetStatus int
	db.Table("user").Where("id = ?", newUID).Limit(1).Pluck("status", &targetStatus)
	if targetStatus == define.UserStatusInactive || targetStatus == define.UserStatusDeleted {
		return nil
	}

	return errors.New("customer_cross_update_not_allowed")
}

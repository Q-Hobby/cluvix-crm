package handler

import (
	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// appointmentBranchAllowed — branch_id client gửi phải thuộc danh sách branch user được gán.
func appointmentBranchAllowed(c *gin.Context, branchID *int) bool {
	if branchID == nil || *branchID <= 0 {
		return false
	}
	for _, bid := range middleware.GetBranchIDs(c) {
		if int(bid) == *branchID {
			return true
		}
	}
	return false
}

// appointmentCustomerInTenant — customer_id client gửi trong body Create/Update phải là khách CÓ
// THẬT. CỐ Ý không lọc chi nhánh (khách đăng ký chi nhánh này vẫn đặt hẹn ở chi nhánh khác).
// Story-36 (D15 — 1 thương hiệu/DB): bỏ vế company ⇒ tenant.CrossBranchLookup.
func (h *Handler) appointmentCustomerInTenant(customerID int) bool {
	var count int64
	tenant.CrossBranchLookup(h.DB, "customer").
		Table("customer").
		Where("id = ?", customerID).
		Count(&count)
	return count > 0
}

// Helpers — copy từ handler dịch vụ điều trị cũ (duplication intentional cho boundary thật). Long-term có thể extract sang pkg/relation
// nếu nhiều module cần.
package handler

import (
	"time"

	"erpCluvix/internal/middleware"
	otpDao "erpCluvix/internal/modules/webapp/crm/order_plan/dao"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"

	"github.com/gin-gonic/gin"
)

// verifyCanEditOrderPlan kiểm tra user có quyền sửa đơn hàng điều trị.
//
// Logic (mirror FE checkOrderPlanUpdate):
//   - Có quyền updatePlanOrder → luôn được phép.
//   - Không có quyền updatePlanOrder → phải đủ CẢ 4 điều kiện:
//     1. Có quyền createPlanOrder (prerequisite).
//     2. user_option id=3 (OPTION_UPDATE_ORDER_PLAN) = 1 (bật).
//     3. Là người tạo ra đơn hàng đó (created_by = userID).
//     4. Vẫn trong cùng ngày tạo (theo timezone của thương hiệu).
func (h *Handler) verifyCanEditOrderPlan(c *gin.Context, id uint) bool {
	var o otpDao.OrderPlan
	if err := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "order_plan", tenant.IsCrossBranchServiceEnabled(c, h.DB)).First(&o, id).Error; err != nil {
		response.NotFound(c)
		return false
	}

	userID := middleware.GetUserID(c)

	if h.hasPermission(userID, "updatePlanOrder") {
		return true
	}

	// Creator+same-day path: branch ownership must hold even when cross-branch read (option 18) là general
	// viewing toggle. Option 23 (UserOptionCrossBranchService) bypass được nhánh này — cùng lý do với
	// payment (xem verifyCanEditPaymentCustomer).
	branchID := 0
	if o.BranchId != nil {
		branchID = *o.BranchId
	}
	if !branchAllowed(c, branchID) && !tenant.IsCrossBranchServiceEnabled(c, h.DB) {
		response.Forbidden(c)
		return false
	}

	if !h.hasPermission(userID, "createPlanOrder") {
		response.Forbidden(c)
		return false
	}

	var optionVal int
	h.DB.Table("user_option").Where("id = ?", define.UserOptionUpdateOrderPlan).Limit(1).Pluck("options", &optionVal)
	if optionVal != 1 {
		response.Forbidden(c)
		return false
	}

	if o.CreatedBy == nil || *o.CreatedBy != userID {
		response.Forbidden(c)
		return false
	}

	now := time.Now().In(tz.LocFromContext(c, h.DB))
	createdAt := time.Unix(o.CreatedAt, 0).In(now.Location())
	if now.Year() != createdAt.Year() || now.Month() != createdAt.Month() || now.Day() != createdAt.Day() {
		response.Forbidden(c)
		return false
	}

	return true
}

// verifyCanDeleteOrderPlan kiểm tra order tồn tại trong tenant + user có quyền
// deletePlanOrder (mirror FE deleteRecordTreatmentPlan — flat permission check,
// không có nhánh creator/same-day như update).
func (h *Handler) verifyCanDeleteOrderPlan(c *gin.Context, id uint) bool {
	var o otpDao.OrderPlan
	if err := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "order_plan", tenant.IsCrossBranchServiceEnabled(c, h.DB)).First(&o, id).Error; err != nil {
		response.NotFound(c)
		return false
	}

	userID := middleware.GetUserID(c)
	if !h.hasPermission(userID, "deletePlanOrder") {
		response.Forbidden(c)
		return false
	}
	return true
}

func (h *Handler) hasPermission(userID uint, permCode string) bool {
	if h.RbacSvc == nil {
		return false
	}
	perms, _ := h.RbacSvc.GetUserPermissions(userID)
	for _, p := range perms {
		if p == permCode {
			return true
		}
	}
	return false
}

// branchAllowed: true khi branchID = 0 (unassigned) hoặc nằm trong branch list của user.
func branchAllowed(c *gin.Context, branchID int) bool {
	if branchID == 0 {
		return true
	}
	if active := middleware.GetBranchActiveID(c); active > 0 {
		return int(active) == branchID
	}
	for _, bid := range middleware.GetBranchIDs(c) {
		if int(bid) == branchID {
			return true
		}
	}
	return false
}

// attachRelationReverse — story-33: gộp về handlerutil.AttachRelationReverse.

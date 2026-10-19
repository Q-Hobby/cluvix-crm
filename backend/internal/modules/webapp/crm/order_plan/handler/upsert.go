package handler

import (
	"strconv"

	"erpCluvix/internal/middleware"
	crmServices "erpCluvix/internal/modules/webapp/crm/service"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// CreateOrderPlan POST /crm/order-plan
func (h *Handler) CreateOrderPlan(c *gin.Context) {
	var req crmServices.OrderPlanRequest
	if err := helpers.BindJSON(c, &req); err != nil {
		response.ValidateErr(c, err)
		return
	}
	// Chặn forge — status/created_at/created_by chỉ được set bởi BeforeCreate hook,
	// KHÔNG nhận từ client (mass assignment qua embed BaseModelWithCode).
	req.ID = 0
	req.Code = ""
	req.Status = nil
	req.CreatedAt = 0
	req.CreatedBy = nil
	req.UpdatedBy = nil

	userID := int(middleware.GetUserID(c))

	if err := h.svc.Create(&req, userID); err != nil {
		response.HandleError(c, err)
		return
	}
	response.Created(c, req.OrderPlan)
}

// UpdateOrderPlan PUT /crm/order-plan/:id
func (h *Handler) UpdateOrderPlan(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if !h.verifyCanEditOrderPlan(c, uint(id)) {
		return
	}
	var req crmServices.OrderPlanRequest
	if err := helpers.BindJSON(c, &req); err != nil {
		response.ValidateErr(c, err)
		return
	}
	req.ID = uint(id)

	userID := int(middleware.GetUserID(c))
	scopedDB := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "order_plan", tenant.IsCrossBranchServiceEnabled(c, h.DB))
	if err := h.svc.Update(&req, userID, scopedDB); err != nil {
		response.HandleError(c, err)
		return
	}
	response.Updated(c, req.OrderPlan)
}

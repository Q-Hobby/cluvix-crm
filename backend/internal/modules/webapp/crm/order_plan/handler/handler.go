// Package handler — HTTP layer cho OrderPlan (đơn hàng điều trị).
//
// Plan B.1 (2026-05-16): Migrate handler ra layered module. Service KHÔNG move —
// `internal/services/crm/order_plan.go` có cross-file dependency với
// `customer_clv_trigger.go`, `order_service_used_validate.go`
// trong cùng package `crm`. Move service đơn lẻ sẽ break compilation; move
// kèm các file kia lại sai phân chia (clv_trigger là shared utility
// cho multiple services). Surgical compromise: handler + repo layered, service
// giữ ở package crm.
package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	crmServices "erpCluvix/internal/modules/webapp/crm/service"

	"gorm.io/gorm"
)

// Deps gom dependency cần inject từ router.
type Deps struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
}

type Handler struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
	svc     *crmServices.OrderPlanService
}

func New(d Deps) *Handler {
	return &Handler{
		DB:      d.DB,
		RbacSvc: d.RbacSvc,
		svc:     crmServices.NewOrderPlanService(d.DB),
	}
}

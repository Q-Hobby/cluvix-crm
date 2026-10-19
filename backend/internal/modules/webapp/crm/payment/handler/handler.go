// Package handler — Payment HTTP layer.
//
// Plan B.2 (2026-05-16): Handler + DTO + http test migrate ra layered module.
// Service KHÔNG move — `internal/services/crm/payment*.go` (5 file, ~2500 LOC
// gồm payment.go, payment_crud.go, payment_commission.go, payment_validate.go +
// tests) có cross-file dependency với customer_clv_trigger,
// order_service_used_validate trong cùng package crm. Pattern giống
// order_plan (cùng compromise).
package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	crmServices "erpCluvix/internal/modules/webapp/crm/service"

	"gorm.io/gorm"
)

type Deps struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
}

type Handler struct {
	DB         *gorm.DB
	RbacSvc    *authSvc.RbacService
	PaymentSvc *crmServices.PaymentService
}

func New(d Deps) *Handler {
	return &Handler{
		DB:         d.DB,
		RbacSvc:    d.RbacSvc,
		PaymentSvc: crmServices.NewPaymentService(d.DB),
	}
}

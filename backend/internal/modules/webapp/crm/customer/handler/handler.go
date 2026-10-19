// Package handler — Customer HTTP layer.
//
// Plan B.2 (2026-05-16) — last hot-spot module: FULL layered migrate (handler +
// service). No repo Reader exists for customer. CustomerService self-contained
// (no cross-file deps trong crm package), nên move sạch giống appointment.
//
// Note: customer_clv_trigger.go (từng ở crm/service) đã xoá cùng tính năng CLV (2026-10-17).
package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	customerService "erpCluvix/internal/modules/webapp/crm/customer/service"
	"erpCluvix/pkg/storage"

	"gorm.io/gorm"
)

type Deps struct {
	DB    *gorm.DB
	Store storage.Store
	// RbacSvc — chỉ route.go dùng (guard export `?_export=1` cần extCustomer, S-7).
	RbacSvc *authSvc.RbacService
}

type Handler struct {
	DB          *gorm.DB
	Store       storage.Store
	CustomerSvc *customerService.CustomerService
}

func New(d Deps) *Handler {
	return &Handler{
		DB:          d.DB,
		Store:       d.Store,
		CustomerSvc: customerService.NewCustomerService(d.DB),
	}
}

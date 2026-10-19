// Package handler — Note CRUD + pin.
//
// Plan B.2 (2026-05-16): Migrate ra layered module. Handler-only — note table
// có company_id (tenant-scoped), không có service file riêng. Logic gồm CRUD
// + pin/unpin + business hook (update customer.last_contact_date khi tạo note).
package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"

	"gorm.io/gorm"
)

type Deps struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
}

type Handler struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
}

func New(d Deps) *Handler { return &Handler{DB: d.DB, RbacSvc: d.RbacSvc} }

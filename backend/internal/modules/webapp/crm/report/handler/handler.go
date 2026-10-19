package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
	// RbacSvc — chỉ route.go dùng (guard export báo cáo cần manageCrmReport, S-7).
	RbacSvc *authSvc.RbacService
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

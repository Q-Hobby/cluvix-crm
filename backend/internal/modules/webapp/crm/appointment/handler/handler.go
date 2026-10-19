// Package handler — Appointment HTTP layer.
//
// Plan B.2 (2026-05-16): FULL layered migrate — handler + service + repo all
// move. Unlike payment / order_plan, the appointment service is
// self-contained (no cross-file dependencies on loyalty/CLV/validate helpers
// in the crm package), so the service moves cleanly into the module.
//
// Pattern: handler.go defines Deps + Handler struct + New; appointment.go
// and schedule.go (moved from internal/handlers/crm/) provide endpoint methods.
package handler

import (
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	apptService "erpCluvix/internal/modules/webapp/crm/appointment/service"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
	// RbacSvc — chỉ route.go dùng (guard export `?_export=1` cần extAppointment, S-7).
	RbacSvc *authSvc.RbacService
}

type Handler struct {
	DB             *gorm.DB
	AppointmentSvc *apptService.AppointmentService
}

func New(d Deps) *Handler {
	return &Handler{
		DB:             d.DB,
		AppointmentSvc: apptService.NewAppointmentService(d.DB),
	}
}

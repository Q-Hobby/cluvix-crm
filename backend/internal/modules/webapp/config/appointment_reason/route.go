// Package appointment_reason — Plan 06 B1.
package appointment_reason

import (
	"erpCluvix/internal/modules/webapp/config/appointment_reason/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/appointment-reason endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/appointment-reason", h.ListAppointmentReason)
	cfg.GET("/appointment-reason/:id", h.GetAppointmentReason)
	cfg.POST("/appointment-reason", h.CreateAppointmentReason)
	cfg.PUT("/appointment-reason/:id", h.UpdateAppointmentReason)
	cfg.DELETE("/appointment-reason/:id", h.DeleteAppointmentReason)
}

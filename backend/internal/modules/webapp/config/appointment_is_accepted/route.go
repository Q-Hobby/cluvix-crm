// Package appointment_is_accepted — Plan 06 B1.
package appointment_is_accepted

import (
	"erpCluvix/internal/modules/webapp/config/appointment_is_accepted/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/appointment-is-accepted endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/appointment-is-accepted", h.ListAppointmentIsAccepted)
	cfg.GET("/appointment-is-accepted/:id", h.GetAppointmentIsAccepted)
	cfg.POST("/appointment-is-accepted", h.CreateAppointmentIsAccepted)
	cfg.PUT("/appointment-is-accepted/:id", h.UpdateAppointmentIsAccepted)
	cfg.DELETE("/appointment-is-accepted/:id", h.DeleteAppointmentIsAccepted)
}

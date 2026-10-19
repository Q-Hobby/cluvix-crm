// Package appointment_not_booked_reasons — Plan 06 B1.
package appointment_not_booked_reasons

import (
	"erpCluvix/internal/modules/webapp/config/appointment_not_booked_reasons/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/appointment-not-booked-reasons endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/appointment-not-booked-reasons", h.ListAppointmentNotBookedReasons)
	cfg.GET("/appointment-not-booked-reasons/:id", h.GetAppointmentNotBookedReasons)
	cfg.POST("/appointment-not-booked-reasons", h.CreateAppointmentNotBookedReasons)
	cfg.PUT("/appointment-not-booked-reasons/:id", h.UpdateAppointmentNotBookedReasons)
	cfg.DELETE("/appointment-not-booked-reasons/:id", h.DeleteAppointmentNotBookedReasons)
}

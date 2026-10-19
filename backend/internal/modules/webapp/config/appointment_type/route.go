package appointment_type

import (
	"erpCluvix/internal/modules/webapp/config/appointment_type/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/appointment-type", h.ListAppointmentType)
	cfg.GET("/appointment-type/:id", h.GetAppointmentType)
	cfg.POST("/appointment-type", h.CreateAppointmentType)
	cfg.PUT("/appointment-type/:id", h.UpdateAppointmentType)
	cfg.POST("/appointment-type/type-revenue-change", h.ToggleAppointmentTypeRevenue)
	cfg.DELETE("/appointment-type/:id", h.DeleteAppointmentType)
}

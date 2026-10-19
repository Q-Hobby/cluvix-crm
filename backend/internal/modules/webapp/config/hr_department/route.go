// Package hr_department — Plan 06 B1.
package hr_department

import (
	"erpCluvix/internal/modules/webapp/config/hr_department/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/hr-department endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/hr-department", h.ListHrDepartment)
	cfg.GET("/hr-department/:id", h.GetHrDepartment)
	cfg.POST("/hr-department", h.CreateHrDepartment)
	cfg.PUT("/hr-department/:id", h.UpdateHrDepartment)
	cfg.DELETE("/hr-department/:id", h.DeleteHrDepartment)
}

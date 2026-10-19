// Package ethnicity — Plan 06 B1.
package ethnicity

import (
	"erpCluvix/internal/modules/webapp/config/ethnicity/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/ethnicity endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/ethnicity", h.ListEthnicity)
	cfg.GET("/ethnicity/:id", h.GetEthnicity)
	cfg.POST("/ethnicity", h.CreateEthnicity)
	cfg.PUT("/ethnicity/:id", h.UpdateEthnicity)
	cfg.DELETE("/ethnicity/:id", h.DeleteEthnicity)
}

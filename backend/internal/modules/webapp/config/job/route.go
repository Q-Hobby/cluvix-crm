// Package job — Plan 06 B1.
package job

import (
	"erpCluvix/internal/modules/webapp/config/job/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/job endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/job", h.ListJob)
	cfg.GET("/job/:id", h.GetJob)
	cfg.POST("/job", h.CreateJob)
	cfg.PUT("/job/:id", h.UpdateJob)
	cfg.DELETE("/job/:id", h.DeleteJob)
}

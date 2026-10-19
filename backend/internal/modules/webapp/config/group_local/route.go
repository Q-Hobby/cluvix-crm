// Package group_local — Plan 06 B1.
package group_local

import (
	"erpCluvix/internal/modules/webapp/config/group_local/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/group-local endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/group-local", h.ListGroupLocal)
	cfg.GET("/group-local/:id", h.GetGroupLocal)
	cfg.POST("/group-local", h.CreateGroupLocal)
	cfg.PUT("/group-local/:id", h.UpdateGroupLocal)
	cfg.DELETE("/group-local/:id", h.DeleteGroupLocal)
}

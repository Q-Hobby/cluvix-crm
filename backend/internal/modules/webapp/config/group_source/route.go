// Package group_source — Plan 06 B1.
package group_source

import (
	"erpCluvix/internal/modules/webapp/config/group_source/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/group-source endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/group-source", h.ListGroupSource)
	cfg.GET("/group-source/:id", h.GetGroupSource)
	cfg.POST("/group-source", h.CreateGroupSource)
	cfg.PUT("/group-source/:id", h.UpdateGroupSource)
	cfg.POST("/group-source/check-online", h.CheckOnlineGroupSource)
	cfg.DELETE("/group-source/:id", h.DeleteGroupSource)
}

// Package group_country — Plan 06 B1.
package group_country

import (
	"erpCluvix/internal/modules/webapp/config/group_country/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/group-country endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/group-country", h.ListGroupCountry)
	cfg.GET("/group-country/:id", h.GetGroupCountry)
	cfg.POST("/group-country", h.CreateGroupCountry)
	cfg.PUT("/group-country/:id", h.UpdateGroupCountry)
	cfg.DELETE("/group-country/:id", h.DeleteGroupCountry)
}

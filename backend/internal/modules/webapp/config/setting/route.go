// Package setting — Plan 06 B1.
package setting

import (
	"erpCluvix/internal/modules/webapp/config/setting/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/setting endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/setting", h.ListSetting)
	cfg.GET("/setting/:id", h.GetSetting)
	cfg.POST("/setting", h.CreateSetting)
	cfg.PUT("/setting/:id", h.UpdateSetting)
	cfg.DELETE("/setting/:id", h.DeleteSetting)
}

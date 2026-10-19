// Package location_province — Plan 06 B1.
package location_province

import (
	"erpCluvix/internal/modules/webapp/config/location_province/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/location-province endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/location-province", h.ListLocationProvince)
	cfg.GET("/location-province/:id", h.GetLocationProvince)
	cfg.POST("/location-province", h.CreateLocationProvince)
	cfg.PUT("/location-province/:id", h.UpdateLocationProvince)
	cfg.DELETE("/location-province/:id", h.DeleteLocationProvince)
}

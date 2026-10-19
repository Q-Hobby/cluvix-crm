// Package location_ward — Plan 06 B1.
package location_ward

import (
	"erpCluvix/internal/modules/webapp/config/location_ward/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/location-ward endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/location-ward", h.ListLocationWard)
	cfg.GET("/location-ward/get-by-province", h.GetLocationWardByProvince)
	cfg.GET("/location-ward/:id", h.GetLocationWard)
	cfg.POST("/location-ward", h.CreateLocationWard)
	cfg.PUT("/location-ward/:id", h.UpdateLocationWard)
	cfg.DELETE("/location-ward/:id", h.DeleteLocationWard)
}

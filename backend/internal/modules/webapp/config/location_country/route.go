// Package location_country — Plan 06 B1.
package location_country

import (
	"erpCluvix/internal/modules/webapp/config/location_country/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/location-country endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/location-country", h.ListLocationCountry)
	cfg.GET("/location-country/:id", h.GetLocationCountry)
	cfg.POST("/location-country", h.CreateLocationCountry)
	cfg.PUT("/location-country/:id", h.UpdateLocationCountry)
	cfg.DELETE("/location-country/:id", h.DeleteLocationCountry)
}

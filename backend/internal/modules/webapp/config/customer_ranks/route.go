// Package customer_ranks — Plan 06 B1.
package customer_ranks

import (
	"erpCluvix/internal/modules/webapp/config/customer_ranks/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/customer-ranks endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/customer-ranks", h.ListCustomerRanks)
	cfg.GET("/customer-ranks/:id", h.GetCustomerRanks)
	cfg.POST("/customer-ranks", h.CreateCustomerRanks)
	cfg.PUT("/customer-ranks/:id", h.UpdateCustomerRanks)
	cfg.DELETE("/customer-ranks/:id", h.DeleteCustomerRanks)
}

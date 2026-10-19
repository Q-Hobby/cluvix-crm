// Package customer_category — Plan 06 B1.
package customer_category

import (
	"erpCluvix/internal/modules/webapp/config/customer_category/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/customer-category endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/customer-category", h.ListCustomerCategory)
	cfg.GET("/customer-category/:id", h.GetCustomerCategory)
	cfg.POST("/customer-category", h.CreateCustomerCategory)
	cfg.PUT("/customer-category/:id", h.UpdateCustomerCategory)
	cfg.DELETE("/customer-category/:id", h.DeleteCustomerCategory)
}

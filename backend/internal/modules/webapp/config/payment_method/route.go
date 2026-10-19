// Package payment_method — Plan 06 B1.
package payment_method

import (
	"erpCluvix/internal/modules/webapp/config/payment_method/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/payment-method endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/payment-method", h.ListPaymentMethod)
	cfg.GET("/payment-method/:id", h.GetPaymentMethod)
	cfg.POST("/payment-method", h.CreatePaymentMethod)
	cfg.PUT("/payment-method/:id", h.UpdatePaymentMethod)
	cfg.DELETE("/payment-method/:id", h.DeletePaymentMethod)
}

// Package customer_rank_services — Plan 06 B1.
package customer_rank_services

import (
	"erpCluvix/internal/modules/webapp/config/customer_rank_services/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/customer-rank-services endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/customer-rank-services", h.ListCustomerRankServices)
	cfg.GET("/customer-rank-services/:id", h.GetCustomerRankServices)
	cfg.POST("/customer-rank-services", h.CreateCustomerRankServices)
	cfg.PUT("/customer-rank-services/:id", h.UpdateCustomerRankServices)
	cfg.DELETE("/customer-rank-services/:id", h.DeleteCustomerRankServices)
}

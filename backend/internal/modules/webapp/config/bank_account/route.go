// Package bank_account — Plan 06 B1.
package bank_account

import (
	"erpCluvix/internal/modules/webapp/config/bank_account/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/bank-account endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/bank-account", h.ListBankAccount)
	cfg.GET("/bank-account/random", h.GetRandomBankAccount)
	cfg.GET("/bank-account/:id", h.GetBankAccount)
	cfg.POST("/bank-account", h.CreateBankAccount)
	cfg.PUT("/bank-account/:id", h.UpdateBankAccount)
	cfg.DELETE("/bank-account/:id", h.DeleteBankAccount)
}

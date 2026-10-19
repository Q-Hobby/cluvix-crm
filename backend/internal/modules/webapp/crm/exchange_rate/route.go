// Package exchange_rate — module root.
package exchange_rate

import (
	"erpCluvix/internal/modules/webapp/crm/exchange_rate/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	crm.GET("/exchange-rate/currency", h.ExchangeRateCurrency)
}

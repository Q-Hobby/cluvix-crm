package payment_in_day

import (
	"erpCluvix/internal/modules/webapp/print/payment_in_day/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(p gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	p.GET("/payment-in-day", h.PaymentInDay)
}

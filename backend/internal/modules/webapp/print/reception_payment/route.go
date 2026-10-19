package reception_payment

import (
	"erpCluvix/internal/modules/webapp/print/reception_payment/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(p gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	p.GET("/reception-payment", h.PrintReceptionPayment)
}

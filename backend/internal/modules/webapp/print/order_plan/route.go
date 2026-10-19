package order_plan

import (
	"erpCluvix/internal/modules/webapp/print/order_plan/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(p gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	p.GET("/order-plan/:id", h.PrintOrderPlan)
}

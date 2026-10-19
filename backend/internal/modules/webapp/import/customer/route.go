package customer

import (
	"erpCluvix/internal/modules/webapp/import/customer/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(importGroup gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	importGroup.POST("/customer/from-excel", h.ImportCustomerFromExcel)
}

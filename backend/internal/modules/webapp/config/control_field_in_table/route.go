package control_field_in_table

import (
	"erpCluvix/internal/modules/webapp/config/control_field_in_table/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/control-field-in-table/:id/get-field-customer", h.GetFieldCustomer)
	cfg.POST("/control-field-in-table/:id/update-field-customer", h.UpdateFieldCustomer)
	cfg.POST("/control-field-in-table/:id/re-build", h.ReBuildFieldCustomer)
}

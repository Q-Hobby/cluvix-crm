package config_code

import (
	"erpCluvix/internal/modules/webapp/config/config_code/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/config-code", h.ListConfigCode)
	cfg.GET("/config-code/customer-code-option", h.GetCustomerCodeOption)
	cfg.GET("/config-code/:id", h.GetConfigCode)
	cfg.POST("/config-code", h.CreateConfigCode)
	cfg.PUT("/config-code/:id", h.UpdateConfigCode)
	cfg.DELETE("/config-code/:id", h.DeleteConfigCode)
}

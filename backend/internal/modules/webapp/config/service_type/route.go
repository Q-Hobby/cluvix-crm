package service_type

import (
	"erpCluvix/internal/modules/webapp/config/service_type/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/service-type", h.ListServiceType)
	cfg.GET("/service-type/:id", h.GetServiceType)
	cfg.POST("/service-type", h.CreateServiceType)
	cfg.POST("/service-type/import-from-excel", h.ImportServiceTypeFromExcel)
	cfg.PUT("/service-type/:id", h.UpdateServiceType)
	cfg.POST("/service-type/:id", h.UpdateServiceType)
	cfg.DELETE("/service-type/:id", h.DeleteServiceType)
}

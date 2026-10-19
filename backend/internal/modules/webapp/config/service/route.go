package service

import (
	"erpCluvix/internal/modules/webapp/config/service/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/service", h.ListService)
	cfg.GET("/service/load-service-by-type", h.LoadServiceByType)
	cfg.GET("/service/load-by-keyword", h.LoadServiceByKeyword)
	cfg.GET("/service/load-list-service-by-keyword", h.LoadListServiceByKeyword)
	cfg.GET("/service/load-service", h.LoadService)
	cfg.GET("/service/:id", h.GetService)
	cfg.POST("/service", h.CreateService)
	cfg.POST("/service/:id", h.UpdateService)
	cfg.PUT("/service/:id", h.UpdateService)
	cfg.POST("/service/import-service-from-excel", h.ImportServiceFromExcel)
	cfg.DELETE("/service/:id", h.DeleteService)
}

package unit

import (
	"erpCluvix/internal/modules/webapp/config/unit/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/unit", h.ListUnit)
	cfg.GET("/unit/:id", h.GetUnit)
	cfg.POST("/unit", h.CreateUnit)
	cfg.PUT("/unit/:id", h.UpdateUnit)
	cfg.POST("/unit/import-from-excel", h.ImportUnitFromExcel)
	cfg.DELETE("/unit/:id", h.DeleteUnit)
}

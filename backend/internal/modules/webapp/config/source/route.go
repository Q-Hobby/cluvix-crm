package source

import (
	"erpCluvix/internal/modules/webapp/config/source/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/source", h.ListSource)
	cfg.GET("/source/overview-chart-source", h.OverviewChartSource)
	cfg.GET("/source/:id", h.GetSource)
	cfg.POST("/source", h.CreateSource)
	cfg.PUT("/source/:id", h.UpdateSource)
	cfg.DELETE("/source/:id", h.DeleteSource)
}

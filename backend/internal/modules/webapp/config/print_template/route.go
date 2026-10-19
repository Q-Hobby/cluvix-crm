package print_template

import (
	"erpCluvix/internal/modules/webapp/config/print_template/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	g := cfg.Group("/print-template")
	{
		g.GET("", h.List)
		g.GET("/default", h.GetDefault) // phải trước /:id
		g.GET("/:id", h.Get)
		g.POST("", h.Create)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
	}
}

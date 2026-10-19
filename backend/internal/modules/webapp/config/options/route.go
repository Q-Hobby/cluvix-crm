package options

import (
	"erpCluvix/internal/modules/webapp/config/options/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/options/get-options", h.GetOptions)
	cfg.GET("/options/get-option/:id", h.GetOption)
	cfg.PUT("/options/update-option/:id", h.UpdateOption)
}

package log

import (
	"erpCluvix/internal/modules/webapp/history/log/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(g gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	g.GET("/log", h.ListLog)
}

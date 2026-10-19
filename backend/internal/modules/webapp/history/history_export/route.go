package history_export

import (
	"erpCluvix/internal/modules/webapp/history/history_export/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(g gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	g.GET("/history-export", h.ListHistoryExport)
}

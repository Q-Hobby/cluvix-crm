// Package history — domain entrypoint cho /history/* (Phase 4).
package history

import (
	"erpCluvix/internal/deps"
	"erpCluvix/internal/modules/webapp/history/history_export"
	historyExportHandler "erpCluvix/internal/modules/webapp/history/history_export/handler"
	"erpCluvix/internal/modules/webapp/history/log"
	logHandler "erpCluvix/internal/modules/webapp/history/log/handler"

	"github.com/gin-gonic/gin"
)

func Register(authed *gin.RouterGroup, d deps.Deps) {
	g := authed.Group("/history")
	history_export.RegisterRoutes(g, historyExportHandler.Deps{DB: d.DB})
	log.RegisterRoutes(g, logHandler.Deps{DB: d.DB})
}

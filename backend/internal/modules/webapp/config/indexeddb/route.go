package indexeddb

import (
	"erpCluvix/internal/modules/webapp/config/indexeddb/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(authed gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	// URL giữ nguyên /crm/indexeddb/sync (backward compat) — data thì là config
	// tables nên module location ở config/indexeddb/.
	authed.GET("/crm/indexeddb/sync", h.Sync)
}

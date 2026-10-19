// Package card_type — Plan 06 B1.
package card_type

import (
	"erpCluvix/internal/modules/webapp/config/card_type/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/card-type endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/card-type", h.ListCardType)
	cfg.GET("/card-type/:id", h.GetCardType)
	cfg.POST("/card-type", h.CreateCardType)
	cfg.PUT("/card-type/:id", h.UpdateCardType)
	cfg.DELETE("/card-type/:id", h.DeleteCardType)
}

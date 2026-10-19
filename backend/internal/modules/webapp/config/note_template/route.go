// Package note_template — Plan 06 B1.
package note_template

import (
	"erpCluvix/internal/modules/webapp/config/note_template/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /config/note-template endpoints.
func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/note-template", h.ListNoteTemplate)
	cfg.GET("/note-template/:id", h.GetNoteTemplate)
	cfg.POST("/note-template", h.CreateNoteTemplate)
	cfg.PUT("/note-template/:id", h.UpdateNoteTemplate)
	cfg.DELETE("/note-template/:id", h.DeleteNoteTemplate)
}

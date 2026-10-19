// Package note — module root.
//
// Plan B.2 (2026-05-16): Migrate note CRUD ra layered module.
package note

import (
	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/note/handler"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes gắn endpoint /crm/note/* vào parent group.
// Static paths (get-all-note, list-advise) đăng ký TRƯỚC /:id.
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	note := crm.Group("/note")
	{
		// Export ghi chú (`?_export=1`, màn báo cáo Ghi chú — FE gác manageCrmReport) chặn thật (S-7).
		note.GET("/get-all-note", middleware.RequireExportPerm(d.RbacSvc, define.PermManageCrmReport), h.GetAllNote)
		note.GET("/list-advise", h.ListAdvise)
		note.GET("/:id", h.GetNote)
		note.POST("", h.CreateNote)
		note.POST("/:id/pin-note", h.PinNote)
		note.DELETE("/:id", h.DeleteNote)
	}
}

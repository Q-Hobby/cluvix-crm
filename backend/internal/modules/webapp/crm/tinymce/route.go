// Package tinymce — module root.
package tinymce

import (
	"erpCluvix/internal/modules/webapp/crm/tinymce/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	crm.POST("/tinymce/upload-image", h.TinymceUploadImage)
}

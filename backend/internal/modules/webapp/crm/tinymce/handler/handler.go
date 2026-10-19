// Package handler — Tinymce inline image upload qua S3 + auto convert.
// Plan B.2 (2026-05-16): Layered module, handler-only.
package handler

import (
	"net/http"

	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/imaging"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
)

type Deps struct {
	Store storage.Store
}

type Handler struct {
	Store storage.Store
}

func New(d Deps) *Handler { return &Handler{Store: d.Store} }

// TinymceUploadImage POST /crm/tinymce/upload-image
// FormFile "image" → ProcessAndUpload (convert+resize+upload) → trả URL plain JSON.
func (h *Handler) TinymceUploadImage(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil || file == nil {
		response.Validate(c, i18n.T(c, "upload_image_invalid"))
		return
	}
	if h.Store == nil {
		response.Validate(c, i18n.T(c, "s3_not_configured"))
		return
	}
	key, err := imaging.ProcessAndUpload(c.Request.Context(), h.Store, file, "tiny/", nil)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	// Ảnh TinyMCE nằm NHÚNG trong HTML nội dung (không có cột riêng để ghép URL lúc đọc) ⇒ trả URL
	// công khai đầy đủ ngay lúc upload. Driver local không khai STORAGE_PUBLIC_BASE ⇒ URL tương đối.
	c.JSON(http.StatusOK, storage.PublicURL(h.Store, key))
}

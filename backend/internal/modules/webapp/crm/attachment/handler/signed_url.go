package handler

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
)

// signedURLRequest — body tuỳ chọn của POST /crm/attachment/:id/signed-url.
type signedURLRequest struct {
	// Download — true ⇒ Content-Disposition: attachment (nút "Tải xuống"); false/không gửi ⇒ inline
	// cho loại xem được trong trình duyệt (ảnh raster, PDF, audio, video), attachment cho phần còn lại.
	Download bool `json:"download"`
}

// SignedURL POST /crm/attachment/:id/signed-url — cấp URL KÝ NGẮN HẠN để trình duyệt đọc thẳng file
// (<video src>, <audio src>, mở tab xem trước, tải xuống) không cần JWT: hỗ trợ HTTP Range ⇒ tua
// video/audio, không phải tải trọn blob vào RAM.
//
// Quyền: ĐÚNG 3 lớp như Download (perm viewCustomer qua route_perm_map → scope chi nhánh của bản ghi →
// user xem được khách, gồm scope telesale) — ngoài phạm vi ⇒ 404. POST (không phải GET) để URL ký
// không bị cache/prefetch và không nằm trong log truy cập dạng link GET.
//
// Response: {url, expires_at (unix giây; 0 với legacy), legacy}. Bản ghi cũ có file_url là URL tuyệt
// đối (object public-read trước 2026-10-19) ⇒ trả nguyên URL đó + legacy=true (như Download redirect).
// Driver local ⇒ url có thể là đường dẫn tương đối gốc site (/api/files/signed/…) khi
// STORAGE_PUBLIC_BASE trống — FE ghép origin của API.
func (h *Handler) SignedURL(c *gin.Context) {
	var req signedURLRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			response.ValidateErr(c, err)
			return
		}
	}

	att, ok := h.loadVisible(c)
	if !ok {
		return
	}

	if storage.IsAbsoluteURL(att.FileUrl) {
		response.Data(c, gin.H{"url": att.FileUrl, "expires_at": 0, "legacy": true})
		return
	}
	signer, ok := h.PrivateStore.(storage.Signer)
	if h.PrivateStore == nil || !ok {
		response.ServerError(c, fmt.Errorf("storage private không hỗ trợ URL ký"))
		return
	}

	contentType := safeContentType(att.FileType)
	disposition := storage.DispositionAttachment
	if !req.Download && inlineViewable(contentType) {
		disposition = storage.DispositionInline
	}
	ttl := storage.ClampSignedURLTTL(h.SignedURLTTL)
	// Hạn tính TRƯỚC khi ký: expires_at trả FE không bao giờ muộn hơn hạn thật trong chữ ký.
	expiresAt := timeNow().Add(ttl).Unix()
	url, err := signer.SignedURL(c.Request.Context(), att.FileUrl, ttl, storage.SignOptions{
		ContentType: contentType,
		Disposition: disposition,
		FileName:    safeDownloadName(att.FileName, att.FileUrl),
	})
	if errors.Is(err, storage.ErrUnsafePath) {
		response.NotFound(c)
		return
	}
	if err != nil {
		response.ServerError(c, err)
		return
	}
	response.Data(c, gin.H{"url": url, "expires_at": expiresAt, "legacy": false})
}

// inlineViewable — loại được phép hiển thị inline khi đọc qua URL ký: ảnh raster (không SVG — không
// có trong whitelist), PDF, audio/video trong whitelist (trình duyệt chỉ phát, không thực thi).
// contentType đã qua safeContentType (ngoài whitelist ⇒ octet-stream ⇒ attachment).
func inlineViewable(contentType string) bool {
	if contentType == "application/pdf" || storage.IsAllowedMIME(contentType, storage.ImageMIMETypes) {
		return true
	}
	return storage.IsAllowedMIME(contentType, storage.AttachmentMIMETypes) &&
		(strings.HasPrefix(contentType, "audio/") || strings.HasPrefix(contentType, "video/"))
}

// timeNow — đồng hồ cho expires_at (biến để test cố định thời điểm). Chỉ lấy Unix ⇒ không phụ thuộc
// timezone.
var timeNow = time.Now

package handler

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	customerEntity "erpCluvix/internal/modules/webapp/crm/customer/dao"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// maxDownloadNameRunes — độ dài tối đa tên file trong Content-Disposition.
const maxDownloadNameRunes = 150

// Download GET /crm/attachment/:id/download — tải file đính kèm qua backend.
//
// Kiểm đủ 3 lớp trước khi đọc kho: perm viewCustomer (PermissionEnforcer, route_perm_map.json) →
// scope chi nhánh của bản ghi attachment (ScopedDBNullBranch) → user xem được khách tương ứng
// (cùng phạm vi GetCustomer, gồm scope telesale). Ngoài phạm vi ⇒ 404 (không lộ sự tồn tại).
//
// Bản ghi cũ (file_url là URL tuyệt đối, object public-read từ trước 2026-10-19) ⇒ 302 tới URL đó.
// Chọn redirect thay vì backend tự tải-rồi-stream: object cũ vốn đã công khai, proxy qua backend
// không che được gì thêm mà còn tốn băng thông + mở đường SSRF nếu cột bị ghi giá trị lạ.
func (h *Handler) Download(c *gin.Context) {
	att, ok := h.loadVisible(c)
	if !ok {
		return
	}

	if storage.IsAbsoluteURL(att.FileUrl) {
		c.Redirect(http.StatusFound, att.FileUrl)
		return
	}
	if h.PrivateStore == nil {
		response.ServerError(c, fmt.Errorf("storage private chưa được cấu hình"))
		return
	}

	rc, info, err := h.PrivateStore.Open(c.Request.Context(), att.FileUrl)
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrUnsafePath) {
		response.NotFound(c)
		return
	}
	if err != nil {
		response.ServerError(c, err)
		return
	}
	defer rc.Close()

	contentType := safeContentType(att.FileType)
	// inline chỉ cho ảnh raster (không SVG — không có trong whitelist) và PDF để FE xem trước;
	// mọi loại khác ⇒ attachment (tải xuống, không render).
	disposition := "attachment"
	if contentType == "application/pdf" || storage.IsAllowedMIME(contentType, storage.ImageMIMETypes) {
		disposition = "inline"
	}
	cd := mime.FormatMediaType(disposition, map[string]string{"filename": safeDownloadName(att.FileName, att.FileUrl)})
	if cd == "" {
		cd = disposition
	}

	size := info.Size
	if size < 0 {
		size = -1
	}
	c.DataFromReader(http.StatusOK, size, contentType, rc, map[string]string{
		"Content-Disposition":    cd,
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "private, no-store",
	})
}

// loadVisible — đọc attachment theo :id với ĐỦ lớp kiểm quyền dùng chung cho download + signed-url:
// id parse ra số → scope chi nhánh (ScopedDBNullBranch) → user xem được khách (customerVisible).
// ok=false ⇒ đã ghi response (404 ngoài phạm vi/không tồn tại — không lộ sự tồn tại; 500 lỗi đọc).
func (h *Handler) loadVisible(c *gin.Context) (customerEntity.Attachment, bool) {
	var att customerEntity.Attachment
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return att, false
	}
	tx := tenant.ScopedDBNullBranch(c, h.DB, "attachment").
		Select("id, customer_id, file_name, file_url, file_type").
		Where("id = ?", id).Limit(1).Find(&att)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return att, false
	}
	if att.ID == 0 {
		response.NotFound(c)
		return att, false
	}
	visible, err := h.customerVisible(c, att.CustomerID)
	if err != nil {
		response.ServerError(c, err)
		return att, false
	}
	if !visible {
		response.NotFound(c)
		return att, false
	}
	return att, true
}

// safeContentType — Content-Type theo MIME đã sniff lúc upload (cột file_type), chỉ khi nằm trong
// whitelist; còn lại ép octet-stream để trình duyệt không bao giờ render nội dung lạ.
func safeContentType(fileType string) string {
	ct := storage.BaseMIME(fileType)
	if !storage.IsAllowedMIME(ct, storage.AttachmentMIMETypes) {
		return "application/octet-stream"
	}
	return ct
}

// safeDownloadName — tên file gợi ý khi tải: tên gốc user upload (để họ nhận ra file), bỏ ký tự
// điều khiển/đường dẫn/ngoặc kép, giới hạn độ dài. Đuôi file lấy theo KEY (đuôi theo MIME đã sniff)
// để tên tải về khớp nội dung thật (ảnh đã convert sang .webp).
func safeDownloadName(original, key string) string {
	base := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(original, "\\", "/")), filepath.Ext(original))
	base = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\"<>:|?*`, r) {
			return '_'
		}
		return r
	}, base)
	base = strings.TrimSpace(base)
	if r := []rune(base); len(r) > maxDownloadNameRunes {
		base = string(r[:maxDownloadNameRunes])
	}
	if base == "" || base == "." || base == ".." {
		base = "attachment"
	}
	return base + strings.ToLower(filepath.Ext(key))
}

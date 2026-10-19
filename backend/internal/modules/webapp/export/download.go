package export

import (
	"os"

	"github.com/gin-gonic/gin"

	"erpCluvix/internal/middleware"
	pkgExport "erpCluvix/pkg/export"
	"erpCluvix/pkg/response"
)

// RegisterDownload đăng ký endpoint tải file export trên nhóm CÓ JWT.
// Thay `r.Static("/download/export", …)` công khai cũ (S-2/O-6, 2026-10-18).
func RegisterDownload(authed *gin.RouterGroup) {
	authed.GET("/download/export/:name", DownloadExport)
}

// DownloadExport GET /api/download/export/:name — trả file export của CHÍNH user đang đăng nhập.
//
// File nằm ở `<DefaultDir>/<userID>/<name>` (pkg/export.Config.OwnerID). Tên sai định dạng (có `/`,
// `..`, bắt đầu bằng `.`, đuôi lạ), file của user khác, file đã bị dọn ⇒ cùng một 404 — không tiết lộ
// file có tồn tại ở chỗ khác hay không.
func DownloadExport(c *gin.Context) {
	path, ok := pkgExport.OwnerFilePath(pkgExport.DefaultDir, middleware.GetUserID(c), c.Param("name"))
	if !ok {
		response.NotFound(c)
		return
	}
	// Lstat (không theo symlink) + chỉ file thường: thư mục export không bao giờ chứa symlink hợp lệ.
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		response.NotFound(c)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.FileAttachment(path, c.Param("name"))
}

package middleware

import (
	"net/http"
	"strings"

	"erpCluvix/pkg/i18n"

	"github.com/gin-gonic/gin"
)

const (
	MaxJSONBodyBytes      int64 = 5 << 20  // 5 MB
	MaxMultipartBodyBytes int64 = 32 << 20 // 32 MB — trần dung lượng 1 FILE (handler tự áp)
	// multipartOverheadBytes — phần dư cho boundary + header từng part + các field text đi kèm file,
	// để một file đúng trần 32 MB vẫn lọt qua trần của CẢ body.
	multipartOverheadBytes int64 = 1 << 20
	// MaxMultipartRequestBytes — trần của TOÀN BỘ body multipart.
	MaxMultipartRequestBytes = MaxMultipartBodyBytes + multipartOverheadBytes
)

// RequestSizeLimit enforces body size limits:
//   - multipart/form-data: Content-Length khai vượt trần ⇒ 413 ngay; LUÔN bọc MaxBytesReader
//     (MaxMultipartRequestBytes) — request chunked (Content-Length = -1) trước đây lọt hoàn toàn
//     vì chỉ so Content-Length. Vượt trần lúc đọc ⇒ *http.MaxBytesError nổi lên ở chỗ parse
//     multipart; response.HandleError/ServerError/ValidateErr nhận ra và trả 413.
//   - everything else: wrap body with MaxBytesReader (5 MB)
func RequestSizeLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.GetHeader("Content-Type")
		if strings.HasPrefix(ct, "multipart/form-data") {
			if c.Request.ContentLength > MaxMultipartRequestBytes {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge,
					gin.H{"message": i18n.T(c, "payload_too_large")})
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxMultipartRequestBytes)
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBodyBytes)
		c.Next()
	}
}

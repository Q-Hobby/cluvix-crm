package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS returns the canonical Cluvix CORS middleware. `isProd` flag gates the
// Allow-Origin whitelist — non-prod reflects any Origin (dev convenience).
// Caller PHẢI truyền `cfg.IsProduction()` — hàm này fail-closed: chỉ APP_ENV dev/develop/local tường
// minh mới là non-prod; "staging"/"production"/giá trị lạ đều dùng whitelist (S-12/O-8).
// `allowedOrigins` (CORS_ALLOWED_ORIGINS, CSV) là whitelist prod; rỗng = fail-closed
// (KHÔNG origin nào được cấp Allow-Origin).
func CORS(isProd bool, allowedOrigins []string) gin.HandlerFunc {
	corsAllowedOrigins := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o = strings.TrimSpace(o); o != "" {
			corsAllowedOrigins[o] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		// Retro F10 (2026-10-17): bỏ 2 nhánh ngoại lệ `/api/client/*` (Zalo Mini App/partner — reflect
		// mọi Origin) và `/api/client/livechat/` (Allow-Credentials: false) — module client/livechat đã
		// gỡ khỏi CRM standalone, không còn route nào dưới prefix đó. Prod: CHỈ origin trong whitelist.
		if origin != "" && (!isProd || corsAllowedOrigins[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		// PATCH BẮT BUỘC có mặt: omnichat dùng PATCH cho gắn khách/assign/status/tab... Thiếu nó thì
		// preflight vẫn trả 204 nhưng browser CHẶN request thật — không có dòng log nào ở BE, DevTools
		// cũng không hiện status code, nên rất dễ chẩn đoán nhầm thành lỗi handler.
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		// 10 phút, KHÔNG 24h: preflight sai (thiếu method/header) sẽ sống thêm đúng bằng Max-Age trên
		// máy TỪNG client kể cả sau khi deploy bản vá — 24h biến một fix 1 dòng thành sự cố cả ngày,
		// và làm chính lớp bug này gần như không chẩn đoán được. Chi phí đổi lại chỉ là 1 preflight
		// mỗi 10 phút cho mỗi endpoint.
		c.Header("Access-Control-Max-Age", "600")
		c.Header("Access-Control-Expose-Headers", "X-Pagination-Total-Count, X-Pagination-Page-Count, X-Pagination-Current-Page, X-Pagination-Per-Page, X-Dexie-Versions, X-Version-Emr, Cluvix-Customer-Lead, Cluvix-Reminder, X-Request-ID, X-User-Auth-Version")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders sets baseline X-Content-Type-Options / X-Frame-Options /
// X-XSS-Protection on every response.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}

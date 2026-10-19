package middleware

import (
	"regexp"

	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
)

// webhookSecretPathRe — mask segment :secret của route webhook public (Telegram) trước khi lưu vào
// goroutine-local store. RequestPath() đăng ký GLOBAL cho MỌI route kể cả webhook public — bất kỳ lỗi
// SQL nào (deadlock/lock-timeout...) xảy ra trong lúc xử lý webhook → gorm.Trace ghi "SQL Error @ POST
// /api/omni/webhook/telegram/<secret>: ..." bằng L.Error, mức Error kích hoạt hook gửi Telegram alert
// (pkg/logger/logger.go) nếu bật → secret có thể lộ thẳng ra group chat. Mask segment :secret để tránh.
var webhookSecretPathRe = regexp.MustCompile(`(/webhook/telegram/)[^/?]+`)

// maskWebhookSecret thay segment secret bằng "***" — chỉ áp dụng route webhook public, path khác
// giữ nguyên.
func maskWebhookSecret(path string) string {
	return webhookSecretPathRe.ReplaceAllString(path, "$1***")
}

// RequestPath set "METHOD /url" vào goroutine-local store ở đầu request
// để gorm.Trace lấy được path khi log SQL Error.
// Defer Clear tránh leak map khi goroutine kết thúc.
func RequestPath() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.SetRequestPath(c.Request.Method + " " + maskWebhookSecret(c.Request.URL.String()))
		defer logger.ClearRequestPath()
		c.Next()
	}
}

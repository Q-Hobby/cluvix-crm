package middleware

import (
	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/response"
)

// RenderErrorGuard chặn lớp lỗi "200 body rỗng câm lặng": Gin (v1.12) khi c.JSON marshal lỗi chỉ
// đẩy lỗi vào c.Errors rồi Abort — status 200 đã set nhưng CHƯA ghi header/body, không panic, không
// log. Client nhận 200 rỗng, FE vỡ ở chỗ khác (sự cố 2026-09-21: note đơn "[11, 21, , 14]" bị
// ParseJSONFields ép thành RawMessage hỏng). Header chưa ghi nên vẫn thay được bằng 500 chuẩn.
func RenderErrorGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Written() || len(c.Errors) == 0 {
			return
		}
		response.ServerErrorReadOnly(c, c.Errors.Last())
	}
}

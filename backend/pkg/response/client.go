package response

import (
	"time"

	"github.com/gin-gonic/gin"
)

// --- Client API response format ---
// Khác với CRM format. Dùng cho module client (partner API).
// Format: {success, code, message, data, timestamp}

func ClientSuccess(c *gin.Context, data interface{}, message string) {
	if message == "" {
		message = "Success"
	}
	c.JSON(200, gin.H{
		"success":   true,
		"code":      200,
		"message":   message,
		"data":      data,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	})
}

func ClientError(c *gin.Context, message string, code int) {
	c.JSON(code, gin.H{
		"success":   false,
		"code":      code,
		"message":   message,
		"data":      nil,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	})
}

func ClientValidate(c *gin.Context, message string) {
	c.JSON(422, gin.H{
		"success":   false,
		"code":      422,
		"message":   message,
		"data":      nil,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	})
}

package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const requestIDKey = "request_id"
const requestIDHeader = "X-Request-ID"

// RequestID generates or propagates X-Request-ID on every request.
// The ID is echoed back in the response header so clients can correlate logs.
// Pairs with GetLogger(c): per-request logger automatically includes request_id.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)

		// Enrich the per-request logger with request_id (composing on top of
		// whatever JWTAuth already set, or falling back to the global logger).
		enriched := GetLogger(c).With(zap.String("request_id", id))
		c.Set("logger", enriched)

		c.Next()
	}
}

// GetRequestID returns the request ID from the context, empty string if absent.
func GetRequestID(c *gin.Context) string {
	v, _ := c.Get(requestIDKey)
	if id, ok := v.(string); ok {
		return id
	}
	return ""
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

package router

import (
	"context"
	"errors"
	"net/http"
	"time"

	"erpCluvix/internal/deps"
	"erpCluvix/pkg/helpers"

	"github.com/gin-gonic/gin"
)

// healthHandler — GET /health.
//
// Phơi thêm trạng thái decimalColumnMap (story-02) + jsonColumnMap (story-26) —
// decimal-json-number. HTTP vẫn 200 kể cả khi map hỏng: KHÔNG fail-fast (đổi sang 5xx sẽ biến lỗi
// đọc INFORMATION_SCHEMA thành restart-loop). Monitor đọc field decimal_column_map.ready /
// json_column_map.ready — false nghĩa là mọi cột DECIMAL/JSON đang trả về dạng string cho tới khi
// restart.
//
// Tách khỏi closure trong Setup() để test đăng ký được ĐÚNG handler này qua route thật
// (health_test.go) — assert tự dựng payload bằng gin.H không chứng minh được wiring.
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"decimal_column_map": helpers.DecimalColumnMapStatusSnapshot(),
		"json_column_map":    helpers.JSONColumnMapStatusSnapshot(),
	})
}

// readyTimeout — trần cho MỖI lần ping phụ thuộc (DB, Redis). Ngắn để probe không treo khi
// dependency chết.
const readyTimeout = 2 * time.Second

// readyCheck — một phép kiểm phụ thuộc; trả lỗi nếu không sẵn sàng.
type readyCheck func(ctx context.Context) error

// readyHandler — GET /health/ready (readiness, O-11).
//
// /health giữ nguyên là liveness nhẹ (luôn 200, không đụng DB/Redis) — dùng cho process-alive.
// /health/ready ping DB + Redis với timeout ngắn: 200 khi cả hai OK, 503 khi bất kỳ phần nào hỏng.
// Body chỉ có "ok"/"fail" từng phần — KHÔNG trả err.Error() (có thể chứa host/DSN).
// Route nằm NGOÀI /api ⇒ không cần route_perm_map.
func readyHandler(checks map[string]readyCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := make(map[string]string, len(checks))
		allOK := true
		for name, chk := range checks {
			ctx, cancel := context.WithTimeout(c.Request.Context(), readyTimeout)
			done := make(chan error, 1)
			go func() { done <- chk(ctx) }()
			var err error
			select {
			case err = <-done:
			case <-ctx.Done():
				err = ctx.Err()
			}
			cancel()
			if err != nil {
				parts[name] = "fail"
				allOK = false
			} else {
				parts[name] = "ok"
			}
		}
		code, status := http.StatusOK, "ok"
		if !allOK {
			code, status = http.StatusServiceUnavailable, "unavailable"
		}
		c.JSON(code, gin.H{"status": status, "checks": parts})
	}
}

// newReadyChecks dựng phép kiểm DB + Redis từ deps (nil-safe: dependency thiếu ⇒ fail).
func newReadyChecks(d deps.Deps) map[string]readyCheck {
	return map[string]readyCheck{
		"database": func(ctx context.Context) error {
			if d.DB == nil {
				return errors.New("db nil")
			}
			sqlDB, err := d.DB.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		},
		"redis": func(ctx context.Context) error {
			if d.Cache == nil {
				return errors.New("redis nil")
			}
			return d.Cache.Ping()
		},
	}
}

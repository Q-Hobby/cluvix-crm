package router

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"erpCluvix/pkg/helpers"

	"github.com/gin-gonic/gin"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// story-02 (decimal-json-number) AC2 — trạng thái decimalColumnMap phải đọc được TỪ NGOÀI
// qua /health.
//
// Vì sao test nằm ở đây chứ không ở pkg/helpers: test cũ tự `json.Marshal(gin.H{...})` rồi
// assert chuỗi ⇒ payload do CHÍNH TEST dựng, xoá field trong router vẫn xanh (verification debt
// QA pass 1 [P2]). pkg/helpers KHÔNG import ngược được internal/router (cycle), nên lưới phải
// đặt ở package router. Không bind port: gin.New() + httptest.NewRecorder.
func serveHealth(t *testing.T) map[string]any {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", healthHandler) // ĐÚNG handler mà Setup() đăng ký

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /health: HTTP %d, muốn 200 (AC4: không fail-fast kể cả khi map hỏng)\nbody: %s",
			w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /health body: %v (%s)", err, w.Body.String())
	}
	return body
}

// Route thật phải phơi field decimal_column_map với trạng thái hỏng — nếu ai xoá/đổi tên field
// trong healthHandler thì test này ĐỎ (đó là điều test cũ không làm được).
func TestHealthRoute_ExposesDecimalColumnMapFailure(t *testing.T) {
	helpers.InvalidateSchemaCache() // ⇒ status = not initialized
	t.Cleanup(helpers.InvalidateSchemaCache)

	body := serveHealth(t)

	if body["status"] != "ok" {
		t.Fatalf(`/health phải giữ "status":"ok" (AC4), got %v`, body["status"])
	}
	raw, ok := body["decimal_column_map"]
	if !ok {
		t.Fatalf("/health THIẾU field decimal_column_map — monitor mất khả năng phát hiện suy biến im lặng; body: %v", body)
	}
	st, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("decimal_column_map phải là object, got %T (%v)", raw, raw)
	}
	if ready, _ := st["ready"].(bool); ready {
		t.Fatalf(`decimal_column_map.ready phải là false khi map chưa init, got %v`, st)
	}
	if errMsg, _ := st["reason"].(string); errMsg == "" {
		t.Fatalf("decimal_column_map.reason phải nêu mã lý do khi ready=false, got %v", st)
	}
	t.Logf("observed /health body: %s", mustJSON(t, body))
}

// story-26 AC2/AC4 — jsonColumnMap phải phơi trạng thái giống decimalColumnMap (trước đây
// InitJSONColumnMap nuốt lỗi INFORMATION_SCHEMA im lặng, không ai biết cột JSON đang trả string thô).
func TestHealthRoute_ExposesJSONColumnMapFailure(t *testing.T) {
	helpers.InvalidateSchemaCache()
	t.Cleanup(helpers.InvalidateSchemaCache)

	body := serveHealth(t)

	raw, ok := body["json_column_map"]
	if !ok {
		t.Fatalf("/health THIẾU field json_column_map — monitor mất khả năng phát hiện suy biến im lặng; body: %v", body)
	}
	st, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("json_column_map phải là object, got %T (%v)", raw, raw)
	}
	if ready, _ := st["ready"].(bool); ready {
		t.Fatalf(`json_column_map.ready phải là false khi map chưa init, got %v`, st)
	}
	if reason, _ := st["reason"].(string); reason == "" {
		t.Fatalf("json_column_map.reason phải nêu mã lý do khi ready=false, got %v", st)
	}
	if _, hasError := st["error"]; hasError {
		t.Fatalf(`json_column_map KHÔNG được có field "error" (rò err.Error() của driver ra route không auth) — got %v`, st)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// ── QA pass 3 finding [low] — verification-gap: gỡ `json:"-"` khỏi DecimalColumnMapStatus.Error
// thì TOÀN BỘ lưới cũ vẫn xanh (không case nào chạy qua nhánh lỗi có chuỗi driver thật + đọc
// route thật cùng lúc). Test dưới đây bám ĐÚNG route thật (`healthHandler` qua gin), tự tạo
// trạng thái lỗi có chuỗi nhạy cảm y hệt driver MySQL thật (không chạm MySQL thật — driver giả),
// rồi khẳng định body /health KHÔNG CHỨA field "error" lẫn nguyên văn chuỗi đó.

var errSimulatedDriver = errors.New("Access denied for user 'root'@'localhost' (using password: YES)")

type failingConnector struct{}

func (failingConnector) Connect(context.Context) (driver.Conn, error) { return nil, errSimulatedDriver }
func (failingConnector) Driver() driver.Driver                        { return failingDriver{} }

type failingDriver struct{}

func (failingDriver) Open(string) (driver.Conn, error) { return nil, errSimulatedDriver }

// failingGorm trả về *gorm.DB mà mọi query đều lỗi với chuỗi driver "nhạy cảm" giả lập —
// không mở kết nối MySQL thật, không DSN thật.
func failingGorm(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB := sql.OpenDB(failingConnector{})
	g, err := gorm.Open(
		mysqldriver.New(mysqldriver.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true,
		}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("open fake gorm: %v", err)
	}
	return g
}

// TestHealthRoute_DoesNotLeakDriverError — assert PHỦ ĐỊNH: body /health không được chứa field
// "error" lẫn nguyên văn chuỗi lỗi driver, kể cả khi InitDecimalColumnMap thất bại với lỗi
// thật-như-thật (user/host lộ ra). Đây là lưới CHẶN TÁI PHÁT: nếu ai gỡ `json:"-"` khỏi
// `DecimalColumnMapStatus.Error` (hoặc thêm field mới mang err.Error() ra JSON), test này phải ĐỎ.
func TestHealthRoute_DoesNotLeakDriverError(t *testing.T) {
	helpers.InvalidateSchemaCache()
	t.Cleanup(helpers.InvalidateSchemaCache)

	helpers.InitDecimalColumnMap(failingGorm(t)) // set status với Error = errSimulatedDriver.Error()

	body := serveHealth(t)

	raw, ok := body["decimal_column_map"]
	if !ok {
		t.Fatalf("/health THIẾU field decimal_column_map; body: %v", body)
	}
	st, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("decimal_column_map phải là object, got %T (%v)", raw, raw)
	}
	if _, hasError := st["error"]; hasError {
		t.Fatalf(`decimal_column_map KHÔNG được có field "error" (rò err.Error() của driver ra route`+
			` không auth) — got %v`, st)
	}

	full := mustJSON(t, body)
	if strings.Contains(full, "\"error\"") {
		t.Fatalf(`/health body chứa field "error" — rò thông tin hạ tầng: %s`, full)
	}
	if strings.Contains(full, errSimulatedDriver.Error()) {
		t.Fatalf("/health body chứa NGUYÊN VĂN chuỗi lỗi driver — rò user/host: %s", full)
	}
	t.Logf("observed /health body (must be clean): %s", full)
}

func serveReady(checks map[string]readyCheck) (int, map[string]any) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health/ready", readyHandler(checks))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestReady_AllOK(t *testing.T) {
	ok := func(context.Context) error { return nil }
	code, body := serveReady(map[string]readyCheck{"database": ok, "redis": ok})
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("muốn 200/ok, got %d %v", code, body)
	}
}

func TestReady_FailureIs503_NoErrorLeak(t *testing.T) {
	ok := func(context.Context) error { return nil }
	bad := func(context.Context) error { return errors.New("dial tcp 10.1.2.3:3306 secret-host") }
	code, body := serveReady(map[string]readyCheck{"database": bad, "redis": ok})
	if code != http.StatusServiceUnavailable {
		t.Fatalf("muốn 503, got %d", code)
	}
	checks := body["checks"].(map[string]any)
	if checks["database"] != "fail" || checks["redis"] != "ok" {
		t.Fatalf("trạng thái từng phần sai: %v", checks)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "secret-host") || strings.Contains(string(raw), "10.1.2.3") {
		t.Fatalf("body lộ thông tin nội bộ: %s", raw)
	}
}

func TestReady_HungDependencyTimesOut(t *testing.T) {
	hang := func(ctx context.Context) error { select {} }
	code, body := serveReady(map[string]readyCheck{"redis": hang})
	if code != http.StatusServiceUnavailable || body["checks"].(map[string]any)["redis"] != "fail" {
		t.Fatalf("dependency treo phải ra 503/fail, got %d %v", code, body)
	}
}

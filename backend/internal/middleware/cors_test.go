package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// preflight chạy 1 request OPTIONS qua CORS(isProd, allowedOrigins) và trả response đã ghi header.
func preflight(origin string, isProd bool) *httptest.ResponseRecorder {
	return preflightWithAllowlist(origin, isProd, []string{"https://app.example.com"})
}

func preflightWithAllowlist(origin string, isProd bool, allowedOrigins []string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(isProd, allowedOrigins))
	r.PATCH("/api/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	req.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	r.ServeHTTP(w, req)
	return w
}

func allowedMethods(w *httptest.ResponseRecorder) map[string]bool {
	set := map[string]bool{}
	for _, m := range strings.Split(w.Header().Get("Access-Control-Allow-Methods"), ",") {
		if m = strings.TrimSpace(m); m != "" {
			set[strings.ToUpper(m)] = true
		}
	}
	return set
}

// TestCORS_PreflightAdvertisesEveryMethodRoutersUse — lưới chặn cho lớp bug đã xảy ra 2026-08-17:
// PATCH bị thiếu trong Access-Control-Allow-Methods, preflight vẫn trả 204 nên KHÔNG có dòng log nào
// ở BE và DevTools cũng không hiện status code; browser lặng lẽ chặn request thật. Hậu quả: cả 8 route
// PATCH của omnichat (mark-read/resolve/status/priority/tab/assign/customer/availability) chết trên
// mọi deployment khác origin — kể cả production, vì FE prod chạy ở origin trong CORS_ALLOWED_ORIGINS.
//
// Build xanh + test xanh vẫn để lọt, vì không có gì buộc header phải khớp với method mà router đăng ký.
// Thêm method mới vào bất kỳ router nào → thêm vào đây, và phải thêm cả trong cors.go.
func TestCORS_PreflightAdvertisesEveryMethodRoutersUse(t *testing.T) {
	w := preflight("https://app.example.com", true)
	header := w.Header().Get("Access-Control-Allow-Methods")
	got := allowedMethods(w)

	for _, m := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch, // omnichat — thiếu cái này là 8 endpoint chết im lặng
		http.MethodDelete,
		http.MethodOptions,
	} {
		if !got[m] {
			t.Errorf("Access-Control-Allow-Methods thiếu %s — browser sẽ chặn mọi request %s cross-origin (header hiện tại: %q)",
				m, m, header)
		}
	}
}

func TestCORS_PreflightAbortsWith204(t *testing.T) {
	w := preflight("https://app.example.com", true)
	if w.Code != http.StatusNoContent {
		t.Errorf("preflight trả %d, muốn 204", w.Code)
	}
}

// Allow-Headers phải phủ mọi header FE thật sự gửi. Thiếu ở đây gây đúng triệu chứng câm như thiếu
// method: preflight 204, request thật bị chặn, BE không có log.
func TestCORS_PreflightAllowsHeadersFrontendSends(t *testing.T) {
	w := preflight("https://app.example.com", true)
	allowed := strings.ToLower(w.Header().Get("Access-Control-Allow-Headers"))
	for _, h := range []string{"authorization", "content-type"} {
		if !strings.Contains(allowed, h) {
			t.Errorf("Access-Control-Allow-Headers thiếu %q (hiện tại: %q)", h, allowed)
		}
	}
}

// Prod chỉ phản chiếu Origin nằm trong whitelist; origin lạ KHÔNG được cấp Allow-Origin.
func TestCORS_ProdRejectsUnknownOrigin(t *testing.T) {
	w := preflight("https://evil.example.com", true)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("origin lạ được cấp Allow-Origin = %q, muốn rỗng", got)
	}
}

// Prod + CORS_ALLOWED_ORIGINS rỗng (chưa cấu hình) = fail-closed: không origin nào
// được cấp Allow-Origin, kể cả origin "hợp lệ trông giống prod thật".
func TestCORS_ProdEmptyAllowlistRejectsEverything(t *testing.T) {
	w := preflightWithAllowlist("https://app.example.com", true, nil)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allowlist rỗng vẫn cấp Allow-Origin = %q, muốn rỗng (fail-closed)", got)
	}
}

// Non-prod phản chiếu mọi Origin (tiện cho dev chạy FE ở port/host khác).
func TestCORS_DevReflectsAnyOrigin(t *testing.T) {
	w := preflight("https://v20-fe.tm:4200", false)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://v20-fe.tm:4200" {
		t.Errorf("Allow-Origin = %q, muốn phản chiếu origin gọi tới", got)
	}
}

// Retro F10: nhánh ngoại lệ `/api/client/*` (reflect mọi Origin ở prod) đã gỡ cùng module client —
// prefix cũ giờ theo đúng whitelist như mọi route khác, và luôn Allow-Credentials: true.
func TestCORS_ProdClientPrefixNoLongerReflects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/api/client/x", "/api/client/livechat/x"} {
		r := gin.New()
		r.Use(CORS(true, []string{"https://app.example.com"}))
		r.GET(path, func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Origin", "https://evil.example.com")
		r.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s: origin lạ được cấp Allow-Origin = %q, muốn rỗng", path, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("%s: Allow-Credentials = %q, muốn \"true\"", path, got)
		}
	}
}

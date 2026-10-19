//go:build integration

// Integration test — S-4/D-1 (crm-prod-readiness): bắt buộc đổi mật khẩu ở lần đăng nhập đầu.
//
// Phủ đầu-cuối trên DB dùng-một-lần (TEST_MYSQL_DSN; TestMain migrate SchemaAll — bỏ file seed —
// nên test tự dựng dòng admin y như `seed_admin` rồi chạy migration):
//   - migration `add_user_must_change_password` bật cờ cho admin seed (hash mặc định công khai);
//   - login admin/ChangeMe@2026 (khớp hash seed — README đúng) ⇒ user.mustChangePassword = true;
//   - API nghiệp vụ qua JWTAuth + AuthResponseHeaders ⇒ 403 code `password_change_required`;
//     /auth/me vẫn 200;
//   - tự đổi mật khẩu trùng mật khẩu cũ ⇒ 422, cờ giữ nguyên;
//   - tự đổi mật khẩu ⇒ cờ = 0 ⇒ đăng nhập lại bằng mật khẩu mới, API nghiệp vụ 200;
//   - admin đặt lại mật khẩu cho người khác ⇒ cờ người đó = 1;
//   - chạy lại migration: không lỗi, KHÔNG bật lại cờ cho admin đã đổi hash, vẫn bật cho dòng còn
//     hash mặc định.
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"erpCluvix/internal/middleware"
	tfSvc "erpCluvix/internal/modules/webapp/auth/twofa/service"
	hrHandler "erpCluvix/internal/modules/webapp/hr/user/handler"
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/token"
)

const (
	seedAdminDefaultPassword = "ChangeMe@2026"
	seedAdminDefaultHash     = "$2a$12$iNdvIu1kH0qt7jlsgZtixeTBLpRyykRHwFxVS5el3hlIUxC7fRrQK"
)

type mcpEnv struct {
	r *gin.Engine
}

func newMustChangeEnv(t *testing.T) *mcpEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	ch := cache.New(mr.Addr(), "", "mcptest", 0)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// JWTAuth verify bằng key + issuer/audience global — khớp Config của ts.
	middleware.SetJWTVerifier("mcp-test-secret", "mcptest.issuer", "mcptest.aud")
	ts := token.New(token.Config{
		Secret: "mcp-test-secret", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
		MaxSessions: 5, AuthPrefix: "mcptest:auth:", DeviceBindingMode: "off",
		Issuer: "mcptest.issuer", Audience: "mcptest.aud",
	}, rc)
	h := New(Deps{DB: testDB, Cache: ch, TokenSvc: ts, TwoFASvc: &tfSvc.Service{DB: testDB, Cache: ch, Issuer: "T"}})
	hr := hrHandler.New(hrHandler.Deps{DB: testDB, UserSvc: &hrSvc.UserService{DB: testDB, Cache: ch, TokenSvc: ts}})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	authed := r.Group("/api", middleware.JWTAuth(ts), middleware.AuthResponseHeaders(testDB, ch))
	authed.GET("/auth/me", h.Me)
	authed.POST("/hr/user/update-password", hr.UpdatePassword)
	authed.PUT("/hr/user/reset-password", hr.ResetPassword)
	// API nghiệp vụ đại diện — handler không quan trọng, chỉ cần qua đủ middleware nhóm authed.
	authed.GET("/crm/customer", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return &mcpEnv{r: r}
}

func (e *mcpEnv) do(t *testing.T, method, path, bearer string, body any) (int, map[string]any) {
	t.Helper()
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (e *mcpEnv) login(t *testing.T, username, password string) (string, bool) {
	t.Helper()
	code, out := e.do(t, http.MethodPost, "/api/auth/login", "", map[string]any{"username": username, "password": password})
	if code != http.StatusOK || out["accessToken"] == nil {
		t.Fatalf("login %s: HTTP=%d out=%v — muốn 200 + token", username, code, out)
	}
	user, _ := out["user"].(map[string]any)
	must, ok := user["mustChangePassword"].(bool)
	if !ok {
		t.Fatalf("login %s: thiếu user.mustChangePassword (bool) trong response: %v", username, out["user"])
	}
	return out["accessToken"].(string), must
}

func mustChangeFlag(t *testing.T, username string) int {
	t.Helper()
	var v int
	if err := testDB.Raw("SELECT must_change_password FROM `user` WHERE username = ?", username).Scan(&v).Error; err != nil {
		t.Fatalf("đọc cờ %s: %v", username, err)
	}
	return v
}

// assertSeedAdminForcesChange — seed_admin là nơi DUY NHẤT bật cờ cho tài khoản admin mặc định (cột
// `must_change_password` có trong create_user từ 2026-10-19, migration nâng cấp riêng đã gộp). TestMain
// migrate SchemaAll = bỏ file *_seed_* nên kiểm nội dung câu INSERT: thiếu cột hoặc giá trị 1 ⇒ admin
// cài mới đăng nhập bằng mật khẩu công khai mà không bị ép đổi.
func assertSeedAdminForcesChange(t *testing.T) {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(testhelpers.MigrationsDir(), "*_seed_admin.up.sql"))
	if len(matches) != 1 {
		t.Fatalf("tìm migration seed_admin: %v", matches)
	}
	sqlBytes, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("đọc seed_admin: %v", err)
	}
	sql := string(sqlBytes)
	if !strings.Contains(sql, "`password_hash`, `must_change_password`,") ||
		!strings.Contains(sql, "'"+seedAdminDefaultHash+"', 1,") {
		t.Fatal("seed_admin phải chèn admin với must_change_password = 1")
	}
}

func TestMustChangePassword_SeedAdmin_EndToEnd(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	// 0. Dựng lại đúng dòng admin của `seed_admin` (TestMain migrate SchemaAll = bỏ file *_seed_*):
	//    hash mặc định công khai + cờ = 1 như seed chèn.
	assertSeedAdminForcesChange(t)
	seedLoginUser(t, 890, "admin", define.UserStatusActive, 0)
	if err := testDB.Exec("UPDATE `user` SET password_hash = ?, must_change_password = 1 WHERE id = 890",
		seedAdminDefaultHash).Error; err != nil {
		t.Fatalf("dựng admin seed: %v", err)
	}
	// 1. Admin seed mang cờ.
	if got := mustChangeFlag(t, "admin"); got != define.UserMustChangePasswordYes {
		t.Fatalf("migration: admin seed must_change_password = %d — muốn 1", got)
	}
	e := newMustChangeEnv(t)

	// 2. Login bằng mật khẩu mặc định README ⇒ cờ true.
	tok, must := e.login(t, "admin", seedAdminDefaultPassword)
	if !must {
		t.Fatal("login admin seed: mustChangePassword = false — muốn true")
	}

	// 3. API nghiệp vụ bị chặn 403 với mã riêng; /auth/me vẫn qua.
	code, out := e.do(t, http.MethodGet, "/api/crm/customer", tok, nil)
	if code != http.StatusForbidden || out["code"] != define.ErrCodePasswordChangeRequired {
		t.Fatalf("API nghiệp vụ khi còn cờ: HTTP=%d out=%v — muốn 403 code=%s", code, out, define.ErrCodePasswordChangeRequired)
	}
	code, out = e.do(t, http.MethodPut, "/api/hr/user/reset-password", tok, map[string]any{"id": 1, "password": "abcdef1"})
	if code != http.StatusForbidden || out["code"] != define.ErrCodePasswordChangeRequired {
		t.Fatalf("reset-password khi còn cờ: HTTP=%d out=%v — muốn 403 (chỉ TỰ đổi được miễn)", code, out)
	}
	if code, out = e.do(t, http.MethodGet, "/api/auth/me", tok, nil); code != http.StatusOK {
		t.Fatalf("/auth/me khi còn cờ: HTTP=%d out=%v — muốn 200", code, out)
	}

	// 4. Đổi sang chính mật khẩu cũ ⇒ 422, cờ giữ nguyên.
	code, out = e.do(t, http.MethodPost, "/api/hr/user/update-password", tok, map[string]any{
		"oldpassword": seedAdminDefaultPassword, "newpassword": seedAdminDefaultPassword, "newpasswordrepeat": seedAdminDefaultPassword,
	})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("đổi sang mật khẩu cũ: HTTP=%d out=%v — muốn 422", code, out)
	}
	if got := mustChangeFlag(t, "admin"); got != define.UserMustChangePasswordYes {
		t.Fatalf("đổi trùng mật khẩu cũ bị từ chối nhưng cờ = %d — muốn giữ 1", got)
	}

	// 5. Tự đổi mật khẩu ⇒ cờ = 0.
	const newPw = "Admin-Moi-2026!"
	code, out = e.do(t, http.MethodPost, "/api/hr/user/update-password", tok, map[string]any{
		"oldpassword": seedAdminDefaultPassword, "newpassword": newPw, "newpasswordrepeat": newPw,
	})
	if code != http.StatusOK {
		t.Fatalf("tự đổi mật khẩu: HTTP=%d out=%v — muốn 200", code, out)
	}
	if got := mustChangeFlag(t, "admin"); got != define.UserMustChangePasswordNo {
		t.Fatalf("sau khi tự đổi: cờ = %d — muốn 0", got)
	}

	// 6. Đăng nhập lại (đổi mật khẩu revoke mọi session) ⇒ cờ false, API nghiệp vụ 200.
	tok, must = e.login(t, "admin", newPw)
	if must {
		t.Fatal("login sau khi đổi: mustChangePassword = true — muốn false")
	}
	if code, out = e.do(t, http.MethodGet, "/api/crm/customer", tok, nil); code != http.StatusOK {
		t.Fatalf("API nghiệp vụ sau khi đổi: HTTP=%d out=%v — muốn 200", code, out)
	}

	// 7. Admin đặt lại mật khẩu cho người khác ⇒ cờ người đó = 1; người đó login ⇒ bị ép đổi.
	seedLoginUser(t, 891, "mcp_staff891", define.UserStatusActive, 0)
	code, out = e.do(t, http.MethodPut, "/api/hr/user/reset-password", tok, map[string]any{"id": 891, "password": "Dat-Lai-123"})
	if code != http.StatusOK {
		t.Fatalf("admin reset mật khẩu người khác: HTTP=%d out=%v — muốn 200", code, out)
	}
	if got := mustChangeFlag(t, "mcp_staff891"); got != define.UserMustChangePasswordYes {
		t.Fatalf("reset bởi người khác: cờ = %d — muốn 1", got)
	}
	staffTok, must := e.login(t, "mcp_staff891", "Dat-Lai-123")
	if !must {
		t.Fatal("login sau khi bị reset: mustChangePassword = false — muốn true")
	}
	if code, _ = e.do(t, http.MethodGet, "/api/crm/customer", staffTok, nil); code != http.StatusForbidden {
		t.Fatalf("user bị reset gọi API nghiệp vụ: HTTP=%d — muốn 403", code)
	}

	// 8. Admin đã tự đổi mật khẩu giữ cờ = 0; cột tồn tại đúng 1 lần trong schema.
	if got := mustChangeFlag(t, "admin"); got != define.UserMustChangePasswordNo {
		t.Fatalf("admin đã đổi mật khẩu: cờ=%d — muốn 0", got)
	}
	var nCol int
	testDB.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() " +
		"AND TABLE_NAME = 'user' AND COLUMN_NAME = 'must_change_password'").Scan(&nCol)
	if nCol != 1 {
		t.Fatalf("cột must_change_password: %d — muốn đúng 1", nCol)
	}
}

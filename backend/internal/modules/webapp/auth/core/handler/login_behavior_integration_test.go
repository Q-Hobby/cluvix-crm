//go:build integration

// Integration test — GIỮ HÀNH VI đăng nhập (story-58), viết TRƯỚC khi gỡ thương hiệu khỏi auth/core.
//
// Phủ: sai mật khẩu, tài khoản không active, khoá theo tài khoản (5 lần sai), giới hạn thiết bị
// (MaxSessions), 2FA challenge (thiết bị chưa tin / đã tin), trust-days khi "nhớ thiết bị"
// (`brand.Profile(db).TotpTrustDays`). Redis = miniredis (in-memory, KHÔNG
// đụng Redis thật). DB = TEST_MYSQL_DSN (dùng-một-lần).
//
// Hồ sơ thương hiệu = dòng `business_profile` id=1 (story-62; bảng 1 dòng, brand đọc id nhỏ nhất).
package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	twofaSvc "erpCluvix/internal/modules/webapp/auth/twofa/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/token"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "auth_core_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

const (
	loginProfileID = 1
	loginPassword  = "Dung-Mat-Khau-9"
	backupCode     = "a1b2c3d4"
)

type loginEnv struct {
	h  *Handler
	r  *gin.Engine
	ts *token.Service
}

func newLoginEnv(t *testing.T, maxSessions int) *loginEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	ch := cache.New(mr.Addr(), "", "s58test", 0)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ts := token.New(token.Config{
		Secret: "s58-test-secret", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
		MaxSessions: maxSessions, AuthPrefix: "s58test:auth:", DeviceBindingMode: "off",
		Issuer: "s58test.issuer", Audience: "s58test.aud",
	}, rc)
	h := New(Deps{DB: testDB, Cache: ch, TokenSvc: ts, TwoFASvc: &twofaSvc.Service{DB: testDB, Cache: ch, Issuer: "T"}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/login/2fa", h.Login2FA)
	return &loginEnv{h: h, r: r, ts: ts}
}

func (e *loginEnv) post(t *testing.T, path string, body any) (int, map[string]any) {
	t.Helper()
	code, out, _ := e.postCookies(t, path, body, nil)
	return code, out
}

// postCookies gửi kèm cookie và trả cookie response set (remember-device dùng cookie HttpOnly).
func (e *loginEnv) postCookies(t *testing.T, path string, body any, cookies []*http.Cookie) (int, map[string]any, []*http.Cookie) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Result().Cookies()
}

// cookieNamed lấy cookie theo tên (nil nếu không có).
func cookieNamed(cookies []*http.Cookie, name string) *http.Cookie {
	for _, ck := range cookies {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

// setTrustDays đặt totp_trust_days trên hồ sơ thương hiệu (khôi phục 30 khi xong).
func setTrustDays(t *testing.T, days int) {
	t.Helper()
	testDB.Exec("INSERT IGNORE INTO business_profile (id, name) VALUES (?, 'A')", loginProfileID)
	if err := testDB.Exec("UPDATE business_profile SET totp_trust_days = ? WHERE id = ?", days, loginProfileID).Error; err != nil {
		t.Fatalf("set trust days: %v", err)
	}
	brand.Invalidate()
	t.Cleanup(func() {
		testDB.Exec("UPDATE business_profile SET totp_trust_days = 30 WHERE id = ?", loginProfileID)
		brand.Invalidate()
	})
}

// seedLoginUser tạo user + hồ sơ (+ backup code 2FA khi totp=1).
func seedLoginUser(t *testing.T, id int, username string, status, totp int) {
	t.Helper()
	testDB.Exec("INSERT IGNORE INTO business_profile (id, name) VALUES (?, 'A')", loginProfileID)
	testDB.Exec("DELETE FROM user_2fa_backup_code WHERE user_id = ?", id)
	testDB.Exec("DELETE FROM user_device WHERE user_id = ?", id)
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", id)
	testDB.Exec("DELETE FROM `user` WHERE id = ?", id)
	hash, err := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.Exec("INSERT INTO `user` (id, username, password_hash, status, branchId, totp_enabled, "+
		"created_at, updated_at) VALUES (?, ?, ?, ?, '[]', ?, 0, 0)", id, username, string(hash), status, totp).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'L')",
		id).Error; err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	if totp == 1 {
		sum := sha256.Sum256([]byte(backupCode))
		if err := testDB.Exec("INSERT INTO user_2fa_backup_code (user_id, code_hash, created_at) VALUES (?, ?, 0)",
			id, hex.EncodeToString(sum[:])).Error; err != nil {
			t.Fatalf("seed backup code: %v", err)
		}
	}
}

func TestLogin_WrongPassword_401(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	e := newLoginEnv(t, 3)
	seedLoginUser(t, 881, "s58login881", 10, 0)
	code, out := e.post(t, "/api/auth/login", map[string]any{"username": "s58login881", "password": "sai-roi"})
	if code != http.StatusUnauthorized || out["accessToken"] != nil {
		t.Fatalf("sai mật khẩu: HTTP=%d out=%v — muốn 401, không token", code, out)
	}
	// Đối chứng: đúng mật khẩu ⇒ có token.
	code, out = e.post(t, "/api/auth/login", map[string]any{"username": "s58login881", "password": loginPassword})
	if code != http.StatusOK || out["accessToken"] == nil {
		t.Fatalf("đúng mật khẩu: HTTP=%d out=%v — muốn 200 + token", code, out)
	}
}

func TestLogin_InactiveAccount_401(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	e := newLoginEnv(t, 3)
	seedLoginUser(t, 882, "s58login882", 0, 0)
	code, out := e.post(t, "/api/auth/login", map[string]any{"username": "s58login882", "password": loginPassword})
	if code != http.StatusUnauthorized || out["accessToken"] != nil {
		t.Fatalf("tài khoản khoá: HTTP=%d out=%v — muốn 401", code, out)
	}
}

// Khoá theo tài khoản: 5 lần sai ⇒ lần thứ 6 bị 429 KỂ CẢ mật khẩu đúng.
func TestLogin_AccountLockout_429(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	e := newLoginEnv(t, 3)
	seedLoginUser(t, 883, "s58login883", 10, 0)
	for i := 0; i < loginFailUserMax; i++ {
		if code, _ := e.post(t, "/api/auth/login", map[string]any{"username": "s58login883", "password": "sai"}); code != http.StatusUnauthorized {
			t.Fatalf("lần sai %d: HTTP=%d — muốn 401", i+1, code)
		}
	}
	code, out := e.post(t, "/api/auth/login", map[string]any{"username": "s58login883", "password": loginPassword})
	if code != http.StatusTooManyRequests || out["accessToken"] != nil {
		t.Fatalf("sau %d lần sai: HTTP=%d out=%v — muốn 429, không token", loginFailUserMax, code, out)
	}
}

// Giới hạn thiết bị: MaxSessions=2, đăng nhập 3 thiết bị ⇒ chỉ còn ĐÚNG 2 fingerprint hoạt động.
// Không assert thiết bị bị đẩy ra: login cùng giây có score zset bằng nhau, tie-break theo rid ngẫu
// nhiên (cùng lý do pkg/token TestDeviceLimitCap) — cap mới là bất biến bảo mật.
func TestLogin_DeviceLimit(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	e := newLoginEnv(t, 2)
	seedLoginUser(t, 884, "s58login884", 10, 0)
	for _, fp := range []string{"fpDeviceAAAA", "fpDeviceBBBB", "fpDeviceCCCC"} {
		code, out := e.post(t, "/api/auth/login", map[string]any{
			"username": "s58login884", "password": loginPassword, "device_fingerprint": fp})
		if code != http.StatusOK || out["accessToken"] == nil {
			t.Fatalf("login %s: HTTP=%d out=%v", fp, code, out)
		}
	}
	active := e.ts.ActiveFingerprints(884)
	if len(active) != 2 {
		t.Fatalf("fingerprint hoạt động = %v — muốn đúng 2 (MaxSessions)", active)
	}
}

// 2FA: user đã bật 2FA + thiết bị chưa tin ⇒ challenge, KHÔNG token. Fingerprint còn hạn tin
// nhưng KHÔNG kèm token remember-device (kẻ có mật khẩu đoán fingerprint) ⇒ vẫn challenge.
// Kèm đúng cookie token đã cấp sau 2FA ⇒ token; token sai ⇒ challenge.
func TestLogin_2FAChallenge(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setTrustDays(t, 30)
	e := newLoginEnv(t, 3)
	seedLoginUser(t, 885, "s58login885", 10, 1)
	login := func(cookies []*http.Cookie) (int, map[string]any) {
		code, out, _ := e.postCookies(t, "/api/auth/login", map[string]any{
			"username": "s58login885", "password": loginPassword, "device_fingerprint": "fpTrust885x"}, cookies)
		return code, out
	}
	code, out := login(nil)
	if code != http.StatusOK || out["requires_2fa"] != true || out["accessToken"] != nil || out["challenge_token"] == nil {
		t.Fatalf("thiết bị chưa tin: HTTP=%d out=%v — muốn requires_2fa, không token", code, out)
	}

	// Fingerprint đã từng được tin (dữ liệu cũ, chưa có hash) — chỉ fingerprint thì KHÔNG được bỏ 2FA.
	future := time.Now().Add(time.Hour).Unix()
	if err := testDB.Exec("INSERT INTO user_device (user_id, fingerprint, status, totp_trusted_until, created_at, updated_at) "+
		"VALUES (?, 'fpTrust885x', 1, ?, 0, 0)", 885, future).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}
	if code, out = login(nil); code != http.StatusOK || out["requires_2fa"] != true || out["accessToken"] != nil {
		t.Fatalf("chỉ fingerprint: HTTP=%d out=%v — muốn requires_2fa", code, out)
	}

	cookies := login2FAFlow(t, e, "s58login885", "fpTrust885x")
	trust := cookieNamed(cookies, deviceTrustCookieName(885))
	if trust == nil || trust.Value == "" || !trust.HttpOnly {
		t.Fatalf("thiếu cookie remember-device HttpOnly: %v", cookies)
	}
	if code, out = login([]*http.Cookie{trust}); code != http.StatusOK || out["accessToken"] == nil || out["requires_2fa"] == true {
		t.Fatalf("thiết bị đã tin + cookie: HTTP=%d out=%v — muốn token, không challenge", code, out)
	}
	forged := &http.Cookie{Name: trust.Name, Value: strings.Repeat("0", len(trust.Value))}
	if code, out = login([]*http.Cookie{forged}); code != http.StatusOK || out["requires_2fa"] != true {
		t.Fatalf("cookie sai: HTTP=%d out=%v — muốn requires_2fa", code, out)
	}
}

// trustedUntil đọc hạn tin thiết bị (nil ⇒ chưa tin).
func trustedUntil(t *testing.T, uid int, fp string) *int64 {
	t.Helper()
	var until *int64
	if err := testDB.Raw("SELECT totp_trusted_until FROM user_device WHERE user_id = ? AND fingerprint = ?", uid, fp).
		Scan(&until).Error; err != nil {
		t.Fatalf("đọc trusted_until: %v", err)
	}
	return until
}

// login2FAFlow: login ⇒ challenge ⇒ verify backup code (remember_device) ⇒ token. Trả cookie bước 2.
func login2FAFlow(t *testing.T, e *loginEnv, username, fp string) []*http.Cookie {
	t.Helper()
	code, out := e.post(t, "/api/auth/login", map[string]any{
		"username": username, "password": loginPassword, "device_fingerprint": fp})
	ch, _ := out["challenge_token"].(string)
	if code != http.StatusOK || ch == "" {
		t.Fatalf("bước 1: HTTP=%d out=%v", code, out)
	}
	code, out, cookies := e.postCookies(t, "/api/auth/login/2fa", map[string]any{
		"challenge_token": ch, "code": backupCode, "remember_device": true}, nil)
	if code != http.StatusOK || out["accessToken"] == nil {
		t.Fatalf("bước 2: HTTP=%d out=%v", code, out)
	}
	return cookies
}

// AC2(d) — trust-days = giá trị của hồ sơ thương hiệu.
func TestLogin2FA_TrustDaysFromBrand(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setTrustDays(t, 7)
	e := newLoginEnv(t, 3)
	seedLoginUser(t, 886, "s58login886", 10, 1)
	before := time.Now().Unix()
	login2FAFlow(t, e, "s58login886", "fpTrust886x")
	until := trustedUntil(t, 886, "fpTrust886x")
	want := before + 7*24*3600
	if until == nil || *until < want-5 || *until > time.Now().Unix()+7*24*3600+5 {
		t.Fatalf("totp_trusted_until = %v, muốn ≈ now + 7 ngày (%d)", until, want)
	}
}

// trust-days = 0 ⇒ KHÔNG nhớ thiết bị (luôn hỏi lại) — 0 là giá trị hợp lệ, không rơi về 30.
func TestLogin2FA_TrustDaysZero_NoTrust(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setTrustDays(t, 0)
	e := newLoginEnv(t, 3)
	seedLoginUser(t, 887, "s58login887", 10, 1)
	login2FAFlow(t, e, "s58login887", "fpTrust887x")
	if until := trustedUntil(t, 887, "fpTrust887x"); until != nil && *until > time.Now().Unix() {
		t.Fatalf("trust-days = 0 mà thiết bị vẫn được tin tới %d", *until)
	}
}

// Story-66: cầu FE-compat (story-62) đã gỡ — `/auth/me` KHÔNG còn `user.companyId` và
// `userProfile.companyActive_id` (FE story-43 không còn đọc). `branchId` vẫn đọc từ cột thật.
func TestMe_S66_KhongConKeyFECompat(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedLoginUser(t, 888, "s62me888", 10, 0)
	if err := testDB.Exec("UPDATE `user` SET branchId = '[3]' WHERE id = 888").Error; err != nil {
		t.Fatalf("seed branchId: %v", err)
	}
	e := newLoginEnv(t, 5)
	e.r.GET("/api/auth/me", func(c *gin.Context) { c.Set("userID", uint(888)); e.h.Me(c) })
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		User struct {
			ID          uint                       `json:"id"`
			BranchID    []uint                     `json:"branchId"`
			UserProfile map[string]json.RawMessage `json:"userProfile"`
		} `json:"user"`
	}
	var raw struct {
		User map[string]json.RawMessage `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v — %s", err, w.Body.String())
	}
	if out.User.ID != 888 || len(out.User.BranchID) != 1 || out.User.BranchID[0] != 3 {
		t.Errorf("user id=%d branchId=%v — muốn 888/[3]", out.User.ID, out.User.BranchID)
	}
	if len(out.User.UserProfile) == 0 {
		t.Fatalf("thiếu userProfile — body=%s", w.Body.String())
	}
	if v, has := raw.User["companyId"]; has {
		t.Errorf("user.companyId phải KHÔNG còn (story-66), nhận %s", v)
	}
	if v, has := out.User.UserProfile["companyActive_id"]; has {
		t.Errorf("userProfile.companyActive_id phải KHÔNG còn (story-66), nhận %s", v)
	}
}

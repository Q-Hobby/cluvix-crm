//go:build integration

// Integration test — GET/PUT `/api/config/business-profile` (story-63).
//
// Bảng singleton, dùng chung `testDB` giữa các test trong package ⇒ mỗi test tự dọn
// `DELETE FROM business_profile` trước khi chạy để không phụ thuộc thứ tự thực thi.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/tz"
)

func resetBusinessProfile(t *testing.T) {
	t.Helper()
	if err := testDB.Exec("DELETE FROM business_profile").Error; err != nil {
		t.Fatalf("reset business_profile: %v", err)
	}
	brand.Invalidate()
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM business_profile")
		brand.Invalidate()
	})
}

func newRouter(t *testing.T, uid uint) (*gin.Engine, *Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := New(Deps{DB: testDB})
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uid); c.Next() })
	r.GET("/api/config/business-profile", h.GetBusinessProfile)
	r.PUT("/api/config/business-profile", h.UpdateBusinessProfile)
	return r, h
}

func doGet(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/config/business-profile", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func doPutJSON(r *gin.Engine, body map[string]interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/config/business-profile", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// GET khi chưa có dòng ⇒ giá trị mặc định như pkg/brand (AC1).
func TestGetBusinessProfile_NoRow_ReturnsDefaults(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	r, _ := newRouter(t, 1)

	w := doGet(r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET phải 200, got %d — %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			ID       uint   `json:"id"`
			Timezone string `json:"timezone"`
			Currency string `json:"currency"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	if resp.Data.ID != 0 {
		t.Errorf("chưa có dòng thì id phải 0, got %d", resp.Data.ID)
	}
	if resp.Data.Timezone != brand.DefaultTimezone || resp.Data.Currency != brand.DefaultCurrency {
		t.Errorf("mặc định sai: timezone=%s currency=%s", resp.Data.Timezone, resp.Data.Currency)
	}
}

// PUT khi chưa có dòng ⇒ tạo mới (upsert, giống ngữ nghĩa onboarding — story không nói khác).
func TestPutBusinessProfile_CreateWhenMissing(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	r, _ := newRouter(t, 5)

	w := doPutJSON(r, map[string]interface{}{
		"updated_at": 0,
		"name":       "ABC Clinic",
		"code":       "ABC",
		"timezone":   "Asia/Ho_Chi_Minh",
		"currency":   "VND",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT tạo mới phải 200, got %d — %s", w.Code, w.Body.String())
	}
	var count int64
	testDB.Table("business_profile").Count(&count)
	if count != 1 {
		t.Fatalf("phải tạo đúng 1 dòng, có %d", count)
	}
	var name string
	testDB.Table("business_profile").Select("name").Scan(&name)
	if name != "ABC Clinic" {
		t.Errorf("name = %q, muốn ABC Clinic", name)
	}

	// updated_at sai (khác 0) khi CHƯA có dòng ⇒ 409 (race tạo trùng).
	resetBusinessProfile(t)
	w2 := doPutJSON(r, map[string]interface{}{"updated_at": 12345, "name": "X", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if w2.Code != http.StatusConflict {
		t.Fatalf("updated_at != 0 khi chưa có dòng phải 409, got %d — %s", w2.Code, w2.Body.String())
	}
}

// Optimistic lock: version đúng ⇒ ghi thành công + updated_at tăng; version cũ (stale) ⇒ 409,
// KHÔNG ghi đè dữ liệu (khuôn print_template story-57).
func TestPutBusinessProfile_OptimisticLock(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	r, _ := newRouter(t, 7)

	// Seed 1 dòng.
	w := doPutJSON(r, map[string]interface{}{"updated_at": 0, "name": "Gốc", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if w.Code != http.StatusOK {
		t.Fatalf("seed PUT phải 200: %d — %s", w.Code, w.Body.String())
	}
	var seeded struct {
		Data struct {
			UpdatedAt int64 `json:"updated_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &seeded); err != nil {
		t.Fatalf("decode seed: %v", err)
	}
	staleVersion := seeded.Data.UpdatedAt

	// Sửa lần 1 với version đúng ⇒ 200, updated_at phải tăng NGHIÊM NGẶT.
	w1 := doPutJSON(r, map[string]interface{}{"updated_at": staleVersion, "name": "Sửa lần 1", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if w1.Code != http.StatusOK {
		t.Fatalf("sửa lần 1 phải 200: %d — %s", w1.Code, w1.Body.String())
	}
	var afterFirst struct {
		Data struct {
			UpdatedAt int64 `json:"updated_at"`
		} `json:"data"`
	}
	json.Unmarshal(w1.Body.Bytes(), &afterFirst)
	if afterFirst.Data.UpdatedAt <= staleVersion {
		t.Fatalf("updated_at phải tăng nghiêm ngặt: %d → %d", staleVersion, afterFirst.Data.UpdatedAt)
	}

	// Sửa lần 2 vẫn dùng version CŨ (staleVersion) ⇒ 409, dữ liệu KHÔNG bị ghi đè.
	w2 := doPutJSON(r, map[string]interface{}{"updated_at": staleVersion, "name": "Sửa lần 2 (stale)", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if w2.Code != http.StatusConflict {
		t.Fatalf("version cũ phải 409, got %d — %s", w2.Code, w2.Body.String())
	}
	var name string
	testDB.Table("business_profile").Select("name").Scan(&name)
	if name != "Sửa lần 1" {
		t.Fatalf("tên bị ghi đè bởi request stale: %q", name)
	}
}

// Validate: timezone rỗng/không hợp lệ, allowed_ips không phải mảng IP/CIDR hợp lệ ⇒ 422.
func TestPutBusinessProfile_ValidationErrors(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	r, _ := newRouter(t, 9)

	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"timezone rỗng", map[string]interface{}{"updated_at": 0, "timezone": "", "currency": "VND"}},
		{"timezone không hợp lệ", map[string]interface{}{"updated_at": 0, "timezone": "Khong/Ton_Tai", "currency": "VND"}},
		{"allowed_ips không hợp lệ", map[string]interface{}{
			"updated_at": 0, "timezone": "Asia/Ho_Chi_Minh", "currency": "VND",
			"allowed_ips": []string{"not-an-ip"},
		}},
		{"totp_trust_days âm", map[string]interface{}{
			"updated_at": 0, "timezone": "Asia/Ho_Chi_Minh", "currency": "VND", "totp_trust_days": -1,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doPutJSON(r, tc.body)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: muốn 422, got %d — %s", tc.name, w.Code, w.Body.String())
			}
		})
		var count int64
		testDB.Table("business_profile").Count(&count)
		if count != 0 {
			t.Fatalf("%s: payload sai KHÔNG được tạo dòng, có %d", tc.name, count)
		}
	}
}

const s63PolicyPath = "/api/crm/customer"

// AC3 — PUT require_2fa=1 phải có hiệu lực NGAY ở request kế tiếp (brand.Invalidate() +
// middleware.InvalidateUserScope SAU commit), không đợi hết TTL cache 5 phút của pkg/brand.
func TestPutBusinessProfile_Require2FA_TakesEffectImmediately(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	r, _ := newRouter(t, 11)

	// Seed dòng ban đầu KHÔNG bắt 2FA.
	seed := doPutJSON(r, map[string]interface{}{"updated_at": 0, "name": "A", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if seed.Code != http.StatusOK {
		t.Fatalf("seed phải 200: %d — %s", seed.Code, seed.Body.String())
	}
	var after0 struct {
		Data struct {
			UpdatedAt int64 `json:"updated_at"`
		} `json:"data"`
	}
	json.Unmarshal(seed.Body.Bytes(), &after0)

	// User chưa enroll 2FA — không FK tới `user` (created_by/updated_by KHÔNG có FK, Hard Rule
	// 2026-09-05) nên chỉ cần seed đúng bảng `user`/`user_profile` cho middleware đọc.
	const uid = 20631
	testDB.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", uid)
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid)
	testDB.Exec("DELETE FROM `user` WHERE id = ?", uid)
	if err := testDB.Exec("INSERT INTO `user` (id, username, password_hash, status, branchId, "+
		"totp_enabled, created_at, updated_at) VALUES (?, 'pol_s63', 'x', 10, '[]', 0, 0, 0)", uid).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'P')", uid)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", uid)
		testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid)
		testDB.Exec("DELETE FROM `user` WHERE id = ?", uid)
	})

	callPolicy := func() (int, string) {
		gin.SetMode(gin.TestMode)
		pr := gin.New()
		pr.Use(func(c *gin.Context) { c.Set("userID", uint(uid)); c.Next() })
		pr.Use(middleware.AuthResponseHeaders(testDB, nil))
		pr.GET(s63PolicyPath, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		req := httptest.NewRequest(http.MethodGet, s63PolicyPath, nil)
		w := httptest.NewRecorder()
		pr.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}

	// Trước PUT: không policy ⇒ qua được.
	if code, body := callPolicy(); code != http.StatusOK {
		t.Fatalf("trước PUT phải 200 (chưa có policy): %d — %s", code, body)
	}

	// PUT bật require_2fa=1.
	w := doPutJSON(r, map[string]interface{}{
		"updated_at": after0.Data.UpdatedAt, "name": "A", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND",
		"require_2fa": 1,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT require_2fa=1 phải 200: %d — %s", w.Code, w.Body.String())
	}

	// Request KẾ TIẾP (không chờ TTL) phải áp policy mới ngay.
	code, body := callPolicy()
	if code != http.StatusForbidden || !strings.Contains(body, "2fa_setup_required") {
		t.Fatalf("sau PUT require_2fa=1, request kế tiếp phải 403 2fa_setup_required ngay: HTTP=%d body=%s", code, body)
	}
}

// AC3 — PUT timezone mới phải có hiệu lực ngay ở `tz.LocFromContext` (qua pkg/brand cache).
func TestPutBusinessProfile_Timezone_TakesEffectImmediately(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	r, _ := newRouter(t, 13)

	seed := doPutJSON(r, map[string]interface{}{"updated_at": 0, "name": "A", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if seed.Code != http.StatusOK {
		t.Fatalf("seed phải 200: %d — %s", seed.Code, seed.Body.String())
	}
	var after0 struct {
		Data struct {
			UpdatedAt int64 `json:"updated_at"`
		} `json:"data"`
	}
	json.Unmarshal(seed.Body.Bytes(), &after0)

	if got := tz.LocFromContext(nil, testDB).String(); got != "Asia/Ho_Chi_Minh" {
		t.Fatalf("trước PUT: timezone phải Asia/Ho_Chi_Minh, got %s", got)
	}

	w := doPutJSON(r, map[string]interface{}{
		"updated_at": after0.Data.UpdatedAt, "name": "A", "timezone": "Asia/Tokyo", "currency": "VND",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT đổi timezone phải 200: %d — %s", w.Code, w.Body.String())
	}

	if got := tz.LocFromContext(nil, testDB).String(); got != "Asia/Tokyo" {
		t.Fatalf("sau PUT: timezone phải đổi NGAY sang Asia/Tokyo, got %s", got)
	}
}

// AC3 — mối nối cache thật (miniredis): scope bảo mật cache 5 phút theo TỪNG user. User KHÁC người
// sửa đã nạp scope cũ (chưa bắt 2FA) trước PUT ⇒ sau PUT require_2fa=1 phải bị chặn NGAY, không đợi
// TTL. Test trên chạy cache nil nên không canh được nhánh này (QA story-63: handler từng chỉ xoá
// scope của người sửa).
func TestPutBusinessProfile_Require2FA_XoaScopeCacheCuaUserKhac(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetBusinessProfile(t)
	mr := miniredis.RunT(t)
	ch := cache.New(mr.Addr(), "", "", 0)

	const editorID = 11
	h := New(Deps{DB: testDB, Cache: ch})
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(editorID)); c.Next() })
	r.PUT("/api/config/business-profile", h.UpdateBusinessProfile)

	seed := doPutJSON(r, map[string]interface{}{"updated_at": 0, "name": "A", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND"})
	if seed.Code != http.StatusOK {
		t.Fatalf("seed phải 200: %d — %s", seed.Code, seed.Body.String())
	}
	var after0 struct {
		Data struct {
			UpdatedAt int64 `json:"updated_at"`
		} `json:"data"`
	}
	json.Unmarshal(seed.Body.Bytes(), &after0)

	const uid = 20632
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid)
	testDB.Exec("DELETE FROM `user` WHERE id = ?", uid)
	if err := testDB.Exec("INSERT INTO `user` (id, username, password_hash, status, branchId, "+
		"totp_enabled, created_at, updated_at) VALUES (?, 'pol_s63b', 'x', 10, '[]', 0, 0, 0)", uid).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'P')", uid)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid)
		testDB.Exec("DELETE FROM `user` WHERE id = ?", uid)
	})

	callPolicy := func() (int, string) {
		pr := gin.New()
		pr.Use(func(c *gin.Context) { c.Set("userID", uint(uid)); c.Next() })
		pr.Use(middleware.AuthResponseHeaders(testDB, ch))
		pr.GET(s63PolicyPath, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		pr.ServeHTTP(w, httptest.NewRequest(http.MethodGet, s63PolicyPath, nil))
		return w.Code, w.Body.String()
	}

	if code, body := callPolicy(); code != http.StatusOK {
		t.Fatalf("trước PUT phải 200 (chưa có policy): %d — %s", code, body)
	}
	scopeKey := fmt.Sprintf("user_scope:%d", uid)
	if !mr.Exists(scopeKey) {
		t.Fatalf("tiền điều kiện: %s chưa được cache — test không chứng minh được việc xoá cache", scopeKey)
	}

	w := doPutJSON(r, map[string]interface{}{
		"updated_at": after0.Data.UpdatedAt, "name": "A", "timezone": "Asia/Ho_Chi_Minh", "currency": "VND",
		"require_2fa": 1,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT require_2fa=1 phải 200: %d — %s", w.Code, w.Body.String())
	}

	code, body := callPolicy()
	if code != http.StatusForbidden || !strings.Contains(body, "2fa_setup_required") {
		t.Fatalf("user khác người sửa phải 403 2fa_setup_required ngay sau PUT: HTTP=%d body=%s", code, body)
	}
}

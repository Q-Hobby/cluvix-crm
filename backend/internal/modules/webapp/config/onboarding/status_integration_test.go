//go:build integration

// Integration test — `GET /api/config/onboarding/status` chạy qua `middleware.AuthResponseHeaders`
// THẬT + route đăng ký bằng `RegisterRoutes` thật, trên DB dùng-một-lần.
package onboarding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/config/onboarding/handler"
	"erpCluvix/internal/testhelpers"
)

const statusPath = "/api/config/onboarding/status"

// seedUser dựng user active + hồ sơ, chưa có chi nhánh (DB trắng). `super` ⇒ gán role `admin`.
func seedUser(t *testing.T, userID int, super bool) {
	t.Helper()
	testDB.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", userID)
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", userID)
	testDB.Exec("DELETE FROM `user` WHERE id = ?", userID)
	if err := testDB.Exec("INSERT INTO `user` (id, username, password_hash, status, branchId, created_at, updated_at) "+
		"VALUES (?, ?, 'x', 10, '[]', 0, 0)", userID, "onb"+strconv.Itoa(userID)).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'U')", userID).Error; err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	if super {
		if err := testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type) VALUES ('admin', 1)").Error; err != nil {
			t.Fatalf("seed rbac_auth_item: %v", err)
		}
		if err := testDB.Exec("INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES ('admin', ?, 0)", userID).Error; err != nil {
			t.Fatalf("seed rbac_auth_assignment: %v", err)
		}
	}
}

func requireBranchEmpty(t *testing.T) {
	t.Helper()
	var n int64
	if err := testDB.Table("branch").Count(&n).Error; err != nil {
		t.Fatalf("đếm branch: %v", err)
	}
	if n != 0 {
		t.Fatalf("tiền điều kiện: bảng branch phải rỗng, đang có %d dòng", n)
	}
}

// call chạy chuỗi thật: set userID (việc của JWTAuth) → AuthResponseHeaders THẬT (ch=nil ⇒ đọc thẳng
// DB) → group /api/config → RegisterRoutes của module.
func call(t *testing.T, userID int, path string) (int, string) {
	t.Helper()
	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(userID)); c.Next() })
	r.Use(middleware.AuthResponseHeaders(testDB, nil))
	RegisterRoutes(r.Group("/api/config"), handler.Deps{DB: testDB})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Code, w.Body.String()
}

// canBootstrap đọc `data.can_bootstrap` — thiếu key là FAIL (không coi nil là false).
func canBootstrap(t *testing.T, body string) bool {
	t.Helper()
	var resp struct {
		Data struct {
			CanBootstrap *bool `json:"can_bootstrap"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("body không phải JSON: %v — %s", err, body)
	}
	if resp.Data.CanBootstrap == nil {
		t.Fatalf("thiếu data.can_bootstrap trong body: %s", body)
	}
	return *resp.Data.CanBootstrap
}

// AC1 + AC4: DB trắng (chưa chi nhánh), super ⇒ đi qua middleware, can_bootstrap = true.
func TestStatus_EmptyDB_Super_CanBootstrap(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	requireBranchEmpty(t)
	seedUser(t, 901, true)

	code, body := call(t, 901, statusPath)
	if code != http.StatusOK {
		t.Fatalf("HTTP = %d, muốn 200 (gate KHÔNG được chặn path onboarding). Body: %s", code, body)
	}
	if !canBootstrap(t, body) {
		t.Errorf("can_bootstrap = false, muốn true (DB rỗng + super). Body: %s", body)
	}
}

// AC2 + AC4: DB trắng, KHÔNG super ⇒ 200 + false (không 403 — FE cần phân nhánh UI).
func TestStatus_EmptyDB_NonSuper_False(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	requireBranchEmpty(t)
	seedUser(t, 902, false)

	code, body := call(t, 902, statusPath)
	if code != http.StatusOK {
		t.Fatalf("HTTP = %d, muốn 200. Body: %s", code, body)
	}
	if canBootstrap(t, body) {
		t.Errorf("can_bootstrap = true cho user KHÔNG super — cửa thiết lập mở cho người không có quyền. Body: %s", body)
	}
}

// AC3 (story-62): DB đã có ≥1 chi nhánh, super CHƯA được gán chi nhánh ⇒ false (cửa tự đóng).
func TestStatus_HasBranch_Super_False(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedUser(t, 904, true)
	if err := testDB.Exec("INSERT INTO branch (id, name, code) VALUES (5001, 'Existing', 'EX5001')").Error; err != nil {
		t.Fatalf("seed branch: %v", err)
	}
	t.Cleanup(func() { testDB.Exec("DELETE FROM branch WHERE id = 5001") })

	code, body := call(t, 904, statusPath)
	if code != http.StatusOK {
		t.Fatalf("HTTP = %d, muốn 200. Body: %s", code, body)
	}
	if canBootstrap(t, body) {
		t.Errorf("can_bootstrap = true khi DB đã có chi nhánh — cửa thiết lập không tự đóng. Body: %s", body)
	}
}

// story-10 AC1: lỗi SQL ở query QUYỀN (`IsSuperUserE`) cũng phải lộ ra 500, y như query đếm
// `branch` — trước đây handler gọi bản nuốt lỗi `IsSuperUser` nên DB lỗi ⇒ 200 +
// `can_bootstrap:false`.
//
// Gây lỗi THẬT trên đúng query đó bằng `RENAME TABLE rbac_auth_assignment` (1146). Bảng `branch`
// phải rỗng để luồng đi tới được nhánh kiểm quyền. Cố ý KHÔNG gắn `AuthResponseHeaders` (cùng lý do
// ca dưới: middleware cũng đọc bảng quyền thì status code không còn chứng minh gì về handler).
//
// Mutation chứng minh lưới sống: đổi lại `helpers.IsSuperUser(h.DB, uid)` ⇒ ca này ĐỎ (200 + false).
func TestStatus_RoleQueryError_500(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	requireBranchEmpty(t)
	seedUser(t, 906, true)

	if err := testDB.Exec("RENAME TABLE rbac_auth_assignment TO rbac_auth_assignment_onbdebt_hidden").Error; err != nil {
		t.Fatalf("không gây được lỗi query quyền: %v", err)
	}
	// Đăng ký NGAY sau rename (cleanup LIFO) để bảng được trả lại trước mọi cleanup khác.
	t.Cleanup(func() {
		if err := testDB.Exec("RENAME TABLE rbac_auth_assignment_onbdebt_hidden TO rbac_auth_assignment").Error; err != nil {
			t.Errorf("KHÔNG khôi phục được bảng rbac_auth_assignment — các test sau trong package sẽ hỏng: %v", err)
		}
	})

	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(906)); c.Next() })
	RegisterRoutes(r.Group("/api/config"), handler.Deps{DB: testDB})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, statusPath, nil))
	body := w.Body.String()
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP = %d, muốn 500 — lỗi SQL ở query quyền đang bị che thành can_bootstrap. Body: %s", w.Code, body)
	}
	if strings.Contains(body, "can_bootstrap") {
		t.Errorf("body chứa can_bootstrap dù query quyền lỗi — lỗi bị che thành kết quả. Body: %s", body)
	}
}

// Lỗi SQL ở query đếm `branch` PHẢI lộ ra 500, không bị che thành `can_bootstrap` (handler/status.go
// nhánh IsQueryError). Gây lỗi THẬT trên đúng query đó bằng `RENAME TABLE branch` (1146 table doesn't
// exist); caller là super nên nếu lỗi bị nuốt thì `ids` rỗng ⇒ 200 + `can_bootstrap: true`.
//
// Cố ý KHÔNG gắn `AuthResponseHeaders` ở ca này (middleware có thể đọc bảng liên quan — status code
// khi đó không còn chứng minh gì về handler).
func TestStatus_BranchQueryError_500(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedUser(t, 905, true)

	if err := testDB.Exec("RENAME TABLE branch TO branch_onbwiz_hidden").Error; err != nil {
		t.Fatalf("không gây được lỗi query branch: %v", err)
	}
	// Đăng ký NGAY sau rename (cleanup chạy LIFO) để bảng được trả lại trước mọi cleanup khác.
	t.Cleanup(func() {
		if err := testDB.Exec("RENAME TABLE branch_onbwiz_hidden TO branch").Error; err != nil {
			t.Errorf("KHÔNG khôi phục được bảng branch — các test sau trong package sẽ hỏng: %v", err)
		}
	})

	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(905)); c.Next() })
	RegisterRoutes(r.Group("/api/config"), handler.Deps{DB: testDB})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, statusPath, nil))
	body := w.Body.String()
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HTTP = %d, muốn 500 — lỗi SQL khi đếm branch đang bị che. Body: %s", w.Code, body)
	}
	if strings.Contains(body, "can_bootstrap") {
		t.Errorf("body chứa can_bootstrap dù query branch lỗi — lỗi bị che thành kết quả. Body: %s", body)
	}
}

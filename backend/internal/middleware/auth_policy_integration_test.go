//go:build integration

// Integration test — GIỮ HÀNH VI chính sách bảo mật của `AuthResponseHeaders` (story-58).
//
// Viết TRƯỚC khi đổi nguồn policy sang `pkg/brand`
// (D22: 2FA/IP/trust-days thành cấu hình toàn hệ thống, giữ thứ tự ưu tiên). Chạy trên thân cũ phải
// xanh, sau khi sửa phải xanh lại. Rủi ro chính của đổi nguồn: policy tắt ÂM THẦM (đọc hụt ⇒ mặc
// định "không bắt 2FA / không giới hạn IP") — nên mọi ca dưới đây đều khẳng định CHIỀU CHẶN, kèm
// đối chứng chiều cho qua để không xanh giả khi middleware chặn mọi thứ.
//
// Seed (story-62): hồ sơ thương hiệu = dòng `business_profile` id=1 (bảng 1 dòng, `brand.Profile` đọc
// id nhỏ nhất). `ch = nil` ⇒ không cache scope.
package middleware

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/logger"
)

// TestMain + testDB của package (story-58 dời về đây khi xoá gate bắt buộc chọn thương hiệu).
var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "mw_policy_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

const policyProfileID = 1

// setBrandPolicy đặt policy trên dòng hồ sơ `business_profile` và khôi phục về "không policy" khi
// test xong. `brand.Invalidate()` — cache in-memory 5' của pkg/brand.
func setBrandPolicy(t *testing.T, require2FA int, roles, ips interface{}) {
	t.Helper()
	testDB.Exec("INSERT IGNORE INTO business_profile (id, name) VALUES (?, 'A')", policyProfileID)
	if err := testDB.Exec("UPDATE business_profile SET require_2fa = ?, require_2fa_roles = ?, allowed_ips = ? WHERE id = ?",
		require2FA, roles, ips, policyProfileID).Error; err != nil {
		t.Fatalf("set policy: %v", err)
	}
	brand.Invalidate()
	t.Cleanup(func() {
		testDB.Exec("UPDATE business_profile SET require_2fa = 0, require_2fa_roles = NULL, allowed_ips = NULL WHERE id = ?",
			policyProfileID)
		brand.Invalidate()
	})
}

type policyUser struct {
	id       int
	branchID string // JSON mảng chi nhánh
	totp     int
	userIPs  interface{} // user.allowed_ips (nil ⇒ NULL)
	roles    []string    // rbac_auth_assignment
}

func seedPolicyUser(t *testing.T, u policyUser) {
	t.Helper()
	testDB.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", u.id)
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", u.id)
	testDB.Exec("DELETE FROM `user` WHERE id = ?", u.id)
	if u.branchID == "" {
		u.branchID = "[]"
	}
	if err := testDB.Exec("INSERT INTO `user` (id, username, password_hash, status, branchId, "+
		"allowed_ips, totp_enabled, created_at, updated_at) VALUES (?, ?, 'x', 10, ?, ?, ?, 0, 0)",
		u.id, fmt.Sprintf("pol%d", u.id), u.branchID, u.userIPs, u.totp).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'P')",
		u.id).Error; err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	for _, r := range u.roles {
		testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 1, ?)", r, r)
		if err := testDB.Exec("INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, 0)",
			r, u.id).Error; err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}
}

// withIPMode đặt IP_ALLOWLIST_MODE cho 1 test (biến package, set lúc boot).
func withIPMode(t *testing.T, mode string) {
	t.Helper()
	SetIPAllowlistMode(mode)
	t.Cleanup(func() { SetIPAllowlistMode("off") })
}

// callFrom giống `call` nhưng chọn IP nguồn (httptest mặc định 192.0.2.1).
func callFrom(t *testing.T, userID int, path, remoteIP string) (int, string, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	reached := false
	r.Use(func(c *gin.Context) { c.Set("userID", uint(userID)); c.Next() })
	r.Use(AuthResponseHeaders(testDB, nil))
	r.GET(path, func(c *gin.Context) { reached = true; c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if remoteIP != "" {
		req.RemoteAddr = remoteIP + ":4321"
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String(), reached
}

const policyPath = "/api/crm/customer"

// AC2(a) — brand require_2fa=1 + user chưa enroll ⇒ 403 `2fa_setup_required`; path miễn trừ đi qua;
// đã enroll ⇒ đi qua.
func TestPolicy_Require2FA_Brand(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 1, nil, nil)

	seedPolicyUser(t, policyUser{id: 861, totp: 0})
	code, body, reached := callFrom(t, 861, policyPath, "")
	if code != http.StatusForbidden || !strings.Contains(body, "2fa_setup_required") || reached {
		t.Fatalf("chưa enroll: HTTP=%d reached=%v body=%s — muốn 403 2fa_setup_required", code, reached, body)
	}
	if code, body, reached := callFrom(t, 861, "/api/auth/2fa/setup", ""); code != http.StatusOK || !reached {
		t.Errorf("path miễn trừ 2FA bị chặn: HTTP=%d body=%s", code, body)
	}

	seedPolicyUser(t, policyUser{id: 862, totp: 1})
	if code, body, reached := callFrom(t, 862, policyPath, ""); code != http.StatusOK || !reached {
		t.Errorf("đã enroll: HTTP=%d body=%s — muốn 200", code, body)
	}
}

// Thứ tự ưu tiên 2FA giữ nguyên: require_2fa=0 nhưng role ∈ require_2fa_roles ⇒ vẫn bắt; role ngoài
// danh sách ⇒ không bắt; không policy nào ⇒ không bắt.
func TestPolicy_Require2FARoles_Brand(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 0, `["s58_role_2fa"]`, nil)

	seedPolicyUser(t, policyUser{id: 863, roles: []string{"s58_role_2fa"}})
	if code, body, _ := callFrom(t, 863, policyPath, ""); code != http.StatusForbidden || !strings.Contains(body, "2fa_setup_required") {
		t.Errorf("role bắt buộc 2FA: HTTP=%d body=%s — muốn 403", code, body)
	}
	seedPolicyUser(t, policyUser{id: 864, roles: []string{"s58_role_khac"}})
	if code, body, _ := callFrom(t, 864, policyPath, ""); code != http.StatusOK {
		t.Errorf("role ngoài danh sách: HTTP=%d body=%s — muốn 200", code, body)
	}
}

func TestPolicy_No2FAPolicy_Passes(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 0, nil, nil)
	seedPolicyUser(t, policyUser{id: 865})
	if code, body, _ := callFrom(t, 865, policyPath, ""); code != http.StatusOK {
		t.Errorf("không policy: HTTP=%d body=%s — muốn 200", code, body)
	}
}

// AC2(b) — allowed_ips của thương hiệu không chứa IP gọi ⇒ bị chặn ở mode enforce; chứa ⇒ đi qua;
// mode log ⇒ chỉ ghi, không chặn; super-user bypass.
func TestPolicy_BrandAllowedIPs_Enforce(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 0, nil, `["10.9.9.0/24"]`)
	withIPMode(t, "enforce")
	seedPolicyUser(t, policyUser{id: 866})

	code, body, reached := callFrom(t, 866, policyPath, "192.0.2.10")
	if code != http.StatusForbidden || reached {
		t.Fatalf("IP ngoài allowlist: HTTP=%d reached=%v body=%s — muốn 403", code, reached, body)
	}
	if code, body, _ := callFrom(t, 866, policyPath, "10.9.9.7"); code != http.StatusOK {
		t.Errorf("IP trong allowlist: HTTP=%d body=%s — muốn 200", code, body)
	}

	seedPolicyUser(t, policyUser{id: 867, roles: []string{"admin"}})
	if code, body, _ := callFrom(t, 867, policyPath, "192.0.2.10"); code != http.StatusOK {
		t.Errorf("super-user phải bypass allowlist: HTTP=%d body=%s", code, body)
	}

	SetIPAllowlistMode("log")
	if code, body, _ := callFrom(t, 866, policyPath, "192.0.2.10"); code != http.StatusOK {
		t.Errorf("mode log không được chặn: HTTP=%d body=%s", code, body)
	}
}

// AC2(c) — IP override của user THẮNG allowlist thương hiệu (cả 2 chiều).
func TestPolicy_UserIPOverrideWins(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 0, nil, `["10.9.9.0/24"]`)
	withIPMode(t, "enforce")

	// User cho phép đúng IP gọi, thương hiệu thì không ⇒ đi qua.
	seedPolicyUser(t, policyUser{id: 868, userIPs: `["192.0.2.10"]`})
	if code, body, _ := callFrom(t, 868, policyPath, "192.0.2.10"); code != http.StatusOK {
		t.Errorf("override user cho phép: HTTP=%d body=%s — muốn 200", code, body)
	}
	// User giới hạn IP khác, thương hiệu cho phép IP gọi ⇒ vẫn bị chặn (override thắng).
	seedPolicyUser(t, policyUser{id: 869, userIPs: `["172.16.0.1"]`})
	if code, body, _ := callFrom(t, 869, policyPath, "10.9.9.7"); code != http.StatusForbidden {
		t.Errorf("override user chặn: HTTP=%d body=%s — muốn 403", code, body)
	}
}

// User không có chi nhánh nào: KHÔNG bị middleware chặn (branch không bắt buộc) và context
// `branchIds` rỗng, `branchActiveId` = 0 — tầng dữ liệu (ScopedDBNullBranch) khi đó chỉ trả bản ghi
// branch NULL (fail-closed ở tầng query, đã có test ở pkg/tenant).
func TestPolicy_UserWithoutBranch(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 0, nil, nil)
	seedPolicyUser(t, policyUser{id: 870, branchID: "[]"})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotIDs []uint
	gotActive := uint(99)
	r.Use(func(c *gin.Context) { c.Set("userID", uint(870)); c.Next() })
	r.Use(AuthResponseHeaders(testDB, nil))
	r.GET(policyPath, func(c *gin.Context) {
		gotIDs = GetBranchIDs(c)
		gotActive = GetBranchActiveID(c)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, policyPath, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
	if len(gotIDs) != 0 || gotActive != 0 {
		t.Errorf("branchIds=%v branchActive=%d — muốn rỗng/0", gotIDs, gotActive)
	}
}

// Fail-closed khi KHÔNG đọc được policy: nguồn policy mất (bảng `business_profile` không truy vấn
// được) ⇒ request bị từ chối (401), KHÔNG được thoái hoá về "không policy" rồi cho qua.
func TestPolicy_PolicySourceError_FailClosed(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 1, nil, nil)
	seedPolicyUser(t, policyUser{id: 871, totp: 0})

	if err := testDB.Exec("RENAME TABLE business_profile TO business_profile_s58_hidden").Error; err != nil {
		t.Fatalf("giấu bảng business_profile: %v", err)
	}
	restored := false
	restore := func() {
		if !restored {
			if err := testDB.Exec("RENAME TABLE business_profile_s58_hidden TO business_profile").Error; err != nil {
				t.Errorf("KHÔI PHỤC bảng business_profile thất bại: %v", err)
			}
			restored = true
			brand.Invalidate()
		}
	}
	defer restore()
	brand.Invalidate()

	code, body, reached := callFrom(t, 871, policyPath, "")
	restore()
	if reached || code == http.StatusOK {
		t.Fatalf("nguồn policy lỗi mà request LỌT tới handler (HTTP=%d) — fail-open. Body: %s", code, body)
	}
	if code != http.StatusUnauthorized {
		t.Errorf("HTTP=%d — muốn 401. Body: %s", code, body)
	}
}

// Policy là toàn hệ thống (dòng `business_profile`) ⇒ 2FA bắt buộc áp cho MỌI user, kể cả user chưa
// có chi nhánh / chưa có phạm vi nào trong hồ sơ.
func TestPolicy_UserWithoutScope_2FAStillEnforced(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	setBrandPolicy(t, 1, nil, nil)
	seedPolicyUser(t, policyUser{id: 873, branchID: "[]", totp: 0})
	code, body, reached := callFrom(t, 873, policyPath, "")
	if code != http.StatusForbidden || !strings.Contains(body, "2fa_setup_required") || reached {
		t.Fatalf("HTTP=%d reached=%v body=%s — muốn 403 2fa_setup_required", code, reached, body)
	}
}

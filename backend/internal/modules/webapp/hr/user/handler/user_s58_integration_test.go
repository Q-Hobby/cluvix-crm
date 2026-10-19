//go:build integration

// Integration test — hr/user sau story-58/62 (E6/D15: schema không còn thương hiệu).
//
// Khoá lại:
//   - key FE-compat trong payload (`companyId`, `user_profile.companyActive_id`) bị BỎ QUA ở
//     Create/Update, không còn 422 (khái niệm đã bỏ — story-62);
//   - [S2-3]: tài khoản super chỉ super sửa (update/xoá/mật khẩu); nhân viên thường non-super sửa được;
//   - Get KHÔNG còn key FE-compat `companyId`/`companyActive_id` (cầu story-62 gỡ ở story-66);
//     get-all-users không lọc thêm.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "hr_user_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	appdb.DB = s.DB
	os.Exit(m.Run())
}

const (
	superCallerID    = 951 // caller mang vai trò admin
	nonSuperCallerID = 952 // caller vai trò thường
)

func newHandler() *Handler {
	return New(Deps{DB: testDB, UserSvc: &hrSvc.UserService{DB: testDB}})
}

// mkUser tạo user trực tiếp + hồ sơ + vai trò; trả id và updated_at.
func mkUser(t *testing.T, username string, role string) (uint, int64) {
	t.Helper()
	testDB.Exec("DELETE a FROM rbac_auth_assignment a JOIN user u ON u.id = a.user_id WHERE u.username = ?", username)
	testDB.Exec("DELETE p FROM user_profile p JOIN user u ON u.id = p.user_id WHERE u.username = ?", username)
	testDB.Exec("DELETE FROM user WHERE username = ?", username)
	if err := testDB.Exec(`INSERT INTO user (username, password_hash, status, branchId, created_at, updated_at)
	             VALUES (?, 'x', 10, '[1]', 0, 0)`, username).Error; err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	var id uint
	testDB.Raw("SELECT id FROM user WHERE username = ?", username).Scan(&id)
	testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'U')", id)
	if role != "" {
		testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 1, ?)", role, role)
		testDB.Exec("INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, 0)", role, id)
	}
	return id, 0
}

// seedCaller tạo caller với id cố định.
func seedCaller(t *testing.T, id int, role string) {
	t.Helper()
	testDB.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", id)
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", id)
	testDB.Exec("DELETE FROM user WHERE id = ?", id)
	testDB.Exec(`INSERT INTO user (id, username, password_hash, status, branchId, created_at, updated_at)
	             VALUES (?, ?, 'x', 10, '[1,2,3,4]', 0, 0)`, id, fmt.Sprintf("caller%d", id))
	testDB.Exec("INSERT INTO user_profile (user_id, fullname) VALUES (?, 'Caller')", id)
	testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 1, ?)", role, role)
	testDB.Exec("INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, 0)", role, id)
}

// serve chạy 1 handler với caller + branchIds (ctx chỉ còn khoá chi nhánh + user).
func serve(t *testing.T, callerID uint, method, route, path, body string, fn func(*Handler, *gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := testhelpers.NewGin()
	r.Handle(method, route, func(c *gin.Context) {
		c.Set("branchIds", []uint{1, 2, 3, 4})
		c.Set("userID", callerID)
		fn(newHandler(), c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func createUser(t *testing.T, body string) *httptest.ResponseRecorder {
	return serve(t, superCallerID, http.MethodPost, "/hr/user", "/hr/user", body,
		func(h *Handler, c *gin.Context) { h.CreateUser(c) })
}

func TestCreateUser_S58_FECompatPayloadIgnored(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, superCallerID, "admin")
	cases := []struct{ name, username, part string }{
		{"payload có key FE-compat companyId + companyActive_id", "s58coA", `"companyId":[2,99],"user_profile":{"fullname":"A","companyActive_id":2},`},
		{"vắng mặt key FE-compat (trước đây 422)", "s58coB", ``},
		{"key FE-compat rỗng (trước đây 422)", "s58coC", `"companyId":[],`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testDB.Exec("DELETE p FROM user_profile p JOIN user u ON u.id = p.user_id WHERE u.username = ?", tc.username)
			testDB.Exec("DELETE FROM user WHERE username = ?", tc.username)
			w := createUser(t, fmt.Sprintf(`{"username":"%s","password":"abc123","status":10,%s"branchId":[1]}`, tc.username, tc.part))
			if w.Code != http.StatusCreated && w.Code != http.StatusOK {
				t.Fatalf("HTTP=%d body=%s", w.Code, w.Body.String())
			}
			var n int64
			testDB.Raw("SELECT COUNT(*) FROM user_profile p JOIN user u ON u.id = p.user_id WHERE u.username = ?", tc.username).Scan(&n)
			if n != 1 {
				t.Errorf("không có dòng user_profile cho %s", tc.username)
			}
			var branch string
			testDB.Raw("SELECT CAST(branchId AS CHAR) FROM user WHERE username = ?", tc.username).Scan(&branch)
			if strings.ReplaceAll(branch, " ", "") != "[1]" {
				t.Errorf("user.branchId = %q — muốn [1] (phần chi nhánh của payload phải được ghi)", branch)
			}
		})
	}
}

func TestUpdateUser_S58_FECompatPayloadIgnored(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, superCallerID, "admin")
	id, _ := mkUser(t, "s58upd", "")
	w := serve(t, superCallerID, http.MethodPut, "/hr/user/:id", fmt.Sprintf("/hr/user/%d", id),
		`{"updated_at":0,"companyId":[],"user_profile":{"nickname":"nn","companyActive_id":2}}`,
		func(h *Handler, c *gin.Context) { h.UpdateUser(c) })
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP=%d body=%s — muốn 200 (key FE-compat rỗng không còn là lỗi)", w.Code, w.Body.String())
	}
	var nick string
	testDB.Raw("SELECT nickname FROM user_profile WHERE user_id = ?", id).Scan(&nick)
	if nick != "nn" {
		t.Errorf("nickname = %q — phần hợp lệ của payload phải được ghi", nick)
	}
}

// [S2-3] — tài khoản super chỉ super-user sửa được.
func TestS23_S58_SuperTargetProtected(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, superCallerID, "admin")
	seedCaller(t, nonSuperCallerID, "s58staff")
	target, _ := mkUser(t, "s58target", "admin")

	ops := []struct {
		name, method, route, path, body string
		fn                              func(*Handler, *gin.Context)
	}{
		{"update", http.MethodPut, "/hr/user/:id", fmt.Sprintf("/hr/user/%d", target), `{"updated_at":0,"user_profile":{"nickname":"x"}}`,
			func(h *Handler, c *gin.Context) { h.UpdateUser(c) }},
		{"reset-password", http.MethodPut, "/hr/user/reset-password", "/hr/user/reset-password", fmt.Sprintf(`{"id":%d,"password":"newpass1"}`, target),
			func(h *Handler, c *gin.Context) { h.ResetPassword(c) }},
		{"change-password", http.MethodPost, "/hr/user/change-password", "/hr/user/change-password", fmt.Sprintf(`{"id":%d,"password":"newpass1"}`, target),
			func(h *Handler, c *gin.Context) { h.ChangePassword(c) }},
		{"delete", http.MethodDelete, "/hr/user/:id", fmt.Sprintf("/hr/user/%d", target), ``,
			func(h *Handler, c *gin.Context) { h.DeleteUser(c) }},
	}
	for _, op := range ops {
		t.Run("non-super "+op.name, func(t *testing.T) {
			w := serve(t, nonSuperCallerID, op.method, op.route, op.path, op.body, op.fn)
			if w.Code != http.StatusForbidden {
				t.Errorf("HTTP=%d body=%s — muốn 403 (tài khoản super)", w.Code, w.Body.String())
			}
		})
	}
	var status int
	testDB.Raw("SELECT status FROM user WHERE id = ?", target).Scan(&status)
	if status != 10 {
		t.Fatalf("tài khoản super bị đổi status = %d bởi non-super", status)
	}
	// Đối chứng: super-user sửa được tài khoản super.
	w := serve(t, superCallerID, ops[0].method, ops[0].route, ops[0].path, ops[0].body, ops[0].fn)
	if w.Code != http.StatusOK {
		t.Errorf("super sửa super: HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
}

// Đối chứng [S2-3]: nhân viên thường (không mang vai trò super) — non-super có quyền nhân sự sửa được.
func TestS23_S58_NonSuperTarget_Editable(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, nonSuperCallerID, "s58staff")
	target, _ := mkUser(t, "s58staffx", "s58staff")
	w := serve(t, nonSuperCallerID, http.MethodPut, "/hr/user/:id", fmt.Sprintf("/hr/user/%d", target),
		`{"updated_at":0,"user_profile":{"nickname":"y"}}`, func(h *Handler, c *gin.Context) { h.UpdateUser(c) })
	if w.Code != http.StatusOK {
		t.Errorf("HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
}

// Get KHÔNG còn key FE-compat (story-66 gỡ cầu story-62); GetAllUsers thấy user.
func TestUserReads_S66_KhongConKeyFECompat(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, superCallerID, "admin")
	id, _ := mkUser(t, "s58other", "s58staff")

	w := serve(t, superCallerID, http.MethodGet, "/hr/user/:id", fmt.Sprintf("/hr/user/%d", id), ``,
		func(h *Handler, c *gin.Context) { h.GetUser(c) })
	if w.Code != http.StatusOK {
		t.Fatalf("GetUser HTTP=%d body=%s", w.Code, w.Body.String())
	}
	var got struct {
		ID          uint                       `json:"id"`
		UserProfile map[string]json.RawMessage `json:"userProfile"`
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v — %s", err, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v — %s", err, w.Body.String())
	}
	if got.ID != id || len(got.UserProfile) == 0 {
		t.Fatalf("GetUser id=%d userProfile=%v — muốn id %d có userProfile", got.ID, got.UserProfile, id)
	}
	if v, has := raw["companyId"]; has {
		t.Errorf("key FE-compat companyId phải KHÔNG còn (story-66), nhận %s", v)
	}
	if v, has := got.UserProfile["companyActive_id"]; has {
		t.Errorf("key FE-compat companyActive_id phải KHÔNG còn (story-66), nhận %s", v)
	}

	w = serve(t, superCallerID, http.MethodGet, "/hr/user/get-all-users", "/hr/user/get-all-users", ``,
		func(h *Handler, c *gin.Context) { h.GetAllUsers(c) })
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), fmt.Sprintf(`"user_id":%d`, id)) {
		t.Errorf("get-all-users thiếu user %d: HTTP=%d body=%s", id, w.Code, w.Body.String())
	}
}

// R-7 (re-review 2026-10-18): user do admin TẠO (admin biết mật khẩu) ⇒ must_change_password = 1.
func TestCreateUser_R7_MustChangePassword(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, superCallerID, "admin")
	const username = "r7mustchg"
	testDB.Exec("DELETE p FROM user_profile p JOIN user u ON u.id = p.user_id WHERE u.username = ?", username)
	testDB.Exec("DELETE FROM user WHERE username = ?", username)
	w := createUser(t, fmt.Sprintf(`{"username":"%s","password":"abc123","status":10,"branchId":[1]}`, username))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("HTTP=%d body=%s", w.Code, w.Body.String())
	}
	var flag int
	testDB.Raw("SELECT must_change_password FROM user WHERE username = ?", username).Scan(&flag)
	if flag != 1 {
		t.Fatalf("must_change_password = %d sau khi admin tạo user — muốn 1", flag)
	}
}

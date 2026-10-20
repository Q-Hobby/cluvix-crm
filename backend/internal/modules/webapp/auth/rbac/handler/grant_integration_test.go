//go:build integration

// Integration test — người có quyền sửa role (updateRole) chỉ được cấp cho role những quyền chính
// mình đang có; role super khác admin (develop) chỉ super xoá được.
package handler

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

const grantCallerID = 961

func seedGrantRole(t *testing.T, role string, perms ...string) {
	t.Helper()
	testDB.Exec("DELETE FROM rbac_auth_item_child WHERE parent = ?", role)
	testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 1, ?)", role, role)
	for _, p := range perms {
		testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 2, ?)", p, p)
		testDB.Exec("INSERT IGNORE INTO rbac_auth_item_child (parent, child) VALUES (?, ?)", role, p)
	}
}

func seedGrantCaller(t *testing.T, role string) {
	t.Helper()
	testDB.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", grantCallerID)
	testDB.Exec("DELETE FROM user WHERE id = ?", grantCallerID)
	testDB.Exec(`INSERT INTO user (id, username, password_hash, status, created_at, updated_at)
	             VALUES (?, 'grantcaller', 'x', 10, 0, 0)`, grantCallerID)
	testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 1, ?)", role, role)
	testDB.Exec("INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, 0)", role, grantCallerID)
}

func callGrant(t *testing.T, method, path, body string, fn func(*Handler, *gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := testhelpers.NewGin()
	r.Handle(method, "/x", func(c *gin.Context) {
		c.Set("userID", uint(grantCallerID))
		fn(New(Deps{DB: testDB, RbacSvc: &authSvc.RbacService{DB: testDB}}), c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/x"+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func roleHas(role, perm string) bool {
	var n int64
	testDB.Raw("SELECT COUNT(*) FROM rbac_auth_item_child WHERE parent = ? AND child = ?", role, perm).Scan(&n)
	return n == 1
}

func TestUpdatePermissionWithRole_ChiCapQuyenMinhCo(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedGrantRole(t, "grantMgr", "updateRole", "grantPermA")
	seedGrantRole(t, "grantStaff", "grantPermForeign") // role đích đã có sẵn quyền caller không có
	testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES ('grantPermB', 2, 'B')")
	seedGrantCaller(t, "grantMgr")
	update := func(role, body string) *httptest.ResponseRecorder {
		return callGrant(t, http.MethodPost, "?roleName="+role, body,
			func(h *Handler, c *gin.Context) { h.UpdatePermissionWithRole(c) })
	}

	// Tự thêm quyền mình không có vào role của chính mình ⇒ 422, không ghi gì.
	if w := update("grantMgr", `{"listPer":[{"oldPer":"","newPer":"grantPermB"}]}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cấp quyền không có: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
	if roleHas("grantMgr", "grantPermB") {
		t.Fatal("quyền bị cấp dù đã chặn")
	}
	// Cấp quyền mình có cho role khác ⇒ 200; gửi lại quyền role ĐÃ có (caller không có) không bị chặn.
	w := update("grantStaff", `{"listPer":[{"oldPer":"","newPer":"grantPermA"},{"oldPer":"","newPer":"grantPermForeign"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("cấp quyền mình có: HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
	if !roleHas("grantStaff", "grantPermA") || !roleHas("grantStaff", "grantPermForeign") {
		t.Fatal("quyền hợp lệ phải được ghi, quyền sẵn có phải giữ")
	}

	// Super cấp được mọi quyền.
	seedGrantCaller(t, "admin")
	if w := update("grantMgr", `{"listPer":[{"oldPer":"","newPer":"grantPermB"}]}`); w.Code != http.StatusOK {
		t.Fatalf("super cấp quyền: HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
}

func TestDeleteRoleName_RoleSuperChiSuperXoa(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedGrantRole(t, "grantDel", "deleteRole")
	testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES ('develop', 1, 'develop')")
	seedGrantCaller(t, "grantDel")
	w := callGrant(t, http.MethodDelete, "?name=develop", "", func(h *Handler, c *gin.Context) { h.DeleteRoleName(c) })
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("non-super xoá role develop: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
	var n int64
	testDB.Raw("SELECT COUNT(*) FROM rbac_auth_item WHERE name = 'develop' AND type = 1").Scan(&n)
	if n != 1 {
		t.Fatal("role develop bị xoá")
	}
}

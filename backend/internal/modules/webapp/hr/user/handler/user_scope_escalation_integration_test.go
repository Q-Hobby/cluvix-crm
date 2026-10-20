//go:build integration

// Integration test — chặn leo thang quyền dưới super + phạm vi chi nhánh của quản lý nhân viên:
//   - non-super chỉ gán được role mà mọi quyền của role đó chính mình cũng có;
//   - caller giới hạn chi nhánh chỉ thấy/sửa nhân viên có MỌI chi nhánh nằm trong phạm vi của mình
//     (nhân viên chưa gán chi nhánh = không giới hạn ⇒ chỉ caller không giới hạn mới quản lý);
//   - caller giới hạn chi nhánh không tạo được nhân viên không có chi nhánh.
package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/internal/testhelpers"
)

// serveScoped — như serve nhưng chọn branchIds của caller và có RbacSvc thật (tra quyền từ DB).
func serveScoped(t *testing.T, callerID uint, branchIDs []uint, method, route, path, body string, fn func(*Handler, *gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := testhelpers.NewGin()
	r.Handle(method, route, func(c *gin.Context) {
		c.Set("branchIds", branchIDs)
		c.Set("userID", callerID)
		fn(New(Deps{DB: testDB, UserSvc: &hrSvc.UserService{DB: testDB}, RbacSvc: &authSvc.RbacService{DB: testDB}}), c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// seedRoleWithPerms tạo role + quyền (type=2) + gán quyền cho role.
func seedRoleWithPerms(t *testing.T, role string, perms ...string) {
	t.Helper()
	testDB.Exec("DELETE FROM rbac_auth_item_child WHERE parent = ?", role)
	testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 1, ?)", role, role)
	for _, p := range perms {
		testDB.Exec("INSERT IGNORE INTO rbac_auth_item (name, type, description) VALUES (?, 2, ?)", p, p)
		testDB.Exec("INSERT IGNORE INTO rbac_auth_item_child (parent, child) VALUES (?, ?)", role, p)
	}
}

func setUserBranches(t *testing.T, id uint, branches string) {
	t.Helper()
	if err := testDB.Exec("UPDATE user SET branchId = ? WHERE id = ?", branches, id).Error; err != nil {
		t.Fatal(err)
	}
}

func putUser(t *testing.T, callerID uint, branchIDs []uint, id uint, fields string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"updated_at":%d%s}`, userUpdatedAt(t, id), fields)
	return serveScoped(t, callerID, branchIDs, http.MethodPut, "/hr/user/:id", fmt.Sprintf("/hr/user/%d", id), body,
		func(h *Handler, c *gin.Context) { h.UpdateUser(c) })
}

// Non-super có quyền sửa nhân viên chỉ gán được role "nằm trong" quyền của chính mình — trước đây
// gán được mọi role trừ admin/develop ⇒ tự gán cho mình (hoặc đồng bọn) role có toàn quyền khác.
func TestUpdateUser_RoleAssign_ChiTrongQuyenCuaMinh(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedRoleWithPerms(t, "escMgr", "escPermView", "escPermEdit")
	seedRoleWithPerms(t, "escSmall", "escPermView")
	seedRoleWithPerms(t, "escBig", "escPermView", "escPermDelete")
	seedCaller(t, nonSuperCallerID, "escMgr")
	target, _ := mkUser(t, "escTarget", "escSmall")
	all := []uint{1, 2, 3, 4}

	if w := putUser(t, nonSuperCallerID, all, target, `,"role":"escBig"`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("gán role có quyền caller không có: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
	if w := putUser(t, nonSuperCallerID, all, uint(nonSuperCallerID), `,"role":"escBig"`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("tự gán role lớn hơn: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
	var role string
	testDB.Raw("SELECT item_name FROM rbac_auth_assignment WHERE user_id = ?", target).Scan(&role)
	if role != "escSmall" {
		t.Fatalf("role bị đổi thành %q dù bị chặn", role)
	}
	if w := putUser(t, nonSuperCallerID, all, target, `,"role":"escMgr"`); w.Code != http.StatusOK {
		t.Fatalf("gán role bằng quyền mình: HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}

	// Super gán được mọi role.
	seedCaller(t, superCallerID, "admin")
	if w := putUser(t, superCallerID, all, target, `,"role":"escBig"`); w.Code != http.StatusOK {
		t.Fatalf("super gán role: HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
}

// Caller chỉ có chi nhánh 1 không sửa được (404 như không tồn tại) nhân viên chi nhánh 2, nhân viên
// 1+2, hay nhân viên chưa gán chi nhánh; sửa được nhân viên chi nhánh 1. Super thấy tất cả.
func TestUpdateUser_PhamViChiNhanh(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, nonSuperCallerID, "s58staff")
	seedCaller(t, superCallerID, "admin")
	cases := []struct {
		username, branches string
		want               int
	}{
		{"brOwn", "[1]", http.StatusOK},
		{"brOther", "[2]", http.StatusNotFound},
		{"brBoth", "[1,2]", http.StatusNotFound},
		{"brNone", "[]", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.username, func(t *testing.T) {
			id, _ := mkUser(t, tc.username, "s58staff")
			setUserBranches(t, id, tc.branches)
			if w := putUser(t, nonSuperCallerID, []uint{1}, id, `,"user_profile":{"nickname":"x"}`); w.Code != tc.want {
				t.Fatalf("caller chi nhánh 1 sửa nhân viên %s: HTTP=%d body=%s — muốn %d", tc.branches, w.Code, w.Body.String(), tc.want)
			}
			if w := putUser(t, superCallerID, []uint{1}, id, `,"user_profile":{"nickname":"y"}`); w.Code != http.StatusOK {
				t.Fatalf("super sửa nhân viên %s: HTTP=%d body=%s — muốn 200", tc.branches, w.Code, w.Body.String())
			}
		})
	}

	// Không được gỡ hết chi nhánh của nhân viên (= thành không giới hạn chi nhánh).
	id, _ := mkUser(t, "brClear", "s58staff")
	if w := putUser(t, nonSuperCallerID, []uint{1}, id, `,"branchId":[]`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("gỡ hết chi nhánh: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
}

// Caller giới hạn chi nhánh phải gán chi nhánh khi tạo nhân viên; caller không giới hạn thì không bắt.
func TestCreateUser_PhamViChiNhanh(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedCaller(t, nonSuperCallerID, "s58staff")
	create := func(username, branchPart string, branchIDs []uint) *httptest.ResponseRecorder {
		testDB.Exec("DELETE p FROM user_profile p JOIN user u ON u.id = p.user_id WHERE u.username = ?", username)
		testDB.Exec("DELETE FROM user WHERE username = ?", username)
		body := fmt.Sprintf(`{"username":"%s","password":"abc12345","status":10%s}`, username, branchPart)
		return serveScoped(t, nonSuperCallerID, branchIDs, http.MethodPost, "/hr/user", "/hr/user", body,
			func(h *Handler, c *gin.Context) { h.CreateUser(c) })
	}
	if w := create("brNew1", ``, []uint{1}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("thiếu branchId: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
	if w := create("brNew2", `,"branchId":[]`, []uint{1}); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("branchId rỗng: HTTP=%d body=%s — muốn 422", w.Code, w.Body.String())
	}
	if w := create("brNew3", `,"branchId":[1]`, []uint{1}); w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("branchId trong phạm vi: HTTP=%d body=%s — muốn 201", w.Code, w.Body.String())
	}
	if w := create("brNew4", ``, nil); w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("caller không giới hạn chi nhánh: HTTP=%d body=%s — muốn 201", w.Code, w.Body.String())
	}
}

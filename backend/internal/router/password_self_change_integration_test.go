//go:build integration

// Integration test — tự đổi mật khẩu không cần quyền updateUser (luồng ép đổi mật khẩu S-4/D-1).
//
// Gate `must_change_password` (AuthResponseHeaders) chỉ cho đi qua POST /api/hr/user/update-password,
// nên route đó PHẢI mở cho nhân viên thường (kind public trong route_perm_map) — nếu còn đòi
// updateUser thì nhân viên bị ép đổi mật khẩu bị khoá chết (gate cho qua, enforcer chặn 403).
// Handler UpdatePassword chỉ dùng uid của phiên + bắt kiểm mật khẩu cũ ⇒ không đổi được của ai khác.
// Đặt mật khẩu cho NGƯỜI KHÁC (change-password) vẫn đòi updateUser.
//
// Engine dựng đúng thứ tự nhóm authed của Setup: (uid từ header thay JWTAuth) → AuthResponseHeaders
// (gate must_change_password thật) → PermissionEnforcer(map embed thật) → route hr thật.
package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"erpCluvix/internal/deps"
	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/auth/rbac/seed"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	"erpCluvix/internal/modules/webapp/hr"
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

const pwUpdatePath = "/api/hr/user/update-password"

func pwEngine(t *testing.T, entries []seed.RoutePermEntry) *gin.Engine {
	t.Helper()
	rbac := &authSvc.RbacService{DB: testDB}
	r := testhelpers.NewGin()
	api := r.Group("/api")
	authed := api.Group("", func(c *gin.Context) {
		uid, _ := strconv.ParseUint(c.GetHeader(hdrUID), 10, 64)
		c.Set("userID", uint(uid))
		c.Next()
	}, middleware.AuthResponseHeaders(testDB, nil), middleware.PermissionEnforcer(rbac, entries))
	hr.Register(authed, deps.Deps{DB: testDB, RbacSvc: rbac, UserSvc: &hrSvc.UserService{DB: testDB}})
	// API nghiệp vụ đại diện — chỉ để đối chứng gate must_change_password có nằm trong chuỗi.
	authed.GET("/crm/customer", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func callJSON(r *gin.Engine, method, url string, uid uint, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(hdrUID, strconv.FormatUint(uint64(uid), 10))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setPassword(t *testing.T, uid uint, pw string, mustChange int) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, "UPDATE `user` SET password_hash = ?, must_change_password = ? WHERE id = ?", string(hash), mustChange, uid)
}

func pwState(t *testing.T, uid uint) (hash string, mustChange int) {
	t.Helper()
	var row struct {
		Hash string `gorm:"column:password_hash"`
		Must int    `gorm:"column:must_change_password"`
	}
	if err := testDB.Raw("SELECT password_hash, must_change_password FROM `user` WHERE id = ?", uid).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.Hash, row.Must
}

func TestPassword_SelfChangeWithoutUpdateUser(t *testing.T) {
	skipNoDB(t)
	withEnforce(t, true)
	// Policy thương hiệu (2FA/IP) không bật — AuthResponseHeaders đọc hồ sơ fail-closed.
	testDB.Exec("INSERT IGNORE INTO business_profile (id, name) VALUES (1, 'A')")

	entries, err := seed.LoadRoutePermMap()
	if err != nil {
		t.Fatalf("LoadRoutePermMap: %v", err)
	}
	r := pwEngine(t, entries)

	tn := testhelpers.FreshTenant(t, testDB)
	staff := testhelpers.CreateScopedUser(t, testDB, "pw_staff", tn, tn.BranchID) // KHÔNG có quyền nào
	other := testhelpers.CreateScopedUser(t, testDB, "pw_other", tn, tn.BranchID)
	const oldPw, newPw = "Old-Pass-1", "New-Pass-2"
	setPassword(t, staff, oldPw, define.UserMustChangePasswordYes)
	setPassword(t, other, "Other-Pass-1", define.UserMustChangePasswordNo)
	otherHash, _ := pwState(t, other)

	// Đối chứng: gate ép đổi mật khẩu đang nằm trong chuỗi ⇒ API nghiệp vụ bị chặn với mã riêng.
	w := callJSON(r, http.MethodGet, "/api/crm/customer", staff, nil)
	expect(t, w, http.StatusForbidden, "API nghiệp vụ khi còn cờ must_change_password")
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["code"] != define.ErrCodePasswordChangeRequired {
		t.Fatalf("API nghiệp vụ khi còn cờ: code=%v — muốn %s (gate không nằm trong chuỗi?)", out["code"], define.ErrCodePasswordChangeRequired)
	}

	// Đối chứng: map CŨ (update-password đòi updateUser) ⇒ chính request tự đổi bị 403 — chứng minh
	// ca 200 bên dưới phụ thuộc đúng entry vừa mở, không phải enforce đang tắt.
	oldEntries := make([]seed.RoutePermEntry, len(entries))
	copy(oldEntries, entries)
	found := false
	for i := range oldEntries {
		if oldEntries[i].Method == http.MethodPost && oldEntries[i].Path == pwUpdatePath {
			oldEntries[i].Perm, oldEntries[i].Kind, oldEntries[i].EnforcePhase1 = define.PermUpdateUser, seed.KindPerm, true
			found = true
		}
	}
	if !found {
		t.Fatalf("route_perm_map thiếu entry POST %s", pwUpdatePath)
	}
	selfBody := map[string]any{"oldpassword": oldPw, "newpassword": newPw, "newpasswordrepeat": newPw}
	expect(t, callJSON(pwEngine(t, oldEntries), http.MethodPost, pwUpdatePath, staff, selfBody),
		http.StatusForbidden, "map cũ (updateUser) — đối chứng")
	if _, must := pwState(t, staff); must != define.UserMustChangePasswordYes {
		t.Fatalf("request bị 403 nhưng cờ = %d — muốn giữ 1", must)
	}

	// 1. Nhân viên thường tự đổi mật khẩu của mình ⇒ 200, cờ về 0, mật khẩu mới có hiệu lực.
	expect(t, callJSON(r, http.MethodPost, pwUpdatePath, staff, selfBody), http.StatusOK, "tự đổi mật khẩu không có updateUser")
	hash, must := pwState(t, staff)
	if must != define.UserMustChangePasswordNo {
		t.Fatalf("sau khi tự đổi: must_change_password = %d — muốn 0", must)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(newPw)) != nil {
		t.Fatal("sau khi tự đổi: hash không khớp mật khẩu mới")
	}

	// 2. Đặt mật khẩu cho người khác (change-password) không có updateUser ⇒ 403, DB người kia không đổi.
	otherBody := map[string]any{"id": other, "password": "Hacked-123"}
	expect(t, callJSON(r, http.MethodPost, "/api/hr/user/change-password", staff, otherBody),
		http.StatusForbidden, "đổi mật khẩu người khác không có updateUser")
	if h, m := pwState(t, other); h != otherHash || m != define.UserMustChangePasswordNo {
		t.Fatalf("bị 403 nhưng mật khẩu/cờ người khác đã đổi (must=%d)", m)
	}

	// Đối chứng: có updateUser thì cùng request qua ⇒ 403 ở trên đúng là do thiếu quyền.
	manager := testhelpers.CreateScopedUser(t, testDB, "pw_manager", tn, tn.BranchID)
	grant(t, manager, define.PermUpdateUser)
	expect(t, callJSON(r, http.MethodPost, "/api/hr/user/change-password", manager, otherBody),
		http.StatusOK, "đổi mật khẩu người khác CÓ updateUser (đối chứng)")
	if h, m := pwState(t, other); h == otherHash || m != define.UserMustChangePasswordYes {
		t.Fatalf("có updateUser: mật khẩu người khác phải đổi + cờ = 1, được must=%d", m)
	}
}

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockRbac thỏa mãn PermissionChecker cho unit test — không chạm DB/Redis.
type mockRbac struct {
	super bool
	perms []string
	role  string
}

func (m *mockRbac) IsSuperUser(uint) bool                     { return m.super }
func (m *mockRbac) GetUserPermissions(uint) ([]string, error) { return m.perms, nil }
func (m *mockRbac) GetUserRole(uint) (string, error)          { return m.role, nil }

// runReq chạy 1 request POST /x qua middleware mw, với uid set sẵn trong context.
func runReq(uid uint, mw gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uid) })
	r.POST("/x", mw, func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.ServeHTTP(w, req)
	return w
}

// withAudit tạm set permEnforce + permAuditSink, trả về capture + restore func.
func withAudit(t *testing.T, enforce bool) (*capturedEvent, func()) {
	t.Helper()
	prevEnforce := permEnforce
	prevSink := permAuditSink
	permEnforce = enforce
	cap := &capturedEvent{}
	permAuditSink = func(c *gin.Context, event, role string, required []string) {
		cap.called = true
		cap.event = event
		cap.role = role
		cap.required = required
	}
	return cap, func() {
		permEnforce = prevEnforce
		permAuditSink = prevSink
	}
}

type capturedEvent struct {
	called   bool
	event    string
	role     string
	required []string
}

// Case 1: super-role → bypass, không log, 200.
func TestRequirePermission_SuperBypass(t *testing.T) {
	cap, restore := withAudit(t, true) // enforce=true để chứng minh super vẫn qua
	defer restore()

	rbac := &mockRbac{super: true}
	w := runReq(7, RequirePermission(rbac, "customer.create"))

	if w.Code != http.StatusOK {
		t.Fatalf("super phải pass, got %d", w.Code)
	}
	if cap.called {
		t.Fatalf("super không được sinh audit event")
	}
}

// Case 2: có ≥1 perm yêu cầu → pass, 200.
func TestRequirePermission_HasPermission(t *testing.T) {
	cap, restore := withAudit(t, true)
	defer restore()

	rbac := &mockRbac{perms: []string{"customer.view", "customer.create"}}
	w := runReq(7, RequirePermission(rbac, "customer.create"))

	if w.Code != http.StatusOK {
		t.Fatalf("user có quyền phải pass, got %d", w.Code)
	}
	if cap.called {
		t.Fatalf("user có quyền không được sinh audit event")
	}
}

// Case 3: thiếu perm + audit-mode → pass (200) NHƯNG log would-deny.
func TestRequirePermission_MissingAuditMode(t *testing.T) {
	cap, restore := withAudit(t, false) // audit-mode
	defer restore()

	rbac := &mockRbac{perms: []string{"customer.view"}, role: "sale"}
	w := runReq(7, RequirePermission(rbac, "customer.create"))

	if w.Code != http.StatusOK {
		t.Fatalf("audit-mode KHÔNG được chặn, got %d", w.Code)
	}
	if !cap.called || cap.event != evPermWouldDeny {
		t.Fatalf("audit-mode phải log would-deny, got called=%v event=%q", cap.called, cap.event)
	}
	if cap.role != "sale" || len(cap.required) != 1 || cap.required[0] != "customer.create" {
		t.Fatalf("audit event thiếu field: role=%q required=%v", cap.role, cap.required)
	}
}

// Case 4: thiếu perm + enforce-mode → 403.
func TestRequirePermission_MissingEnforceMode(t *testing.T) {
	cap, restore := withAudit(t, true) // enforce-mode
	defer restore()

	rbac := &mockRbac{perms: []string{"customer.view"}, role: "sale"}
	w := runReq(7, RequirePermission(rbac, "customer.create"))

	if w.Code != http.StatusForbidden {
		t.Fatalf("enforce-mode thiếu quyền phải 403, got %d", w.Code)
	}
	if !cap.called || cap.event != evPermDenied {
		t.Fatalf("enforce-mode phải log deny, got called=%v event=%q", cap.called, cap.event)
	}
}

// RequireRole: super + role khớp → pass; role sai + enforce → 403.
func TestRequireRole(t *testing.T) {
	// role khớp
	_, restore := withAudit(t, true)
	rbac := &mockRbac{role: "doctor"}
	if w := runReq(7, RequireRole(rbac, "doctor")); w.Code != http.StatusOK {
		restore()
		t.Fatalf("role khớp phải pass, got %d", w.Code)
	}
	restore()

	// super bypass dù không nằm trong roles
	_, restore = withAudit(t, true)
	rbacSuper := &mockRbac{role: "admin"}
	if w := runReq(7, RequireRole(rbacSuper, "doctor")); w.Code != http.StatusOK {
		restore()
		t.Fatalf("super phải pass RequireRole, got %d", w.Code)
	}
	restore()

	// role sai + enforce → 403
	_, restore = withAudit(t, true)
	defer restore()
	rbacWrong := &mockRbac{role: "sale"}
	if w := runReq(7, RequireRole(rbacWrong, "doctor")); w.Code != http.StatusForbidden {
		t.Fatalf("role sai enforce phải 403, got %d", w.Code)
	}
}

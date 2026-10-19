package middleware

import (
	"strings"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/seclog"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PermissionChecker là subset của *rbac/service.RbacService mà middleware cần.
// Khai báo interface tại đây (thay vì import authSvc trực tiếp) vì RbacService
// đã import package middleware → import ngược sẽ tạo import cycle.
// *RbacService thỏa mãn interface này structurally.
type PermissionChecker interface {
	IsSuperUser(userID uint) bool
	GetUserPermissions(userID uint) ([]string, error)
	GetUserRole(userID uint) (string, error)
}

// permEnforce: false = audit-mode (log would-deny, KHÔNG chặn), true = trả 403.
// Set 1 lần lúc boot qua SetPermEnforce (từ PERM_ENFORCE env) — KHÔNG đọc env mỗi request.
var permEnforce = false

// SetPermEnforce cấu hình chế độ enforcement permission (từ cfg.PermEnforce).
func SetPermEnforce(enforce bool) { permEnforce = enforce }

// PermEnforcing trả trạng thái enforce hiện tại (audit-mode = false).
func PermEnforcing() bool { return permEnforce }

const (
	// evPermWouldDeny — audit-mode: request lẽ ra bị chặn nhưng vẫn cho qua (thu thập traffic thật).
	evPermWouldDeny = "perm_would_deny"
	// evPermDenied — enforce-mode: request bị chặn 403.
	evPermDenied = "perm_denied"
)

// permAuditSink đẩy 1 sự kiện would-deny/deny vào seclog DB sink (async, non-blocking,
// KHÔNG thêm query đồng bộ trên hot path). Là biến để test override assert — production giữ nguyên.
var permAuditSink = emitPermEvent

func emitPermEvent(c *gin.Context, event, role string, required []string) {
	decision := "would-deny"
	if event == evPermDenied {
		decision = "deny"
	}
	// user_id được seclog promote sang cột; role/required/path/method/decision vào Detail JSON.
	seclog.Event(c, event,
		zap.Uint("user_id", GetUserID(c)),
		zap.String("role", role),
		zap.String("perms_required", strings.Join(required, ",")),
		zap.String("path", routePath(c)),
		zap.String("method", c.Request.Method),
		zap.String("decision", decision),
	)
}

// RequirePermission trả middleware chặn nếu user thiếu MỌI quyền trong perms (OR-logic:
// có ≥1 perm là pass). Đặt SAU JWTAuth (uid luôn có). Super-role bypass.
// Audit-mode (PERM_ENFORCE=false, default): chỉ log would-deny → c.Next(), KHÔNG 403.
func RequirePermission(rbac PermissionChecker, perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rbac == nil {
			c.Next()
			return
		}
		uid := GetUserID(c)
		if uid == 0 {
			// Middleware đặt sai vị trí (trước JWTAuth) → fail-safe theo mode.
			decide(c, "", perms)
			return
		}
		if rbac.IsSuperUser(uid) {
			c.Next()
			return
		}
		granted, _ := rbac.GetUserPermissions(uid)
		if hasAnyPerm(granted, perms) {
			c.Next()
			return
		}
		role, _ := rbac.GetUserRole(uid)
		decide(c, role, perms)
	}
}

// RequireRole trả middleware chặn nếu user KHÔNG có role nào trong roles (OR-logic).
// Biến thể cho endpoint kind=role (doctor/super...). Super-role luôn pass.
// Cùng audit-mode logic với RequirePermission.
func RequireRole(rbac PermissionChecker, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rbac == nil {
			c.Next()
			return
		}
		uid := GetUserID(c)
		if uid == 0 {
			decide(c, "", roles)
			return
		}
		role, _ := rbac.GetUserRole(uid)
		if define.IsSuperRole(role) || helpers.StringInSlice(roles, role) {
			c.Next()
			return
		}
		decide(c, role, roles)
	}
}

// decide áp dụng mode: enforce → log deny + 403 + Abort; audit → log would-deny + Next.
func decide(c *gin.Context, role string, required []string) {
	if permEnforce {
		permAuditSink(c, evPermDenied, role, required)
		response.Forbidden(c)
		c.Abort()
		return
	}
	permAuditSink(c, evPermWouldDeny, role, required)
	c.Next()
}

// hasAnyPerm true nếu granted chứa ≥1 phần tử của required (OR-logic).
func hasAnyPerm(granted, required []string) bool {
	if len(required) == 0 {
		return true // không yêu cầu quyền cụ thể
	}
	set := make(map[string]struct{}, len(granted))
	for _, g := range granted {
		set[g] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}

// routePath ưu tiên route pattern (c.FullPath) để log gộp được theo endpoint, fallback URL thật.
func routePath(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return c.Request.URL.Path
}

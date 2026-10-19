package middleware

import (
	"strings"

	"erpCluvix/internal/modules/webapp/auth/rbac/seed"

	"github.com/gin-gonic/gin"
)

// rolePrefix — trong route_perm_map.json, entry kind=role có perm dạng "role:doctor".
const rolePrefix = "role:"

// PermissionEnforcer là 1 middleware TOÀN CỤC (declarative): đọc route→perm map lúc
// khởi tạo, build sẵn 1 handler enforcement cho mỗi endpoint cần chặn, rồi mỗi request
// tra O(1) theo `method + " " + c.FullPath()`. KHÔNG parse JSON / build map mỗi request.
//
// Chỉ endpoint có kind=perm|role VÀ enforce_phase1=true mới được gắn handler
// (giai đoạn đầu chỉ enforce mutating; GET / public / unmapped bỏ qua → c.Next()).
// Handler tái dùng RequirePermission/RequireRole (P1.1) — vẫn audit-mode mặc định
// (PERM_ENFORCE=false → chỉ log would-deny, KHÔNG chặn).
//
// entries: kết quả seed.LoadRoutePermMap(). rbac: PermissionChecker (d.RbacSvc).
func PermissionEnforcer(rbac PermissionChecker, entries []seed.RoutePermEntry) gin.HandlerFunc {
	handlers := buildEnforceHandlers(rbac, entries)
	return func(c *gin.Context) {
		if h, ok := handlers[c.Request.Method+" "+c.FullPath()]; ok {
			h(c) // gọi c.Next()/decide bên trong — hành vi hệt như gắn tay per-route.
			return
		}
		c.Next()
	}
}

// buildEnforceHandlers dựng map key(`method path`) → handler enforcement cho những entry
// thật sự enforce (kind perm/role + enforce_phase1). Mọi key khác không có trong map
// → request rơi vào nhánh c.Next() (không chặn). Tách hàm để test coverage tái dùng.
func buildEnforceHandlers(rbac PermissionChecker, entries []seed.RoutePermEntry) map[string]gin.HandlerFunc {
	handlers := make(map[string]gin.HandlerFunc, len(entries))
	for _, e := range entries {
		if !e.EnforcePhase1 {
			continue
		}
		key := e.Method + " " + e.Path
		switch e.Kind {
		case seed.KindPerm:
			if e.Perm == "" {
				continue
			}
			// perm có thể là CSV nhiều quyền (OR-gate: 1 endpoint phục vụ ≥2 màn).
			// Tách theo "," → RequirePermission(perms...) đã có OR-logic. Perm đơn → 1 phần tử.
			perms := splitPerms(e.Perm)
			if len(perms) == 0 {
				continue
			}
			handlers[key] = RequirePermission(rbac, perms...)
		case seed.KindRole:
			role := strings.TrimPrefix(e.Perm, rolePrefix)
			if role == "" {
				continue
			}
			handlers[key] = RequireRole(rbac, role)
		default:
			// public / unmapped → không enforce (chờ Epic 3 định nghĩa quyền).
		}
	}
	return handlers
}

// splitPerms tách field perm dạng CSV ("a,b") thành slice quyền, trim khoảng trắng và
// bỏ phần tử rỗng. Perm đơn → slice 1 phần tử; an toàn cho mọi entry hiện có.
func splitPerms(perm string) []string {
	parts := strings.Split(perm, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

package middleware

import (
	"reflect"

	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

// HasAnyPermStrict — user đang đăng nhập có ≥1 quyền trong perms (super-role luôn có).
// KHÁC RequirePermission: KHÔNG phụ thuộc PERM_ENFORCE (không có audit-mode) — dùng cho guard
// tường minh ở điểm đọc trọn dữ liệu (export Excel), nơi thiếu quyền là phải chặn thật.
// rbac nil (kể cả *RbacService nil bọc trong interface) hoặc uid = 0 ⇒ false (fail-closed).
func HasAnyPermStrict(c *gin.Context, rbac PermissionChecker, perms ...string) bool {
	if isNilChecker(rbac) {
		return false
	}
	uid := GetUserID(c)
	if uid == 0 {
		return false
	}
	if rbac.IsSuperUser(uid) {
		return true
	}
	granted, err := rbac.GetUserPermissions(uid)
	if err != nil {
		return false
	}
	return hasAnyPerm(granted, perms)
}

// RequireExportPerm — middleware gắn ở route/group có nhánh xuất Excel `?_export=1`: request export
// phải có ≥1 quyền perms (ext* của đúng loại dữ liệu), nếu không ⇒ 403 + Abort + log perm_denied.
// Request KHÔNG export đi thẳng c.Next() (quyền xem do PermissionEnforcer/route_perm_map lo).
// Chặn thật bất kể PERM_ENFORCE (S-7 lens bảo mật 2026-10-18) — cùng ngữ nghĩa guard
// define.PermExtPayment trong crm/payment/handler/list.go.
func RequireExportPerm(rbac PermissionChecker, perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query("_export") != "1" {
			c.Next()
			return
		}
		requirePermStrict(c, rbac, perms)
	}
}

// RequirePermStrict — như RequireExportPerm nhưng cho endpoint CHỈ dùng để xuất file (vd
// `/by-date/export`, `/print/appointment-byhour-list-export`): luôn kiểm, không cần `_export=1`.
func RequirePermStrict(rbac PermissionChecker, perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requirePermStrict(c, rbac, perms)
	}
}

func requirePermStrict(c *gin.Context, rbac PermissionChecker, perms []string) {
	if HasAnyPermStrict(c, rbac, perms...) {
		c.Next()
		return
	}
	role := ""
	if !isNilChecker(rbac) && GetUserID(c) != 0 {
		role, _ = rbac.GetUserRole(GetUserID(c))
	}
	permAuditSink(c, evPermDenied, role, perms)
	response.Forbidden(c)
	c.Abort()
}

// isNilChecker — nil interface HOẶC con trỏ nil bọc trong interface (vd deps chưa wire RbacSvc:
// `(*RbacService)(nil)` truyền vào PermissionChecker ⇒ interface != nil nhưng gọi method sẽ panic).
func isNilChecker(rbac PermissionChecker) bool {
	if rbac == nil {
		return true
	}
	v := reflect.ValueOf(rbac)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

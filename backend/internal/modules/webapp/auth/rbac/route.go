package rbac

import (
	"erpCluvix/internal/modules/webapp/auth/rbac/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(authed gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	authed.GET("/auth/rbac/child-role-for-dropdown", h.ChildRoleForDropdown)
	authed.GET("/auth/rbac/list-permission", h.ListPermissionByRole)
	authed.POST("/auth/rbac/update-permission-with-role", h.UpdatePermissionWithRole)

	// RBAC > Role
	authed.GET("/auth/rbac/role", h.ListRole)
	authed.GET("/auth/rbac/role/get-name", h.GetRoleName)
	authed.POST("/auth/rbac/role/create-name", h.CreateRoleName)
	authed.PUT("/auth/rbac/role/update-name", h.UpdateRoleName)
	authed.POST("/auth/rbac/role/update-permission-with-role", h.UpdatePermissionWithRole)
	authed.DELETE("/auth/rbac/role/delete-name", h.DeleteRoleName)

	// RBAC > Permission
	authed.GET("/auth/rbac/permission", h.ListPermission)
	authed.GET("/auth/rbac/permissions", h.ListPermission)
	authed.GET("/auth/rbac/roles", h.ListRole)
	authed.GET("/auth/rbac/permission/get-name", h.GetPermissionName)
	authed.POST("/auth/rbac/permission/create-name", h.CreatePermissionName)
	authed.PUT("/auth/rbac/permission/update-name", h.UpdatePermissionName)
	authed.DELETE("/auth/rbac/permission/delete-name", h.DeletePermissionName)
}

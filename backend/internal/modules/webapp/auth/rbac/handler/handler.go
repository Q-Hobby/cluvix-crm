package handler

import (
	"errors"
	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"

	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/token"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	DB       *gorm.DB
	Cache    *cache.RedisCache
	TokenSvc *token.Service
	RbacSvc  *authSvc.RbacService
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// requirePerm trả false + 403 nếu user không có permission `perm`.
// Super-user pass mặc định (GetUserPermissions trả toàn bộ perm cho super).
// Ai được gán perm tương ứng (viewRole/createRole/updateRole/deleteRole) đều dùng được.
func (h *Handler) requirePerm(c *gin.Context, perm string) bool {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		response.Forbidden(c)
		return false
	}
	// Super-user luôn pass — không phụ thuộc perm row đã sync vào rbac_auth_item hay chưa.
	if h.RbacSvc.IsSuperUser(userID) {
		return true
	}
	perms, err := h.RbacSvc.GetUserPermissions(userID)
	if err != nil {
		response.Forbidden(c)
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	response.Forbidden(c)
	return false
}

// requireSuper — RBAC là global (không company_id, xem [C1]). Dùng cho permission
// catalog (tạo/sửa/xoá permission type=2) — mutation này ảnh hưởng toàn hệ thống
// (mọi role), không gắn với 1 role cụ thể nên không có permission per-role nào
// hợp lý để gate, luôn yêu cầu super-user.
// Role management (create/update/delete role, gán quyền cho role) dùng requirePerm
// (createRole/updateRole/deleteRole) thay vì hàm này — xem UpdatePermissionWithRole,
// CreateRoleName, UpdateRoleName, DeleteRoleName. DeleteRoleName tự chặn riêng role
// "admin" (define.RoleAdmin) để user được phân công deleteRole không xoá được role gốc.
func (h *Handler) requireSuper(c *gin.Context) bool {
	userID := middleware.GetUserID(c)
	if userID == 0 || !helpers.IsSuperUser(h.DB, userID) {
		response.Forbidden(c)
		return false
	}
	return true
}

// POST /auth/rbac-auth-item-child/update-permission-with-role?roleName=xxx
func (h *Handler) UpdatePermissionWithRole(c *gin.Context) {
	if !h.requirePerm(c, "updateRole") {
		return
	}
	roleName := c.Query("roleName")
	if roleName == "" {
		response.Validate(c, i18n.T(c, "role_name_required"))
		return
	}

	var body struct {
		ListPer []struct {
			OldPer string `json:"oldPer"`
			NewPer string `json:"newPer"`
		} `json:"listPer"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.ListPer) == 0 {
		response.Validate(c, i18n.T(c, "permission_invalid"))
		return
	}

	var toDelete, toInsert []string
	for _, item := range body.ListPer {
		if item.OldPer != "" {
			toDelete = append(toDelete, item.OldPer)
		}
		if item.NewPer != "" {
			toInsert = append(toInsert, item.NewPer)
		}
	}
	// Chỉ được cấp quyền mình đang có (super bỏ qua) — chặn tự thêm quyền vào role của mình. Quyền
	// role ĐÃ có (FE gửi lại) không tính là cấp mới.
	current := make(map[string]bool)
	for _, p := range h.RbacSvc.GetPermissionsByRole(roleName) {
		current[p] = true
	}
	var granted []string
	for _, p := range toInsert {
		if !current[p] {
			granted = append(granted, p)
		}
	}
	notHeld, err := h.RbacSvc.PermissionsNotHeld(middleware.GetUserID(c), granted)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	if len(notHeld) > 0 {
		response.Validate(c, i18n.TF(c, "permission_grant_not_held", strings.Join(notHeld, ", ")))
		return
	}

	if err := h.RbacSvc.UpdatePermissionWithRole(roleName, toDelete, toInsert); err != nil {
		// Quyền lạ (FE deploy trước BE seed catalog) là lỗi INPUT, không phải lỗi hệ thống:
		// trả 422 kèm tên quyền để admin biết chính xác cái nào chưa tồn tại, thay vì 500 câm.
		var unknownErr *authSvc.ErrUnknownPermissions
		if errors.As(err, &unknownErr) {
			response.Validate(c, i18n.TF(c, "permission_not_in_catalog", strings.Join(unknownErr.Names, ", ")))
			return
		}
		response.ServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": i18n.T(c, "updated")})
}

// GET /auth/rbac/child-role-for-dropdown
func (h *Handler) ChildRoleForDropdown(c *gin.Context) {
	if !h.requirePerm(c, "viewRole") {
		return
	}
	roles, err := h.RbacSvc.GetRolesDropdown()
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}
	response.Data(c, roles)
}

// GET /auth/role
func (h *Handler) ListRole(c *gin.Context) {
	if !h.requirePerm(c, "viewRole") {
		return
	}
	keyword := c.Query("_keyword")
	roles, err := h.RbacSvc.ListRoles(keyword)
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}
	c.JSON(http.StatusOK, roles)
}

// GET /auth/role/get-name?name=xxx
func (h *Handler) GetRoleName(c *gin.Context) {
	if !h.requirePerm(c, "viewRole") {
		return
	}
	name := c.Query("name")
	if name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	role, err := h.RbacSvc.GetRole(name)
	if err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, role)
}

// POST /auth/role/create-name
func (h *Handler) CreateRoleName(c *gin.Context) {
	if !h.requirePerm(c, "createRole") {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	if err := h.RbacSvc.CreateRole(body.Name, body.Description); err != nil {
		if errors.Is(err, authSvc.ErrRbacNameExists) {
			response.Validate(c, i18n.T(c, "rbac_name_exists"))
			return
		}
		response.ServerError(c, err)
		return
	}
	response.CreatedDataWithMessage(c, gin.H{"name": body.Name, "description": body.Description}, i18n.T(c, "created"))
}

// PUT /auth/role/update-name?name=xxx
func (h *Handler) UpdateRoleName(c *gin.Context) {
	if !h.requirePerm(c, "updateRole") {
		return
	}
	name := c.Query("name")
	if name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	// Whitelist — chỉ description được sửa. Tránh mass-assignment vào rule_name/data/timestamps.
	updates := map[string]interface{}{}
	if v, ok := body["description"]; ok {
		updates["description"] = v
	}
	if err := h.RbacSvc.UpdateRole(name, updates); err != nil {
		response.HandleError(c, err)
		return
	}
	response.DataWithMessage(c, gin.H{"name": name, "description": updates["description"]}, i18n.T(c, "updated"))
}

// DELETE /auth/role/delete-name?name=xxx
func (h *Handler) DeleteRoleName(c *gin.Context) {
	if !h.requirePerm(c, "deleteRole") {
		return
	}
	name := c.Query("name")
	if name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	// Role gốc — không cho xoá kể cả bởi user có permission deleteRole (chỉ super-user
	// mới toàn quyền hệ thống thật sự; xoá "admin" là thao tác không thể hoàn tác).
	// Các role super khác (develop) cũng chỉ super-user được xoá.
	if name == define.RoleAdmin || (define.IsSuperRole(name) && !h.RbacSvc.IsSuperUser(middleware.GetUserID(c))) {
		response.Validate(c, i18n.T(c, "rbac_cannot_delete_admin_role"))
		return
	}
	if err := h.RbacSvc.DeleteRole(name); err != nil {
		response.ServerError(c, err)
		return
	}
	response.Deleted(c)
}

// ── Permission (type=2) ──

// GET /auth/permission
func (h *Handler) ListPermission(c *gin.Context) {
	if !h.requirePerm(c, "viewRole") {
		return
	}
	perms, err := h.RbacSvc.ListPermissions(c.Query("_keyword"))
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}
	c.JSON(http.StatusOK, perms)
}

// GET /auth/permission/get-name?name=xxx
func (h *Handler) GetPermissionName(c *gin.Context) {
	if !h.requirePerm(c, "viewRole") {
		return
	}
	name := c.Query("name")
	if name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	item, err := h.RbacSvc.GetPermission(name)
	if err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

// POST /auth/permission/create-name
func (h *Handler) CreatePermissionName(c *gin.Context) {
	if !h.requireSuper(c) {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	if err := h.RbacSvc.CreatePermission(body.Name, body.Description); err != nil {
		response.ServerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, true)
}

// PUT /auth/permission/update-name?name=xxx
func (h *Handler) UpdatePermissionName(c *gin.Context) {
	if !h.requireSuper(c) {
		return
	}
	name := c.Query("name")
	if name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	delete(body, "name")
	delete(body, "type")
	if err := h.RbacSvc.UpdatePermission(name, body); err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, true)
}

// DELETE /auth/permission/delete-name?name=xxx
func (h *Handler) DeletePermissionName(c *gin.Context) {
	if !h.requireSuper(c) {
		return
	}
	name := c.Query("name")
	if name == "" {
		response.Validate(c, i18n.T(c, "rbac_name_required"))
		return
	}
	if err := h.RbacSvc.DeletePermission(name); err != nil {
		response.ServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, true)
}

// ListPermissionByRole GET /auth/rbac/list-permission?roleName=xxx
func (h *Handler) ListPermissionByRole(c *gin.Context) {
	if !h.requirePerm(c, "viewRole") {
		return
	}
	roleName := c.Query("roleName")
	if roleName == "" {
		response.Validate(c, i18n.T(c, "role_name_required"))
		return
	}
	c.JSON(http.StatusOK, h.RbacSvc.GetPermissionsByRole(roleName))
}

// Ghi chú: endpoint POST /auth/rbac/sync (SyncPermissions) đã gỡ — thay bằng startup seeder
// (BMAD perm-codegen-seeder). Lõi service RbacService.SyncPermissions vẫn dùng bởi SeedPermissions.

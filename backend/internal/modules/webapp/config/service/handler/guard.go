package handler

import (
	"erpCluvix/internal/middleware"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

// stripBhytMappingFields bỏ các cột mapping BHYT/BYT (mã kỹ thuật BYT, mã chuyên khoa Phụ lục 2, cờ
// công khám) khỏi payload ghi. Trước story-12 crm-standalone 2 mã này đi qua guard validate theo danh
// mục BYT (bảng EMR) + seclog; CRM không còn danh mục đó nên KHÔNG cho ghi thay vì ghi không kiểm.
// Payload service là map thô từ request (không whitelist) nên phải xoá key tường minh.
func stripBhytMappingFields(updates map[string]interface{}) {
	for _, k := range []string{"byt_code", "examination_specialty_code", "is_examination"} {
		delete(updates, k)
	}
}

// guardExportPermission kiểm quyền manageConfig trước khi export — route `_export=1` dùng chung
// GET /config/service với list thường, và PermissionEnforcer đang audit-mode nên không chặn thật,
// nên phải tự check quyền export ở đây (cùng lý do với hasServiceWritePerm). Trả false + đã tự response.Forbidden khi bị chặn.
func (h *Handler) guardExportPermission(c *gin.Context) bool {
	uid := middleware.GetUserID(c)
	if h.RbacSvc == nil || !hasManageConfigPerm(h.RbacSvc, uid, define.PermManageConfig) {
		response.Forbidden(c)
		return false
	}
	return true
}

// hasServiceWritePerm — guard quyền GHI cho luồng import Excel. Import tạo/sửa hàng loạt dịch vụ
// (giá, hoa hồng) nên phải đòi đúng quyền của thao tác: dòng có id = update, dòng không id = create.
// route_perm_map.json khai `manageConfig` cho endpoint này, nhưng PermissionEnforcer đang chạy
// audit-mode (permEnforce=false) nên không chặn thật — quyền ghi phải tự kiểm ở đây.
func (h *Handler) hasServiceWritePerm(c *gin.Context, perm string) bool {
	if h.RbacSvc == nil {
		return false
	}
	return hasManageConfigPerm(h.RbacSvc, middleware.GetUserID(c), perm)
}

// hasManageConfigPerm — user có quyền `perm` (super-role tự có tất cả perm qua GetUserPermissions).
// LƯU Ý: super chỉ "có tất cả perm" theo nghĩa `SELECT name FROM rbac_auth_item WHERE type=2` —
// perm đã bị xoá khỏi catalog thì super CŨNG không có. Đừng truyền key đã xoá vào đây.
func hasManageConfigPerm(rbac *authSvc.RbacService, uid uint, perm string) bool {
	perms, err := rbac.GetUserPermissions(uid)
	if err != nil {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// Package handler — bố cục dashboard CRM tuỳ biến (port từ ERP epic crm-dashboard-customizable story-04).
//
// 1 DB = 1 thương hiệu (D15) ⇒ phạm vi bố cục chỉ là user (từ SESSION) + bản "mặc định thương hiệu"
// (`user_id IS NULL`). Không nhận id nào từ client ⇒ không có IDOR theo id.
//
// Bố cục KHÔNG đi qua IndexedDB sync (dữ liệu theo TỪNG user; sync là master-data chung, mỗi lần ghi phải
// bump phiên bản cho mọi client) ⇒ đường ghi ở đây CỐ Ý KHÔNG gọi bumpDexieVersion/BumpDexieTable. FE cache
// bản tạm ở localStorage.
package handler

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erpCluvix/internal/middleware"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	dao "erpCluvix/internal/modules/webapp/crm/dashboard_layout/dao"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
)

type Deps struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// upsertInput — DTO riêng (KHÔNG bind entity): user_id/id trong body bị bỏ qua.
type upsertInput struct {
	Config json.RawMessage `json:"config"`
}

// layoutView — response: bố cục hiệu lực + nguồn. `config` = null khi source = system (FE dùng mặc định
// trong registry).
type layoutView struct {
	Config          datatypes.JSON `json:"config"`
	Source          string         `json:"source"`
	HasBrandDefault bool           `json:"hasBrandDefault"`
	UpdatedAt       int64          `json:"updatedAt"`
}

// currentUser — user từ SESSION (không lấy từ body). uid = 0 (không có phiên) ⇒ 403, fail-closed.
func currentUser(c *gin.Context) (uint, bool) {
	uid := middleware.GetUserID(c)
	if uid == 0 {
		response.Forbidden(c)
		return 0, false
	}
	return uid, true
}

// canManageBrandDefault — chỉ người có manageConfig được lưu/xoá mặc định thương hiệu. Kiểm TẠI ĐÂY bằng
// HasAnyPermStrict (không phụ thuộc PERM_ENFORCE audit-mode, super-role luôn qua, RbacSvc nil ⇒ false).
func (h *Handler) canManageBrandDefault(c *gin.Context) bool {
	return middleware.HasAnyPermStrict(c, h.RbacSvc, define.PermManageConfig)
}

// load đọc bản của user và bản mặc định thương hiệu (tối đa 2 dòng nhờ UNIQUE).
func (h *Handler) load(c *gin.Context, userID uint) (*dao.DashboardLayout, *dao.DashboardLayout, error) {
	var rows []dao.DashboardLayout
	// dashboard_layout không có branch_id ⇒ ScopedDBNullBranch không thêm vế nào; giữ để đúng quy ước scope.
	err := tenant.ScopedDBNullBranch(c, h.DB, "dashboard_layout").
		Select("id, user_id, dashboard_key, config, created_at, updated_at").
		Where("dashboard_key = ? AND (user_id = ? OR user_id IS NULL)", define.CrmDashboardKey, userID).
		Find(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	var mine, brand *dao.DashboardLayout
	for i := range rows {
		if rows[i].UserID == nil {
			brand = &rows[i]
		} else if *rows[i].UserID == userID {
			mine = &rows[i]
		}
	}
	return mine, brand, nil
}

func (h *Handler) view(c *gin.Context, userID uint) (layoutView, error) {
	mine, brand, err := h.load(c, userID)
	if err != nil {
		return layoutView{}, err
	}
	v := layoutView{Source: define.CrmDashboardLayoutSourceSystem, HasBrandDefault: brand != nil}
	switch {
	case mine != nil:
		v.Source, v.Config, v.UpdatedAt = define.CrmDashboardLayoutSourceUser, mine.Config, mine.UpdatedAt
	case brand != nil:
		v.Source, v.Config, v.UpdatedAt = define.CrmDashboardLayoutSourceBrand, brand.Config, brand.UpdatedAt
	}
	return v, nil
}

// Get — GET /crm/crm-dashboard/layout. Bố cục hiệu lực: của user > mặc định thương hiệu > hệ thống (config null).
func (h *Handler) Get(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	v, err := h.view(c, uid)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}
	response.Data(c, v)
}

// Save — PUT /crm/crm-dashboard/layout. Lưu bố cục của CHÍNH user đang đăng nhập.
func (h *Handler) Save(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	h.upsert(c, uid, &uid)
}

// SaveBrandDefault — PUT /crm/crm-dashboard/layout/brand-default. Cần manageConfig.
func (h *Handler) SaveBrandDefault(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	if !h.canManageBrandDefault(c) {
		response.Forbidden(c)
		return
	}
	h.upsert(c, uid, nil)
}

func (h *Handler) upsert(c *gin.Context, uid uint, owner *uint) {
	var in upsertInput
	if err := c.ShouldBindJSON(&in); err != nil {
		// Không đẩy lỗi decode thô ra client.
		response.Validate(c, i18n.T(c, "crm_dashboard_layout_invalid"))
		return
	}
	norm, key := parseConfig(in.Config)
	if key != "" {
		response.Validate(c, i18n.T(c, key))
		return
	}
	row := dao.DashboardLayout{UserID: owner, DashboardKey: define.CrmDashboardKey, Config: datatypes.JSON(norm)}
	// INSERT … ON DUPLICATE KEY UPDATE trên UNIQUE (COALESCE(user_id,0), dashboard_key): 2 lần lưu đồng thời
	// không sinh dòng thứ 2. created_by/created_at giữ nguyên khi đã có dòng.
	err := h.DB.Set("userID", uid).Clauses(clause.OnConflict{
		DoUpdates: clause.AssignmentColumns([]string{"config", "updated_by", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		response.ServerError(c, err)
		return
	}
	// Đọc lại sau khi ghi = đường ĐỌC ⇒ FailQuery (ServerErrorReadOnly).
	v, err := h.view(c, uid)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}
	response.Updated(c, v)
}

// Reset — DELETE /crm/crm-dashboard/layout. Xoá bản của user = "Khôi phục mặc định". Idempotent.
func (h *Handler) Reset(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	h.remove(c, uid, &uid)
}

// DeleteBrandDefault — DELETE /crm/crm-dashboard/layout/brand-default. Cần manageConfig. Idempotent.
func (h *Handler) DeleteBrandDefault(c *gin.Context) {
	uid, ok := currentUser(c)
	if !ok {
		return
	}
	if !h.canManageBrandDefault(c) {
		response.Forbidden(c)
		return
	}
	h.remove(c, uid, nil)
}

func (h *Handler) remove(c *gin.Context, uid uint, owner *uint) {
	q := tenant.ScopedDBNullBranch(c, h.DB, "dashboard_layout").
		Where("dashboard_key = ?", define.CrmDashboardKey)
	if owner == nil {
		q = q.Where("user_id IS NULL")
	} else {
		q = q.Where("user_id = ?", *owner)
	}
	if err := q.Delete(&dao.DashboardLayout{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.ServerError(c, err)
		return
	}
	// Đọc lại sau khi ghi = đường ĐỌC ⇒ FailQuery (ServerErrorReadOnly).
	v, err := h.view(c, uid)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}
	response.Data(c, v)
}

package handler

import (
	"errors"
	"strconv"
	"strings"

	"erpCluvix/internal/middleware"
	ptDao "erpCluvix/internal/modules/webapp/config/print_template/dao"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/optlock"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct{ DB *gorm.DB }
type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// List — GET /config/print-template
func (h *Handler) List(c *gin.Context) {
	q := tenant.ScopedDBNullBranch(c, h.DB, "print_templates").
		Model(&ptDao.PrintTemplate{}).
		Select("id, branch_id, template_type, name, description, is_default, is_active, created_at, updated_at, created_by, updated_by").
		Order("print_templates.id DESC")

	if kw := c.Query("_keyword"); kw != "" {
		like := "%" + strings.TrimSpace(kw) + "%"
		q = q.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	q = query.ApplyFilter(c, q, "template_type", "template_type")
	q = query.ApplyFilter(c, q, "branch_id", "branch_id")
	q = query.ApplyFilter(c, q, "is_default", "is_default")
	q = query.ApplyFilter(c, q, "is_active", "is_active")

	var items []ptDao.PrintTemplate
	c.JSON(200, pagination.PaginateTyped(c, q, &items))
}

// Get — GET /config/print-template/:id
func (h *Handler) Get(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item ptDao.PrintTemplate
	if err := tenant.ScopedDBNullBranch(c, h.DB, "print_templates").
		First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

// GetDefault — GET /config/print-template/default?type=payment&branch_id=1
// Trả về template mặc định của (branch, type), fallback về mẫu chung (branch_id=0, type).
//
// Story-39 (E6/D15, Q2): KHÔNG còn theo company (bỏ lỗi print_template.err_company_required). Dữ
// liệu cũ nhiều thương hiệu có thể có >1 mẫu mặc định cùng khoá ⇒ ORDER BY id DESC (bản mới nhất)
// để kết quả xác định, không phụ thuộc thứ tự vật lý.
func (h *Handler) GetDefault(c *gin.Context) {
	templateType := c.Query("type")
	branchParam := c.Query("branch_id")
	if templateType == "" {
		response.Validate(c, i18n.T(c, "print_template.err_type_required"))
		return
	}

	var item ptDao.PrintTemplate

	// Ưu tiên: branch cụ thể → mẫu chung (branch_id=0). branch_id phải parse ra số trước khi vào
	// query (Hard Rule): giá trị rác ("undefined") coi như không gửi ⇒ rơi về mẫu chung.
	if branchID, err := strconv.ParseUint(strings.TrimSpace(branchParam), 10, 64); err == nil && branchID > 0 {
		if err := h.DB.
			Where("branch_id = ? AND template_type = ? AND is_default = 1 AND is_active = 1",
				branchID, templateType).
			Order("id DESC").
			First(&item).Error; err == nil {
			response.Data(c, item)
			return
		}
	}
	// Fallback: mẫu chung
	if err := h.DB.
		Where("branch_id = 0 AND template_type = ? AND is_default = 1 AND is_active = 1", templateType).
		Order("id DESC").
		First(&item).Error; err != nil {
		// Chưa cấu hình template là trạng thái hợp lệ — trả null thay vì 404
		// để FE không thấy network error và tự fall back layout mặc định.
		response.Data(c, nil)
		return
	}
	response.Data(c, item)
}

// Create — POST /config/print-template
func (h *Handler) Create(c *gin.Context) {
	var input createInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.HandleError(c, err)
		return
	}
	if err := validateInput(&input.baseInput); err != nil {
		response.Validate(c, i18n.T(c, err.Error()))
		return
	}

	uid := middleware.GetUserID(c)
	item := inputToEntity(&input.baseInput, uid)

	// Mỗi (branch, type) chỉ 1 mẫu mặc định.
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if item.IsDefault == 1 {
			if err := tx.Table("print_templates").
				Where("branch_id = ? AND template_type = ?",
					item.BranchId, item.TemplateType).
				Update("is_default", 0).Error; err != nil {
				return err
			}
		}
		return tx.Create(&item).Error
	})
	if err != nil {
		response.ServerError(c, err)
		return
	}
	response.Created(c, item)
}

// Update — PUT /config/print-template/:id
func (h *Handler) Update(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var current ptDao.PrintTemplate
	if err := tenant.ScopedDBNullBranch(c, h.DB, "print_templates").
		First(&current, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var input updateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.HandleError(c, err)
		return
	}
	if err := validateInput(&input.baseInput); err != nil {
		response.Validate(c, i18n.T(c, err.Error()))
		return
	}

	uid := middleware.GetUserID(c)
	applyInput(&current, &input.baseInput, uid)

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if current.IsDefault == 1 {
			if err := tx.Table("print_templates").
				Where("branch_id = ? AND template_type = ? AND id != ?",
					current.BranchId, current.TemplateType, current.ID).
				Update("is_default", 0).Error; err != nil {
				return err
			}
		}
		// Ghi có khoá: WHERE updated_at = expected + bump GREATEST(now, updated_at+1); 0 dòng ⇒
		// errs.ErrVersionConflict (D21, thay so sánh `version` cũ).
		return optlock.Save(tx, &current, input.UpdatedAt)
	})
	if err != nil {
		// 409 cùng i18n key `version_conflict` như nhánh `version` cũ; lỗi khác (vd mất kết nối DB
		// giữa lúc chạy transaction) vẫn ServerError như trước, KHÔNG đổi qua response.HandleError
		// (tránh lộ raw error text ra 422 cho các lỗi hạ tầng ngoài optlock).
		if errors.Is(err, errs.ErrVersionConflict) {
			response.Conflict(c, i18n.T(c, err.Error()))
			return
		}
		response.ServerError(c, err)
		return
	}
	response.Updated(c, current)
}

// Delete — DELETE /config/print-template/:id
func (h *Handler) Delete(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item ptDao.PrintTemplate
	if err := tenant.ScopedDBNullBranch(c, h.DB, "print_templates").
		First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if item.IsDefault == 1 {
		response.Validate(c, i18n.T(c, "print_template.err_delete_default"))
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Deleted(c)
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

type baseInput struct {
	BranchId     uint   `json:"branch_id"`
	TemplateType string `json:"template_type" binding:"required,max=50"`
	Name         string `json:"name"          binding:"required,max=255"`
	Description  string `json:"description"`
	TemplateHtml string `json:"template_html" binding:"required"`
	IsDefault    int    `json:"is_default"`
	IsActive     *int   `json:"is_active"`
}

type createInput struct{ baseInput }
type updateInput struct {
	baseInput
	UpdatedAt int64 `json:"updated_at" binding:"required"`
}

func validateInput(in *baseInput) error {
	if !define.ValidPrintTemplateTypes[in.TemplateType] {
		return errs.NewLocalizedError("print_template.err_type_invalid")
	}
	return nil
}

func inputToEntity(in *baseInput, uid uint) ptDao.PrintTemplate {
	isActive := 1
	if in.IsActive != nil {
		isActive = *in.IsActive
	}
	return ptDao.PrintTemplate{
		BranchId:     in.BranchId,
		TemplateType: in.TemplateType,
		Name:         in.Name,
		Description:  in.Description,
		TemplateHtml: in.TemplateHtml,
		IsDefault:    in.IsDefault,
		IsActive:     isActive,
		CreatedBy:    uid,
		UpdatedBy:    uid,
	}
}

func applyInput(e *ptDao.PrintTemplate, in *baseInput, uid uint) {
	e.BranchId = in.BranchId
	e.TemplateType = in.TemplateType
	e.Name = in.Name
	e.Description = in.Description
	e.TemplateHtml = in.TemplateHtml
	e.IsDefault = in.IsDefault
	if in.IsActive != nil {
		e.IsActive = *in.IsActive
	}
	e.UpdatedBy = uid
}

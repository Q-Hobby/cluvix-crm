package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"errors"
	"net/http"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/config/config_code/dao"
	"erpCluvix/pkg/codegen"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/repository"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// Deps + Handler — module-local (Plan 06). Full Deps để preserve h.Cache compatibility.
type Deps struct {
	DB    *gorm.DB
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) ListConfigCode(c *gin.Context) {
	query := tenant.ScopedDBNullBranch(c, h.DB, "config_code").Model(&dao.ConfigCode{})

	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}

	var items []dao.ConfigCode
	result := pagination.Paginate(c, query.Order("config_code.id DESC"), &items)
	c.JSON(http.StatusOK, result)
}

// GetCustomerCodeOption trả kiểu mã khách đang chạy (env CUSTOMER_CODE_OPTION, pkg/codegen). Khác
// option mặc định thì dòng config_code "customer" chỉ còn là bộ đếm (+ infix cho option 5/6) — FE báo
// cho người sửa cấu hình biết, tránh sửa rồi tưởng hệ thống lỗi.
func (h *Handler) GetCustomerCodeOption(c *gin.Context) {
	response.Data(c, gin.H{
		"option":         codegen.CustomerCodeOption(),
		"default_option": define.CustomerCodeOptionDefault,
	})
}

// Plan 09 (2026-05-16): BaseRepo migration. Fix IDOR — `config_code` là
// tenant table; First() cũ không scope.
func (h *Handler) GetConfigCode(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	repo := repository.New[dao.ConfigCode](h.DB, "config_code")
	item, err := repo.FindByID(c, id, nil)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			response.NotFound(c)
			return
		}
		response.ServerError(c, err)
		return
	}
	response.Data(c, item)
}

// parentTableTaken — đã có dòng config_code khác cùng (branch_id, parent_table) chưa. DB có UNIQUE
// (branch_id, parent_table) nhưng lỗi 1062 thô sẽ ra 500; kiểm trước để trả lỗi i18n thân thiện.
// Scope qua ScopedDBNullBranch như mọi truy vấn config_code; excludeID loại chính dòng đang sửa.
func (h *Handler) parentTableTaken(c *gin.Context, branchID int, parentTable string, excludeID uint) (bool, error) {
	var n int64
	q := tenant.ScopedDBNullBranch(c, h.DB, "config_code").Table("config_code").
		Where("config_code.branch_id = ? AND config_code.parent_table = ?", branchID, parentTable)
	if excludeID > 0 {
		q = q.Where("config_code.id <> ?", excludeID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func (h *Handler) CreateConfigCode(c *gin.Context) {
	var item dao.ConfigCode
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	// Create: khóa chính do DB cấp — reset `id` client gửi kèm trong body/form (mass-assignment).
	item.ID = 0
	// parent_table bắt buộc: UNIQUE (branch_id, parent_table) không chặn được giá trị rỗng/NULL
	// (story-62, Dev Notes story-25) — row thiếu bảng đích cũng vô dụng với codegen.
	if !requireFields(c, [][2]any{{item.BranchId, "Chi nhanh"}, {item.ParentTable, "Bang du lieu"}}) {
		return
	}
	taken, err := h.parentTableTaken(c, item.BranchId, item.ParentTable, 0)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	if taken {
		response.Validate(c, i18n.T(c, "config_code_parent_table_exists"))
		return
	}
	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))
	item.CreatedAt = now
	item.UpdatedAt = now
	item.CreatedBy = &uid
	item.UpdatedBy = &uid

	if err := h.DB.Create(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *Handler) UpdateConfigCode(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.ConfigCode
	if err := tenant.ScopedDBNullBranch(c, h.DB, "config_code").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	if err := helpers.BindUpdate(c, &item); err != nil {
		response.HandleError(c, err)
		return
	}
	// parent_table bắt buộc: UNIQUE (branch_id, parent_table) không chặn được giá trị rỗng/NULL
	// (story-62, Dev Notes story-25) — row thiếu bảng đích cũng vô dụng với codegen.
	if !requireFields(c, [][2]any{{item.BranchId, "Chi nhanh"}, {item.ParentTable, "Bang du lieu"}}) {
		return
	}

	taken, err := h.parentTableTaken(c, item.BranchId, item.ParentTable, item.ID)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	if taken {
		response.Validate(c, i18n.T(c, "config_code_parent_table_exists"))
		return
	}

	now := time.Now().Unix()
	item.UpdatedAt = now
	item.UpdatedBy = new(int(middleware.GetUserID(c)))

	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Updated(c, item)
}

func (h *Handler) DeleteConfigCode(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	result := tenant.ScopedDBNullBranch(c, h.DB, "config_code").Table("config_code").Where("id = ?", id).Delete(nil)
	if result.Error != nil {
		response.Validate(c, i18n.T(c, "fk_constraint"))
		return
	}
	if result.RowsAffected == 0 {
		response.NotFound(c)
		return
	}
	response.Deleted(c)
}

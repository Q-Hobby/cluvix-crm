package handler

import (
	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	hrConfigDao "erpCluvix/internal/modules/webapp/config/hr_department/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

// Deps + Handler — module-local (Plan 06 B1).
type Deps struct {
	DB *gorm.DB
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

// requireValidHeadUser validate head_user_id (nếu có) tồn tại. Story-39 (E6/D15): bỏ lọc JSON
// user.companyId — DB 1 thương hiệu; bảng `user` không đi qua ScopedDB* (Hard Rule).
func (h *Handler) requireValidHeadUser(c *gin.Context, headUserID *int) bool {
	if headUserID == nil {
		return true
	}
	var userExists int64
	if err := h.DB.Table("user").Where("id = ?", *headUserID).Count(&userExists).Error; err != nil {
		response.ServerError(c, err)
		return false
	}
	if userExists == 0 {
		response.Validate(c, i18n.T(c, "user_not_found"))
		return false
	}
	return true
}

func (h *Handler) ListHrDepartment(c *gin.Context) {
	query := h.DB.Model(&hrConfigDao.HrDepartment{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []hrConfigDao.HrDepartment
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetHrDepartment(c *gin.Context) {
	var item hrConfigDao.HrDepartment
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateHrDepartment(c *gin.Context) {
	var item hrConfigDao.HrDepartment
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	// Create: khóa chính do DB cấp — reset `id` client gửi kèm trong body/form (mass-assignment).
	item.ID = 0
	if !requireName(c, item.Name) {
		return
	}
	if !h.requireValidHeadUser(c, item.HeadUserId) {
		return
	}
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Create(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	h.DB.Model(&item).Update("SortOrder", item.ID)
	dexie.BumpDexieTable(h.DB, "hrDepartments")
	response.Created(c, item)
}

func (h *Handler) UpdateHrDepartment(c *gin.Context) {
	var item hrConfigDao.HrDepartment
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if err := helpers.BindUpdate(c, &item); err != nil {
		response.HandleError(c, err)
		return
	}
	if !requireName(c, item.Name) {
		return
	}
	if !h.requireValidHeadUser(c, item.HeadUserId) {
		return
	}
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "hrDepartments")
	response.Updated(c, item)
}

func (h *Handler) DeleteHrDepartment(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if id <= 14 {
		response.Validate(c, i18n.T(c, "default_department_cannot_delete"))
		return
	}
	if err := models.DeleteWithCheck(h.DB, &hrConfigDao.HrDepartment{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "hrDepartments")
	response.Deleted(c)
}

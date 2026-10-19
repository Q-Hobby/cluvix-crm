package handler

import (
	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/ethnicity/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
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

// --- List ---

func (h *Handler) ListEthnicity(c *gin.Context) {
	query := h.DB.Model(&dao.Ethnicity{})

	// Keyword search
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}

	// Filter
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}

	query = query.Order("SortOrder DESC, id ASC")

	var items []dao.Ethnicity
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

// --- Get ---

func (h *Handler) GetEthnicity(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.Ethnicity
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

// --- Create ---

func (h *Handler) CreateEthnicity(c *gin.Context) {
	var item dao.Ethnicity
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	// Create: khóa chính do DB cấp — reset `id` client gửi kèm trong body/form (mass-assignment).
	item.ID = 0
	if !requireName(c, item.Name) {
		return
	}

	if err := h.DB.Set("userID", middleware.GetUserID(c)).Create(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	// Auto SortOrder = id
	if item.SortOrder == nil {
		sort := int(item.ID)
		h.DB.Model(&item).Update("SortOrder", sort)
	}

	dexie.BumpDexieTable(h.DB, "ethnicity")
	response.Created(c, item)
}

// --- Update ---

func (h *Handler) UpdateEthnicity(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.Ethnicity
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

	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	dexie.BumpDexieTable(h.DB, "ethnicity")
	response.Updated(c, item)
}

// --- Delete ---

func (h *Handler) DeleteEthnicity(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.Ethnicity{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "ethnicity")
	response.Deleted(c)
}

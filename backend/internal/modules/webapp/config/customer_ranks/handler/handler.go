package handler

import (
	"gorm.io/gorm"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/customer_ranks/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"net/http"
	"strings"

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

func (h *Handler) ListCustomerRanks(c *gin.Context) {
	query := h.DB.Model(&dao.CustomerRanks{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("id DESC")
	var items []dao.CustomerRanks
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetCustomerRanks(c *gin.Context) {
	var item dao.CustomerRanks
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

func (h *Handler) CreateCustomerRanks(c *gin.Context) {
	var item dao.CustomerRanks
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
	dexie.BumpDexieTable(h.DB, "customerRanks")
	response.Created(c, item)
}

func (h *Handler) UpdateCustomerRanks(c *gin.Context) {
	var item dao.CustomerRanks
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
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "customerRanks")
	response.Updated(c, item)
}

func (h *Handler) DeleteCustomerRanks(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.CustomerRanks{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "customerRanks")
	response.Deleted(c)
}

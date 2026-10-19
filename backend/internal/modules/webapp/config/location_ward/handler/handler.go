package handler

import (
	"gorm.io/gorm"
	"strconv"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	dao "erpCluvix/internal/modules/webapp/config/location_ward/dao"
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

func (h *Handler) ListLocationWard(c *gin.Context) {
	query := h.DB.Model(&dao.LocationWard{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []dao.LocationWard
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

// Lấy danh sách phường/xã theo tỉnh/thành phố, trả về dạng dropdown {label, value}
func (h *Handler) GetLocationWardByProvince(c *gin.Context) {
	provinceId, errParse := strconv.ParseUint(c.Query("province_id"), 10, 64)
	// Parse ra SỐ: MySQL ép '12abc' → 12 (warning 1292 bị nuốt) ⇒ URL rác đọc trúng dữ liệu khách khác.
	if errParse != nil || provinceId == 0 {
		response.Validate(c, i18n.T(c, "province_id_required"))
		return
	}
	type wardOption struct {
		Label string `json:"label"`
		Value uint   `json:"value"`
	}
	var items []wardOption
	if err := h.DB.Model(&dao.LocationWard{}).Select("name AS label, id AS value").
		Where("province_id = ?", provinceId).Order("SortOrder DESC, id ASC").Find(&items).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	if items == nil {
		items = make([]wardOption, 0)
	}
	response.Data(c, items)
}
func (h *Handler) GetLocationWard(c *gin.Context) {
	var item dao.LocationWard
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
func (h *Handler) CreateLocationWard(c *gin.Context) {
	var item dao.LocationWard
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
	h.DB.Model(&item).Update("SortOrder", item.ID)

	response.Created(c, item)
}
func (h *Handler) UpdateLocationWard(c *gin.Context) {
	var item dao.LocationWard
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

	response.Updated(c, item)
}
func (h *Handler) DeleteLocationWard(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.LocationWard{}, id); err != nil {
		response.HandleError(c, err)
		return
	}

	response.Deleted(c)
}

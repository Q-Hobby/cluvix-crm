package handler

import (
	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/appointment_is_accepted/dao"
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

func (h *Handler) ListAppointmentIsAccepted(c *gin.Context) {
	query := h.DB.Model(&dao.AppointmentIsAccepted{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []dao.AppointmentIsAccepted
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetAppointmentIsAccepted(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.AppointmentIsAccepted
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateAppointmentIsAccepted(c *gin.Context) {
	var item dao.AppointmentIsAccepted
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
	dexie.BumpDexieTable(h.DB, "appointmentIsAccepteds")
	response.Created(c, item)
}

func (h *Handler) UpdateAppointmentIsAccepted(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.AppointmentIsAccepted
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
	dexie.BumpDexieTable(h.DB, "appointmentIsAccepteds")
	response.Updated(c, item)
}

func (h *Handler) DeleteAppointmentIsAccepted(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if id < 3 {
		response.Validate(c, i18n.T(c, "cannot_delete_no_permission"))
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.AppointmentIsAccepted{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "appointmentIsAccepteds")
	response.Deleted(c)
}

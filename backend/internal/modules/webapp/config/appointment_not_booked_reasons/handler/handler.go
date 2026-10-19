package handler

import (
	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/appointment_not_booked_reasons/dao"
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

func (h *Handler) ListAppointmentNotBookedReasons(c *gin.Context) {
	query := h.DB.Model(&dao.AppointmentNotBookedReasons{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []dao.AppointmentNotBookedReasons
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetAppointmentNotBookedReasons(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.AppointmentNotBookedReasons
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateAppointmentNotBookedReasons(c *gin.Context) {
	var item dao.AppointmentNotBookedReasons
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
	dexie.BumpDexieTable(h.DB, "appointmentNotBookedReasons")
	response.Created(c, item)
}

func (h *Handler) UpdateAppointmentNotBookedReasons(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.AppointmentNotBookedReasons
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
	dexie.BumpDexieTable(h.DB, "appointmentNotBookedReasons")
	response.Updated(c, item)
}

func (h *Handler) DeleteAppointmentNotBookedReasons(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.AppointmentNotBookedReasons{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "appointmentNotBookedReasons")
	response.Deleted(c)
}

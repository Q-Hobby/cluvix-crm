package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/appointment_type/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"

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

func (h *Handler) ListAppointmentType(c *gin.Context) {
	query := h.DB.Model(&dao.AppointmentType{}).
		Select("id, name, isTypeRevenue, SortOrder, status, created_by, updated_by, created_at, updated_at")
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []dao.AppointmentType
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetAppointmentType(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.AppointmentType
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateAppointmentType(c *gin.Context) {
	var item dao.AppointmentType
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
	h.invalidateAppointmentTypeCache()
	response.Created(c, item)
}

func (h *Handler) UpdateAppointmentType(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.AppointmentType
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	// BindUpdate ghim `ID` sau khi bind: `Save()` ghi theo khoá chính trong struct, để client
	// gửi `{"id": 99}` là sửa sang bản ghi khác.
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
	h.invalidateAppointmentTypeCache()
	response.Updated(c, item)
}

func (h *Handler) DeleteAppointmentType(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if id < 5 {
		response.Validate(c, i18n.T(c, "cannot_delete_no_permission"))
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.AppointmentType{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	h.invalidateAppointmentTypeCache()
	response.Deleted(c)
}

// --- Custom toggle actions ---

func (h *Handler) ToggleAppointmentTypeRevenue(c *gin.Context) {
	h.toggleField(c, "appointment_type", "isTypeRevenue")
	h.invalidateAppointmentTypeCache()
}

// toggleField generic toggle 0↔1
func (h *Handler) toggleField(c *gin.Context, table string, field string) {
	var body struct {
		ID uint `json:"id" form:"id"`
	}
	c.ShouldBind(&body)
	if body.ID == 0 {
		response.Validate(c, i18n.T(c, "id_required"))
		return
	}
	var current int
	if err := h.DB.Table(table).Select(field).Where("id = ?", body.ID).Scan(&current).Error; err != nil {
		response.NotFound(c)
		return
	}
	newVal := 1
	if current == 1 {
		newVal = 0
	}
	h.DB.Table(table).Where("id = ?", body.ID).Update(field, newVal)
	response.DataWithMessage(c, nil, i18n.T(c, "updated"))
}

// --- Cache ---

func (h *Handler) invalidateAppointmentTypeCache() {
	if h.Cache == nil {
		return
	}
	h.Cache.Delete("dropdown:appointment_type:list")
	h.Cache.Delete("getAllAppointmentType")
	h.bumpDexieVersion()
}

// bumpDexieVersion — FE đọc header X-Version-Dexie để biết khi nào refresh IndexedDB
func (h *Handler) bumpDexieVersion() {
	dexie.BumpDexieTable(h.DB, "appointmentTypes")
}

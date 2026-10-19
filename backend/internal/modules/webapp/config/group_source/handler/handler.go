package handler

import (
	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/group_source/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

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

func (h *Handler) ListGroupSource(c *gin.Context) {
	query := tenant.ScopedDBNullBranch(c, h.DB, "group_source").Model(&dao.GroupSource{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR `desc` LIKE ?", like, like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []dao.GroupSource
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetGroupSource(c *gin.Context) {
	var item dao.GroupSource
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "group_source").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateGroupSource(c *gin.Context) {
	var item dao.GroupSource
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
	dexie.BumpDexieTable(h.DB, "groupSources")
	response.Created(c, item)
}

func (h *Handler) UpdateGroupSource(c *gin.Context) {
	var item dao.GroupSource
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "group_source").First(&item, id).Error; err != nil {
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
	dexie.BumpDexieTable(h.DB, "groupSources")
	response.Updated(c, item)
}

func (h *Handler) DeleteGroupSource(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.GroupSource{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "groupSources")
	response.Deleted(c)
}

func (h *Handler) CheckOnlineGroupSource(c *gin.Context) {
	var body struct {
		ID uint `json:"id" form:"id"`
	}
	c.ShouldBind(&body)
	if body.ID == 0 {
		response.Validate(c, i18n.T(c, "id_required"))
		return
	}

	var online int
	if err := h.DB.Table("group_source").Select("online").Where("id = ?", body.ID).Scan(&online).Error; err != nil {
		response.NotFound(c)
		return
	}

	newOnline := 1
	msgKey := "group_source_marked_online"
	if online == 1 {
		newOnline = 0
		msgKey = "group_source_marked_offline"
	}

	h.DB.Table("group_source").Where("id = ?", body.ID).Update("online", newOnline)
	response.Data(c, gin.H{"message": i18n.T(c, msgKey)})
}

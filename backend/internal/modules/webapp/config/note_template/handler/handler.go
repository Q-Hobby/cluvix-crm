package handler

import (
	"gorm.io/gorm"

	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/note_template/dao"
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

func (h *Handler) ListNoteTemplate(c *gin.Context) {
	query := h.DB.Model(&dao.NoteTemplate{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	if typeID := c.Query("filter[type_id]"); typeID != "" {
		query = query.Where("type_id = ?", typeID)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []dao.NoteTemplate
	result := pagination.Paginate(c, query, &items)

	// Gán typeLabel cho từng item
	typeLabelKeys := map[int]string{1: "note_template_type_appointment", 2: "note_template_type_consultation"}
	type NoteTemplateWithLabel struct {
		dao.NoteTemplate
		TypeLabel string `json:"type"`
	}
	labeledItems := make([]NoteTemplateWithLabel, len(items))
	for i, item := range items {
		label := ""
		if item.TypeId != nil {
			if key, ok := typeLabelKeys[*item.TypeId]; ok {
				label = i18n.T(c, key)
			}
		}
		labeledItems[i] = NoteTemplateWithLabel{NoteTemplate: item, TypeLabel: label}
	}
	result.Items = labeledItems
	c.JSON(http.StatusOK, result)
}
func (h *Handler) GetNoteTemplate(c *gin.Context) {
	var item dao.NoteTemplate
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
func (h *Handler) CreateNoteTemplate(c *gin.Context) {
	var item dao.NoteTemplate
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
	dexie.BumpDexieTable(h.DB, "noteTemplates")
	response.Created(c, item)
}
func (h *Handler) UpdateNoteTemplate(c *gin.Context) {
	var item dao.NoteTemplate
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
	dexie.BumpDexieTable(h.DB, "noteTemplates")
	response.Updated(c, item)
}
func (h *Handler) DeleteNoteTemplate(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.NoteTemplate{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "noteTemplates")
	response.Deleted(c)
}

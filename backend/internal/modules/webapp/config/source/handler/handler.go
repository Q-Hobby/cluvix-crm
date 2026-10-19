package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"encoding/json"
	"net/http"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	sourceDao "erpCluvix/internal/modules/webapp/config/source/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"

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

func (h *Handler) ListSource(c *gin.Context) {
	query := tenant.ScopedDBNullBranch(c, h.DB, "source").Model(&sourceDao.Source{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("SortOrder DESC, id ASC")
	var items []sourceDao.Source
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}
func (h *Handler) GetSource(c *gin.Context) {
	var item sourceDao.Source
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "source").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}
func (h *Handler) CreateSource(c *gin.Context) {
	var item sourceDao.Source
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
	dexie.BumpDexieTable(h.DB, "sources")
	response.Created(c, item)
}
func (h *Handler) UpdateSource(c *gin.Context) {
	var item sourceDao.Source
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "source").First(&item, id).Error; err != nil {
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
	dexie.BumpDexieTable(h.DB, "sources")
	response.Updated(c, item)
}
func (h *Handler) DeleteSource(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &sourceDao.Source{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "sources")
	response.Deleted(c)
}

// GET /config/source/overview-chart-source
func (h *Handler) OverviewChartSource(c *gin.Context) {
	now := time.Now().In(tz.LocFromContext(c, h.DB))
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	to := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Unix()

	type row struct {
		Total    int    `json:"total"`
		SourceID int    `json:"source_id"`
		Name     string `json:"nameSource"`
	}
	var rows []row
	tenant.ScopedDBNullBranch(c, h.DB, "customer").Table("customer").
		Select("COUNT(source_id) as total, source_id, source.name as name").
		Joins("LEFT JOIN source ON source.id = customer.source_id").
		Where("source_id IS NOT NULL AND source.status = 1").
		Where("created_day BETWEEN ? AND ?", from, to).
		Group("source_id").Order("total DESC").
		Scan(&rows)

	if len(rows) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	// Map source_id → group_name from group_source
	type gsRow struct {
		Name     string `json:"name"`
		SourceId string `json:"sourceId"`
	}
	var groups []gsRow
	tenant.ScopedDBNullBranch(c, h.DB, "group_source").Table("group_source").Select("name, sourceId").Where("status = 1").Find(&groups)

	sourceToGroup := map[int]string{}
	for _, g := range groups {
		var ids []int
		json.Unmarshal([]byte(g.SourceId), &ids)
		for _, id := range ids {
			sourceToGroup[id] = g.Name
		}
	}

	var result []gin.H
	othersValue := 0
	for i, r := range rows {
		groupName := sourceToGroup[r.SourceID]
		if groupName == "" {
			groupName = i18n.T(c, "source_other")
		}
		name := r.Name
		if name == "" {
			name = i18n.T(c, "source_unknown")
		}

		if i < 5 {
			result = append(result, gin.H{"name": name, "value": r.Total, "group": groupName})
		} else {
			othersValue += r.Total
		}
	}
	if othersValue > 0 {
		result = append(result, gin.H{"name": i18n.T(c, "source_other"), "value": othersValue, "group": i18n.T(c, "source_other")})
	}

	c.JSON(http.StatusOK, result)
}

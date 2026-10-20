package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	"erpCluvix/internal/modules/webapp/config/unit/dao"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/dexie"
	pkgExport "erpCluvix/pkg/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/datatypes"
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

func (h *Handler) ListUnit(c *gin.Context) {
	// Build where conditions
	var wheres []func(*gorm.DB) *gorm.DB

	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		wheres = append(wheres, func(q *gorm.DB) *gorm.DB {
			return q.Where("name LIKE ? OR description LIKE ?", like, like)
		})
	}
	if status := c.Query("filter[status]"); status != "" {
		wheres = append(wheres, func(q *gorm.DB) *gorm.DB {
			return q.Where("status = ?", status)
		})
	}
	if catID := c.Query("filter[unitCategory_id]"); catID != "" {
		wheres = append(wheres, func(q *gorm.DB) *gorm.DB {
			return q.Where("JSON_CONTAINS(CAST(categoryId AS JSON), ?)", catID)
		})
	}

	// Export — dùng Table() để Find vào map không bị pointer
	if c.Query("_export") == "1" {
		q := h.DB.Table("unit").Order("id DESC")
		for _, w := range wheres {
			q = w(q)
		}
		exportHandlers.ExportUnit(c, q)
		return
	}

	// List — dùng Model() để Paginate scan vào struct
	query := h.DB.Model(&dao.Unit{}).Order("id DESC")
	for _, w := range wheres {
		query = w(query)
	}

	var items []dao.Unit
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetUnit(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.Unit
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateUnit(c *gin.Context) {
	var item dao.Unit
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	// Create: khóa chính do DB cấp — reset `id` client gửi kèm trong body/form (mass-assignment).
	item.ID = 0
	if !requireName(c, item.Name) {
		return
	}

	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))
	item.CreatedAt = now
	item.UpdatedAt = now
	item.CreatedBy = &uid
	item.UpdatedBy = &uid

	if err := h.DB.Create(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	if item.SortOrder == nil {
		sort := int(item.ID)
		h.DB.Model(&item).Update("SortOrder", sort)
	}

	h.invalidateUnitCache()
	response.Created(c, item)
}

func (h *Handler) UpdateUnit(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.Unit
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	if v, ok := body["name"]; ok {
		if !requireFields(c, [][2]any{{v, "Tên đơn vị"}}) {
			return
		}
		item.Name = fmt.Sprintf("%v", v)
	}
	if v, ok := body["type"]; ok {
		item.Type = new(toIntVal(v))
	}
	if v, ok := body["description"]; ok {
		item.Description = fmt.Sprintf("%v", v)
	}
	if v, ok := body["categoryId"]; ok {
		b, _ := json.Marshal(v)
		item.CategoryId = datatypes.JSON(b)
	}
	if v, ok := body["SortOrder"]; ok {
		item.SortOrder = new(toIntVal(v))
	}
	if v, ok := body["status"]; ok {
		item.Status = new(toIntVal(v))
	}

	now := time.Now().Unix()
	item.UpdatedAt = now
	item.UpdatedBy = new(int(middleware.GetUserID(c)))

	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	h.invalidateUnitCache()
	response.Updated(c, item)
}

func (h *Handler) DeleteUnit(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &dao.Unit{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	h.invalidateUnitCache()
	response.Deleted(c)
}

func toIntVal(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	default:
		return 0
	}
}

func (h *Handler) invalidateUnitCache() {
	if h.Cache != nil {
		h.Cache.Delete("dropdown:unit:list")
		h.Cache.InvalidateTag("unit:list")
	}
	dexie.BumpDexieTable(h.DB, "units")
}

// ImportUnitFromExcel POST /config/unit/import-from-excel
func (h *Handler) ImportUnitFromExcel(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Validate(c, i18n.T(c, "file_required"))
		return
	}

	src, err := file.Open()
	if err != nil {
		response.ServerError(c, err)
		return
	}
	defer src.Close()

	f, err := excelize.OpenReader(src, pkgExport.ImportOpenOptions())
	if err != nil {
		response.Validate(c, i18n.TF(c, "excel_read_failed", err.Error()))
		return
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) < 2 {
		response.Validate(c, i18n.T(c, "no_data"))
		return
	}
	if len(rows)-1 > MaxImportRows {
		response.Validate(c, i18n.TF(c, "import_too_many_rows", MaxImportRows))
		return
	}

	// Skip header row
	rows = rows[1:]

	// Map cột: 0:id, 1:name, 2:type, 3:description, 4:categoryId, 5:(skip tên danh mục), 6:SortOrder, 7:status
	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))

	// Pass 1 — collect IDs and names for batch lookup
	var rawIDs []string
	var rawNames []string
	for _, row := range rows {
		if len(row) == 0 || allEmpty(row) {
			continue
		}
		if len(row) > 0 && row[0] != "" {
			rawIDs = append(rawIDs, row[0])
		} else if len(row) > 1 && row[1] != "" {
			rawNames = append(rawNames, row[1])
		}
	}

	byID := map[uint]dao.Unit{}
	if len(rawIDs) > 0 {
		var found []dao.Unit
		h.DB.Where("id IN ?", rawIDs).Find(&found)
		for _, u := range found {
			byID[u.ID] = u
		}
	}
	byName := map[string]dao.Unit{}
	if len(rawNames) > 0 {
		var found []dao.Unit
		h.DB.Where("name IN ?", rawNames).Find(&found)
		for _, u := range found {
			byName[u.Name] = u
		}
	}

	// Pass 2 — build create/update slices
	var creates []dao.Unit
	var updates []dao.Unit
	for _, row := range rows {
		if len(row) == 0 || allEmpty(row) {
			continue
		}
		var item dao.Unit
		isUpdate := false
		if len(row) > 0 && row[0] != "" {
			if u, ok := byID[uint(parseUint(row[0]))]; ok {
				item = u
				isUpdate = true
			}
		}
		if !isUpdate && len(row) > 1 && row[1] != "" {
			if u, ok := byName[row[1]]; ok {
				item = u
				isUpdate = true
			}
		}
		if len(row) > 1 && row[1] != "" {
			item.Name = row[1]
		}
		if len(row) > 2 && row[2] != "" {
			item.Type = new(int(parseUint(row[2])))
		}
		if len(row) > 3 {
			item.Description = row[3]
		}
		if len(row) > 4 && row[4] != "" {
			raw := strings.TrimSpace(row[4])
			if !strings.HasPrefix(raw, "[") {
				raw = "[" + raw + "]"
			}
			var ids []int
			if json.Unmarshal([]byte(raw), &ids) == nil {
				item.CategoryId = datatypes.JSON(raw)
			}
		}
		// index 5 = tên danh mục (skip)
		if len(row) > 6 && row[6] != "" {
			item.SortOrder = new(int(parseUint(strings.TrimSpace(row[6]))))
		}
		if len(row) > 7 && row[7] != "" {
			item.Status = new(int(parseUint(strings.TrimSpace(row[7]))))
		}
		item.UpdatedAt = now
		item.UpdatedBy = &uid
		if isUpdate {
			updates = append(updates, item)
		} else {
			item.CreatedAt = now
			item.CreatedBy = &uid
			creates = append(creates, item)
		}
	}

	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	if len(creates) > 0 {
		if err := tx.CreateInBatches(creates, 500).Error; err != nil {
			tx.Rollback()
			response.HandleError(c, err)
			return
		}
	}
	for _, u := range updates {
		if err := tx.Save(&u).Error; err != nil {
			tx.Rollback()
			response.HandleError(c, err)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		response.ServerError(c, err)
		return
	}
	h.invalidateUnitCache()
	c.JSON(http.StatusOK, gin.H{"message": i18n.T(c, "created")})
}

func allEmpty(row []string) bool {
	for _, v := range row {
		if v != "" {
			return false
		}
	}
	return true
}

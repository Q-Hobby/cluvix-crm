package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"errors"
	"net/http"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	serviceTypeDao "erpCluvix/internal/modules/webapp/config/service_type/dao"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/errs"
	pkgExport "erpCluvix/pkg/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/repository"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/validation"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// Deps + Handler — module-local (Plan 06). Không còn Store: cột ảnh nhóm dịch vụ đã gỡ 2026-10-19.
type Deps struct {
	DB    *gorm.DB
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

type ServiceTypeInput struct {
	// Story-39 (E6/D15): bỏ field company_id (trước `binding:"required"`) — FE cũ còn gửi thì JSON
	// thừa bị bỏ qua.
	Name      string `json:"name" form:"name" binding:"required,max=255"`
	Status    *int   `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
	SortOrder *int   `json:"SortOrder" form:"SortOrder"`
}

type ServiceTypeUpdateInput struct {
	Name      string `json:"name" form:"name" binding:"omitempty,max=255"`
	Status    *int   `json:"status" form:"status" binding:"omitempty,oneof=0 1"`
	SortOrder *int   `json:"SortOrder" form:"SortOrder"`
}

func (h *Handler) validateServiceType(input *ServiceTypeInput, excludeID uint) *validation.Errors {
	v := validation.New()
	v.Required(input.Name, "Tên nhóm dịch vụ")
	v.Unique(h.DB, "service_type", "name", input.Name, excludeID, "Tên")
	return v
}

// buildServiceTypeQuery áp dụng filter _keyword + status lên query đã có scope.
// Dùng chung cho list (Model) và export (Table) theo export.md pattern.
func buildServiceTypeQuery(c *gin.Context, q *gorm.DB) *gorm.DB {
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		q = q.Where("name LIKE ?", like)
	}
	if status := c.Query("filter[status]"); status != "" {
		q = q.Where("status = ?", status)
	}
	return q.Order("service_type.SortOrder DESC")
}

func (h *Handler) ListServiceType(c *gin.Context) {
	if c.Query("_export") == "1" {
		// Table() thay vì Model() để GORM scan map với value types (int64), không phải pointer
		eq := buildServiceTypeQuery(c,
			tenant.ScopedDBNullBranch(c, h.DB, "service_type").Table("service_type"))
		exportHandlers.ExportServiceType(c, eq)
		return
	}

	query := buildServiceTypeQuery(c,
		tenant.ScopedDBNullBranch(c, h.DB, "service_type").Model(&serviceTypeDao.ServiceType{}))
	var items []serviceTypeDao.ServiceType
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

// Plan 09 (2026-05-16): migrate sang BaseRepo[T].
// Fix kèm: `h.DB.First` cũ KHÔNG scope theo tenant — IDOR risk vì
// `service_type` là tenant table (whitelist trong pkg/tenant/scope.go).
// BaseRepo.FindByID scope qua ScopedDBNullBranch.
func (h *Handler) GetServiceType(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	repo := repository.New[serviceTypeDao.ServiceType](h.DB, "service_type")
	item, err := repo.FindByID(c, id, nil)
	if err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			response.NotFound(c)
			return
		}
		response.ServerError(c, err)
		return
	}
	response.Data(c, item)
}

func (h *Handler) CreateServiceType(c *gin.Context) {
	var input ServiceTypeInput
	if strings.Contains(c.ContentType(), "multipart") {
		if err := c.ShouldBind(&input); err != nil {
			response.ValidateErr(c, err)
			return
		}
	} else {
		if err := c.ShouldBindJSON(&input); err != nil {
			response.ValidateErr(c, err)
			return
		}
	}

	if v := h.validateServiceType(&input, 0); v.HasErrors() {
		response.Validate(c, v.Error())
		return
	}
	item := serviceTypeDao.ServiceType{
		Name:      input.Name,
		Status:    input.Status,
		SortOrder: input.SortOrder,
	}

	if err := h.DB.Set("userID", middleware.GetUserID(c)).Create(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	h.DB.Model(&item).Update("SortOrder", item.ID)
	dexie.BumpDexieTable(h.DB, "serviceTypes")
	response.Created(c, item)
}

func (h *Handler) UpdateServiceType(c *gin.Context) {
	var item serviceTypeDao.ServiceType
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "service_type").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	var input ServiceTypeUpdateInput
	if strings.Contains(c.ContentType(), "multipart") {
		if err := c.ShouldBind(&input); err != nil {
			response.ValidateErr(c, err)
			return
		}
	} else {
		if err := c.ShouldBindJSON(&input); err != nil {
			response.ValidateErr(c, err)
			return
		}
	}

	if input.Name != "" {
		v := validation.New()
		v.Unique(h.DB, "service_type", "name", input.Name, item.ID, "Tên")
		if v.HasErrors() {
			response.Validate(c, v.Error())
			return
		}
		item.Name = input.Name
	}
	if input.Status != nil {
		item.Status = input.Status
	}
	if input.SortOrder != nil {
		item.SortOrder = input.SortOrder
	}

	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "serviceTypes")
	response.Updated(c, item)
}

func (h *Handler) DeleteServiceType(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item serviceTypeDao.ServiceType
	if err := tenant.ScopedDBNullBranch(c, h.DB, "service_type").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &serviceTypeDao.ServiceType{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "serviceTypes")
	response.Deleted(c)
}

// ImportServiceTypeFromExcel POST /config/service-type/import-from-excel
// Đọc file .xlsx do user export ra, cập nhật name/status/SortOrder theo id.
// Columns nhận diện qua header name (không phụ thuộc vị trí cột).
func (h *Handler) ImportServiceTypeFromExcel(c *gin.Context) {
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

	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil || len(rows) < 2 {
		response.Validate(c, i18n.T(c, "no_data"))
		return
	}
	if len(rows)-1 > MaxImportRows {
		response.Validate(c, i18n.TF(c, "import_too_many_rows", MaxImportRows))
		return
	}

	// Map header → column index
	headerToIdx := map[string]int{}
	for j, h := range rows[0] {
		headerToIdx[strings.TrimSpace(h)] = j
	}
	idIdx, hasIDCol := headerToIdx["id"]
	if !hasIDCol {
		response.Validate(c, i18n.T(c, "no_data"))
		return
	}
	nameIdx, hasName := headerToIdx["Nhóm dịch vụ"]
	statusIdx, hasStatus := headerToIdx["Trạng thái"]
	sortIdx, hasSort := headerToIdx["Vị trí"]
	// Story-39 (D15, D23): cột company của file cũ (nếu còn) bị bỏ qua; dòng tạo mới không cần nó.

	dataRows := rows[1:]

	// Pass 1: tách rows có ID (update) và không có ID (insert)
	var ids []uint
	var insertRows [][]string
	for _, row := range dataRows {
		if allEmpty(row) {
			continue
		}
		if idIdx >= len(row) || strings.TrimSpace(row[idIdx]) == "" {
			insertRows = append(insertRows, row)
		} else {
			ids = append(ids, uint(parseUint(strings.TrimSpace(row[idIdx]))))
		}
	}
	if len(ids) == 0 && len(insertRows) == 0 {
		response.Validate(c, i18n.T(c, "no_data"))
		return
	}

	// Batch load existing records cho update
	byID := map[uint]*serviceTypeDao.ServiceType{}
	if len(ids) > 0 {
		var existing []serviceTypeDao.ServiceType
		tenant.ScopedDBNullBranch(c, h.DB, "service_type").Where("id IN ?", ids).Find(&existing)
		for i := range existing {
			byID[existing[i].ID] = &existing[i]
		}
	}

	now := time.Now().Unix()
	uid := uint(middleware.GetUserID(c))

	// Pass 2: build updates
	var updates []serviceTypeDao.ServiceType
	for _, row := range dataRows {
		if allEmpty(row) || idIdx >= len(row) || strings.TrimSpace(row[idIdx]) == "" {
			continue
		}
		id := uint(parseUint(strings.TrimSpace(row[idIdx])))
		item, ok := byID[id]
		if !ok {
			continue
		}
		if hasName && nameIdx < len(row) && strings.TrimSpace(row[nameIdx]) != "" {
			item.Name = strings.TrimSpace(row[nameIdx])
		}
		if hasStatus && statusIdx < len(row) && strings.TrimSpace(row[statusIdx]) != "" {
			s := int(parseUint(strings.TrimSpace(row[statusIdx])))
			item.Status = &s
		}
		if hasSort && sortIdx < len(row) && strings.TrimSpace(row[sortIdx]) != "" {
			s := int(parseUint(strings.TrimSpace(row[sortIdx])))
			item.SortOrder = &s
		}
		item.UpdatedAt = now
		item.UpdatedBy = &uid
		updates = append(updates, *item)
	}

	// Pass 3: build inserts (row không có ID)
	// Bước 3a: thu thập tên hợp lệ để batch check trùng
	type insertCandidate struct {
		name string
		row  []string
	}
	var candidates []insertCandidate
	insertNames := make([]string, 0, len(insertRows))
	for _, row := range insertRows {
		name := ""
		if hasName && nameIdx < len(row) {
			name = strings.TrimSpace(row[nameIdx])
		}
		if name == "" {
			continue
		}
		candidates = append(candidates, insertCandidate{name, row})
		insertNames = append(insertNames, name)
	}

	// Bước 3b: tên đã tồn tại — kiểm TOÀN DB (story-39, D15: trước theo từng company).
	existSet := make(map[string]bool)
	if len(insertNames) > 0 {
		var existing []struct {
			Name string `gorm:"column:name"`
		}
		if err := h.DB.Model(&serviceTypeDao.ServiceType{}).Select("name").
			Where("name IN ?", insertNames).Scan(&existing).Error; err != nil {
			response.ServerError(c, err)
			return
		}
		for _, e := range existing {
			existSet[e.Name] = true
		}
	}

	// Bước 3c: build inserts, skip tên đã tồn tại
	var inserts []serviceTypeDao.ServiceType
	for _, c := range candidates {
		if existSet[c.name] {
			continue
		}
		existSet[c.name] = true // trùng tên ngay trong file cũng chỉ tạo 1 bản
		defaultStatus := 1
		newItem := serviceTypeDao.ServiceType{
			Name:      c.name,
			Status:    &defaultStatus,
			CreatedBy: &uid,
			UpdatedBy: &uid,
		}
		newItem.CreatedAt = now
		newItem.UpdatedAt = now
		if hasStatus && statusIdx < len(c.row) && strings.TrimSpace(c.row[statusIdx]) != "" {
			s := int(parseUint(strings.TrimSpace(c.row[statusIdx])))
			newItem.Status = &s
		}
		if hasSort && sortIdx < len(c.row) && strings.TrimSpace(c.row[sortIdx]) != "" {
			s := int(parseUint(strings.TrimSpace(c.row[sortIdx])))
			newItem.SortOrder = &s
		}
		inserts = append(inserts, newItem)
	}

	if len(updates) == 0 && len(inserts) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": i18n.T(c, "created"), "updated": 0, "inserted": 0})
		return
	}

	tx := h.DB.Begin()
	if tx.Error != nil {
		response.ServerError(c, tx.Error)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()
	for _, u := range updates {
		if err := tx.Save(&u).Error; err != nil {
			tx.Rollback()
			response.HandleError(c, err)
			return
		}
	}
	for i := range inserts {
		if err := tx.Create(&inserts[i]).Error; err != nil {
			tx.Rollback()
			response.HandleError(c, err)
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "serviceTypes")
	c.JSON(http.StatusOK, gin.H{"message": i18n.T(c, "created"), "updated": len(updates), "inserted": len(inserts)})
}

package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	configCodeDao "erpCluvix/internal/modules/webapp/config/config_code/dao"
	"erpCluvix/pkg/dexie"
	"net/http"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/models"
	branchDao "erpCluvix/internal/modules/webapp/config/branch/dao"
	"erpCluvix/internal/modules/webapp/config/userscope"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// Deps + Handler — module-local (Plan 06).
type Deps struct {
	DB    *gorm.DB
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

// --- List ---

func (h *Handler) ListBranch(c *gin.Context) {
	// Story-39 (E6/D15): chi nhánh không còn thuộc company — bỏ lọc cột JSON companyId.
	query := tenant.ScopedDBNullBranch(c, h.DB, "branch").Model(&branchDao.Branch{})

	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		query = query.Where("name LIKE ? OR address LIKE ? OR hotline LIKE ? OR email LIKE ? OR code LIKE ?",
			like, like, like, like, like)
	}

	if status := c.Query("filter[status]"); status != "" {
		query = query.Where("status = ?", status)
	}

	query = query.Order("SortOrder DESC, id ASC")

	var items []branchDao.Branch
	result := pagination.Paginate(c, query, &items)
	c.JSON(http.StatusOK, result)
}

// --- Get ---

func (h *Handler) GetBranch(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item branchDao.Branch
	if err := tenant.ScopedDBNullBranch(c, h.DB, "branch").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

// --- Create ---

func (h *Handler) CreateBranch(c *gin.Context) {
	var item branchDao.Branch
	if err := c.ShouldBind(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	// Create: khóa chính do DB cấp — reset `id` client gửi kèm trong body/form (mass-assignment).
	item.ID = 0

	if !requireFields(c, [][2]any{{item.Name, "Tên chi nhánh"}, {item.Code, "Mã chi nhánh"}, {item.Address, "Địa chỉ"}}) {
		return
	}
	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))
	item.CreatedAt = now
	item.UpdatedAt = now
	item.CreatedBy = &uid
	item.UpdatedBy = &uid

	// Create → SortOrder → gán quyền creator → seed config_code nằm TRONG 1 transaction: lỗi bất kỳ
	// bước nào ⇒ rollback sạch, KHÔNG để chi nhánh mồ côi quyền (creator không thấy record vừa tạo)
	// hay thiếu config_code. Trước đây 4 bước chạy rời, `.Update("SortOrder")` còn bỏ qua `.Error`.
	userID := middleware.GetUserID(c)
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if item.SortOrder == nil {
			sort := int(item.ID)
			if err := tx.Model(&item).Update("SortOrder", sort).Error; err != nil {
				return err
			}
			item.SortOrder = &sort
		}
		if err := userscope.AppendID(tx, userID, userscope.ColumnBranchIDs, item.ID); err != nil {
			return err
		}
		// SeedConfigCodeForBranch tự mở `db.Transaction` bên trong — truyền `tx` ⇒ GORM dùng
		// SAVEPOINT lồng trong tx này, KHÔNG mở tx thứ hai (cùng khuôn `onboarding/service.go`).
		return SeedConfigCodeForBranch(tx, item.ID, uid)
	}); err != nil {
		response.ServerError(c, err)
		return
	}

	// Sau commit: side effect ngoài DB (cache/dexie) không được chạy khi tx còn có thể rollback.
	middleware.InvalidateUserScope(h.Cache, userID)
	dexie.BumpDexieTable(h.DB, "branches")
	response.Created(c, item)
}

// --- Update ---

func (h *Handler) UpdateBranch(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item branchDao.Branch
	if err := tenant.ScopedDBNullBranch(c, h.DB, "branch").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}

	// `Save()` phía dưới ghi theo khoá chính TRONG STRUCT — nếu client ghi đè `item.ID` từ
	// payload thì tenant scope ở bước First() bị vô hiệu (sửa sang bản ghi khác).
	// helpers.BindUpdate ghim lại `ID` sau khi bind.
	if err := helpers.BindUpdate(c, &item); err != nil {
		response.HandleError(c, err)
		return
	}

	if !requireFields(c, [][2]any{{item.Name, "Tên chi nhánh"}, {item.Code, "Mã chi nhánh"}, {item.Address, "Địa chỉ"}}) {
		return
	}

	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))
	item.UpdatedAt = now
	item.UpdatedBy = &uid

	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	if err := SeedConfigCodeForBranch(h.DB, item.ID, uid); err != nil {
		response.ServerError(c, err)
		return
	}

	dexie.BumpDexieTable(h.DB, "branches")
	response.Updated(c, item)
}

// --- Delete ---

func (h *Handler) DeleteBranch(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := models.DeleteWithCheck(h.DB, &branchDao.Branch{}, id); err != nil {
		response.HandleError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "branches")
	response.Deleted(c)
}

// --- Toggle: statusOpenAppointment ---

func (h *Handler) ToggleBranchOpenAppointment(c *gin.Context) {
	var body struct {
		ID uint `json:"id" form:"id"`
	}
	c.ShouldBind(&body)
	if body.ID == 0 {
		response.Validate(c, i18n.T(c, "id_required"))
		return
	}

	var item branchDao.Branch
	if err := h.DB.First(&item, body.ID).Error; err != nil {
		response.NotFound(c)
		return
	}

	val := 1
	msgKey := "branch_accepting_customer"
	if item.StatusOpenAppointment != nil && *item.StatusOpenAppointment == 1 {
		val = 0
		msgKey = "branch_not_accepting_customer"
	}

	h.DB.Model(&item).Update("statusOpenAppointment", val)
	response.DataWithMessage(c, item, i18n.T(c, msgKey))
}

// --- Helpers moved from user_assign + config_code_seed (Plan 06 B7) ---
// appendUserJSONID hợp nhất về `userscope.AppendID` (story-03, epic onboarding-setup-wizard) —
// hàm này trước đây nuốt lỗi Scan/Update. `assignBranchToCreator` (wrapper gọi AppendID + xoá cache)
// đã bỏ ở story-10: AppendID nay chạy TRONG transaction của CreateBranch, còn `InvalidateUserScope`
// phải chạy SAU commit — gộp 2 việc đó vào một hàm sẽ xoá cache trước khi tx chắc chắn thành công.

// SeedConfigCodeForBranch — seed config_code mặc định cho chi nhánh (idempotent). Khoá tra theo chi
// nhánh (khớp codegen.findConfig: chi nhánh → toàn hệ thống): row theo chi nhánh cho mọi bảng trừ
// `service` — 1 row DÙNG CHUNG toàn hệ thống (không ghép chi nhánh, không reset tháng), chỉ tạo khi
// CHƯA có row service nào.
func SeedConfigCodeForBranch(db *gorm.DB, branchID uint, uid int) error {
	if branchID == 0 {
		return nil
	}
	now := time.Now().Unix()

	return db.Transaction(func(tx *gorm.DB) error {
		var existing []string
		if err := tx.Table("config_code").
			Where("branch_id = ?", branchID).
			Pluck("parent_table", &existing).Error; err != nil {
			return err
		}
		have := make(map[string]struct{}, len(existing))
		for _, p := range existing {
			have[p] = struct{}{}
		}

		// service: mã dùng chung toàn hệ thống — check tồn tại KHÔNG kèm branch_id, để không tái tạo
		// row branch-scoped mỗi lần chi nhánh khác được tạo/sửa (phá consolidation).
		var hasService int64
		if err := tx.Table("config_code").
			Where("parent_table = ?", "service").
			Count(&hasService).Error; err != nil {
			return err
		}

		rows := make([]configCodeDao.ConfigCode, 0, len(defaultConfigCodeTemplates))
		for _, t := range defaultConfigCodeTemplates {
			if t.ParentTable == "service" {
				if hasService > 0 {
					continue
				}
				rows = append(rows, configCodeDao.ConfigCode{
					BranchId:          int(branchID),
					Name:              t.Name,
					ParentTable:       t.ParentTable,
					InfixCode:         t.InfixCode,
					IncludeBranchCode: false,
					MonthlyReset:      false,
					SerialLength:      3,
					CreatedAt:         now,
					UpdatedAt:         now,
					CreatedBy:         new(uid),
					UpdatedBy:         new(uid),
				})
				continue
			}
			if _, ok := have[t.ParentTable]; ok {
				continue
			}
			rows = append(rows, configCodeDao.ConfigCode{
				BranchId:    int(branchID),
				Name:        t.Name,
				ParentTable: t.ParentTable,
				InfixCode:   t.InfixCode,
				CreatedAt:   now,
				UpdatedAt:   now,
				CreatedBy:   new(uid),
				UpdatedBy:   new(uid),
			})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

type configCodeTemplate struct {
	Name        string
	ParentTable string
	InfixCode   string
}

var defaultConfigCodeTemplates = []configCodeTemplate{
	{"Khách hàng", "customer", "KH"},
	{"Đơn hàng", "order_plan", "DH"},
	{"Thanh toán", "payment", "TT"},
	{"Lịch hẹn", "appointment", "LH"},
	{"Dịch vụ", "service", "DV"},
	{"Nhắc lịch", "reminder", "RE"},
	{"Dịch vụ sử dụng", "order_service_used", "SV"},
}

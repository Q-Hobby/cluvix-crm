package handler

import (
	"gorm.io/gorm"

	"crypto/rand"
	"math/big"
	"net/http"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/config/bank_account/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
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

func (h *Handler) ListBankAccount(c *gin.Context) {
	q := tenant.ScopedDBNullBranch(c, h.DB, "bank_accounts").Model(&dao.BankAccount{})
	if keyword := c.Query("_keyword"); keyword != "" {
		like := "%" + strings.TrimSpace(keyword) + "%"
		q = q.Where("account_name LIKE ? OR account_no LIKE ? OR bank_code LIKE ?", like, like, like)
	}
	q = query.ApplyFilter(c, q, "status", "bank_accounts.status")
	q = q.Order("sort_order DESC, id ASC")
	var items []dao.BankAccount
	result := pagination.Paginate(c, q, &items)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetBankAccount(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.BankAccount
	if err := tenant.ScopedDBNullBranch(c, h.DB, "bank_accounts").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

// GetRandomBankAccount trả về 1 STK ngẫu nhiên trong danh sách active — dùng khi in bill sinh QR
func (h *Handler) GetRandomBankAccount(c *gin.Context) {
	var accounts []dao.BankAccount
	if err := tenant.ScopedDBNullBranch(c, h.DB, "bank_accounts").Where("status = 1").Find(&accounts).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	if len(accounts) == 0 {
		response.Data(c, nil)
		return
	}
	// Random load-balance across active bank accounts (crypto/rand vì gosec G404,
	// dù use case này không thực sự security-critical — chỉ chọn 1 tài khoản hiển thị FE).
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(accounts))))
	if err != nil {
		response.Data(c, accounts[0])
		return
	}
	response.Data(c, accounts[idx.Int64()])
}

func (h *Handler) CreateBankAccount(c *gin.Context) {
	var item dao.BankAccount
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	// Create: khóa chính do DB cấp — reset `id` client gửi kèm trong body/form (mass-assignment).
	item.ID = 0
	if !requireFields(c, [][2]any{{item.BankCode, "Ma ngan hang"}, {string(item.AccountNo), "So tai khoan"}, {item.AccountName, "Ten chu TK"}}) {
		return
	}
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Create(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	h.DB.Model(&item).Update("sort_order", item.ID)
	dexie.BumpDexieTable(h.DB, "__all__")
	response.Created(c, item)
}

func (h *Handler) UpdateBankAccount(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.BankAccount
	if err := tenant.ScopedDBNullBranch(c, h.DB, "bank_accounts").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if err := helpers.BindUpdate(c, &item); err != nil {
		response.HandleError(c, err)
		return
	}
	if !requireFields(c, [][2]any{{item.BankCode, "Ma ngan hang"}, {string(item.AccountNo), "So tai khoan"}, {item.AccountName, "Ten chu TK"}}) {
		return
	}
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "__all__")
	response.Updated(c, item)
}

func (h *Handler) DeleteBankAccount(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.BankAccount
	if err := tenant.ScopedDBNullBranch(c, h.DB, "bank_accounts").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "__all__")
	response.Deleted(c)
}

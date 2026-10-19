package debt

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
)

var customerDebtKeySearch = []string{
	"customer.phone", "customer.code", "customer.fullname",
	"customer.nickname", "customer.old_code",
}

const customerDebtCols = `customer.id, customer.code, customer.old_code, customer.fullname, customer.nickname, customer.phone,
	customer.branch_id, customer.totalRevenue, customer.totalPayment, customer.totalDeposit, customer.totalDebt`

type customerDebtTotals struct {
	Count        int64           `gorm:"column:cnt"`
	TotalRevenue decimal.Decimal `gorm:"column:totalRevenue"`
	TotalPayment decimal.Decimal `gorm:"column:totalPayment"`
	TotalDebts   decimal.Decimal `gorm:"column:totalDebts"`
}

// customerDebtQuery — khách đang nợ (customer.totalDebt != 0) trong phạm vi chi nhánh + bộ lọc, chưa SELECT/ORDER.
// Mỗi lần gọi trả một query MỚI (câu tổng và câu trang không dùng chung statement).
func (h *Handler) customerDebtQuery(c *gin.Context) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, h.DB, "customer").Table("customer").Where("customer.totalDebt != 0")
	q = query.ApplyFilter(c, q, "branch_id", "customer.branch_id")
	q = query.ApplyKeywordFilter(c, q, customerDebtKeySearch...)
	q = query.ApplyFilter(c, q, "created_day", "customer.created_day")
	// S-10: phạm vi telesale (khách mình phụ trách) — lỗi đọc phạm vi ⇒ query mang lỗi, không trả dòng.
	return scope.WhereCustomerID(c, h.DB, q, "customer.id")
}

func customerDebtSort(c *gin.Context, q *gorm.DB) *gorm.DB {
	return query.ApplySort(c, q, "customer.id DESC", "customer.id")
}

// CustomerDebts GET /crm/report/debt/customer — công nợ theo khách hàng (customer.totalDebt).
func (h *Handler) CustomerDebts(c *gin.Context) {
	if c.Query("_export") == "1" {
		exportHandlers.ExportCustomerDebts(c, customerDebtSort(c, h.customerDebtQuery(c)))
		return
	}

	var totals customerDebtTotals
	if err := h.customerDebtQuery(c).
		Select(`COUNT(*) AS cnt, COALESCE(SUM(customer.totalRevenue), 0) AS totalRevenue,
			COALESCE(SUM(customer.totalPayment), 0) AS totalPayment, COALESCE(SUM(customer.totalDebt), 0) AS totalDebts`).
		Scan(&totals).Error; err != nil {
		pagination.FailQuery(c, err)
		return
	}

	items := []map[string]interface{}{}
	var result pagination.Result
	if page, perPage, offset := pagination.PageParams(c); totals.Count <= int64(offset) {
		// Không khách nào nợ / trang vượt tổng ⇒ trang chắc chắn rỗng, không chạy câu trang.
		result = pagination.ResultForPage(c, &items, totals.Count, page, perPage, "customer")
	} else {
		q := customerDebtSort(c, h.customerDebtQuery(c)).Select(customerDebtCols)
		result = pagination.PaginateWithTotal(c, q, &items, totals.Count)
		if c.IsAborted() {
			return
		}
	}
	phoneutil.MaskPhoneInItems(phoneutil.CanSeePhone(c, h.DB), items)

	c.JSON(http.StatusOK, gin.H{
		"items":        items,
		"_meta":        result.Meta,
		"totalRevenue": totals.TotalRevenue,
		"totalPayment": totals.TotalPayment,
		"totalDebts":   totals.TotalDebts,
	})
}

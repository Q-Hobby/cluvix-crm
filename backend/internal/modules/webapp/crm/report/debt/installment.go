package debt

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/handlerutil"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"
)

var installmentDebtKeySearch = []string{
	"customer.fullname", "customer.phone", "customer.code", "customer.old_code", "osu.code",
}

// installmentDebtPageCols — cột câu trang. Tham số: ngày hôm nay (tz.LocalDate) cho cờ quá hạn.
// Quá hạn là điều kiện NGÀY trên kỳ còn mở, KHÔNG phải status.
const installmentDebtPageCols = `
		order_installment.id,
		order_installment.order_service_used_id,
		order_installment.period_number,
		order_installment.amount,
		COALESCE(order_installment.paid_amount, 0) AS paid_amount,
		(order_installment.amount - COALESCE(order_installment.paid_amount, 0)) AS remaining,
		DATE_FORMAT(order_installment.due_date, '%d/%m/%Y') AS due_date,
		order_installment.status,
		(order_installment.due_date < ?) AS is_overdue,
		osu.code AS service_used_code,
		osu.service_id,
		osu.branch_id,
		otp.customer_id`

type installmentDebtSummary struct {
	Total          int64           `gorm:"column:total"`
	TotalAmount    decimal.Decimal `gorm:"column:total_amount"`
	TotalPaid      decimal.Decimal `gorm:"column:total_paid"`
	TotalRemaining decimal.Decimal `gorm:"column:total_remaining"`
}

// installmentDebtQuery — kỳ trả góp CÒN MỞ trong phạm vi chi nhánh (theo dịch vụ) + bộ lọc, chưa SELECT/ORDER.
// withCustomer = LEFT JOIN customer (từ khoá / export).
func (h *Handler) installmentDebtQuery(c *gin.Context, withCustomer bool) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, h.DB, "order_service_used", "osu").
		Table("order_installment").
		Joins("JOIN order_service_used osu ON osu.id = order_installment.order_service_used_id").
		Joins("JOIN order_plan otp ON otp.id = osu.order_plan_id")
	if withCustomer {
		q = q.Joins("LEFT JOIN customer ON customer.id = otp.customer_id")
	}
	q = q.Where("order_installment.status IN ?", define.InstallmentOpenStatuses).
		// Khoá dịch vụ (isLock=2) chuyển phần còn lại sang giảm trừ ⇒ không còn là công nợ thật dù kỳ chưa đóng.
		// Tạm ngưng (isLock=3) vẫn tính.
		Where("osu.isLock != ?", define.ServiceUsedIsLock)

	if withCustomer {
		q = query.ApplyKeywordFilter(c, q, installmentDebtKeySearch...)
	}
	q = query.ApplyFilter(c, q, "branch_id", "osu.branch_id")

	// Khoảng ngày đến hạn — cột DATE, giá trị dạng YYYY-MM-DD đi qua placeholder.
	if gte, lte := c.Query("_dueDateGte"), c.Query("_dueDateLte"); gte != "" && lte != "" {
		q = q.Where("order_installment.due_date BETWEEN ? AND ?", gte, lte)
	}

	// filter[status][] — chỉ nhận tập con của InstallmentOpenStatuses (đã khoá ở WHERE gốc).
	valid := map[string]bool{}
	for _, s := range define.InstallmentOpenStatuses {
		valid[strconv.Itoa(s)] = true
	}
	var statuses []int
	for _, s := range c.QueryArray("filter[status][]") {
		if valid[s] {
			n, _ := strconv.Atoi(s)
			statuses = append(statuses, n)
		}
	}
	if len(statuses) > 0 {
		q = q.Where("order_installment.status IN ?", statuses)
	}
	// S-10: phạm vi telesale — lỗi đọc phạm vi ⇒ query mang lỗi, không trả dòng.
	return scope.WhereCustomerID(c, h.DB, q, "otp.customer_id")
}

// installmentDebtOrder — kỳ quá hạn lên đầu, sau đó theo due_date. ORDER BY dạng CHUỖI (clause.Expr có Vars bị
// GORM vứt khi ApplySort gắn thêm .Order chuỗi). `today` do server format time.DateOnly — không phải input.
func installmentDebtOrder(c *gin.Context, q *gorm.DB, today string) *gorm.DB {
	return q.Order("(order_installment.due_date < '" + today + "') DESC").
		Order(query.SortClause(c, "order_installment.due_date ASC", "order_installment.due_date", "order_installment.id"))
}

// InstallmentDebts GET /crm/report/debt/installment — công nợ trả góp (kỳ chưa thu xong).
func (h *Handler) InstallmentDebts(c *gin.Context) {
	today := tz.LocalDate(tz.LocFromContext(c, h.DB), 0)
	withCustomer := c.Query("_keyword") != ""

	if c.Query("_export") == "1" {
		exportHandlers.ExportInstallmentDebts(c, installmentDebtOrder(c, h.installmentDebtQuery(c, true), today), today)
		return
	}

	var summary installmentDebtSummary
	if err := h.installmentDebtQuery(c, withCustomer).Select(`
		COUNT(*) AS total,
		COALESCE(SUM(order_installment.amount), 0) AS total_amount,
		COALESCE(SUM(COALESCE(order_installment.paid_amount, 0)), 0) AS total_paid,
		COALESCE(SUM(order_installment.amount - COALESCE(order_installment.paid_amount, 0)), 0) AS total_remaining
	`).Scan(&summary).Error; err != nil {
		pagination.FailQuery(c, err)
		return
	}

	items := []map[string]interface{}{}
	var result pagination.Result
	if page, perPage, offset := pagination.PageParams(c); summary.Total <= int64(offset) {
		result = pagination.ResultForPage(c, &items, summary.Total, page, perPage, "order_installment")
	} else {
		q := installmentDebtOrder(c, h.installmentDebtQuery(c, withCustomer), today).Select(installmentDebtPageCols, today)
		result = pagination.PaginateWithTotal(c, q, &items, summary.Total)
		if c.IsAborted() {
			return
		}
		if err := h.attachTotalPeriods(items); err != nil {
			pagination.FailQuery(c, err)
			return
		}
	}
	for _, item := range items {
		helpers.ParseDecimalFields(item, "amount", "paid_amount", "remaining")
	}
	attachDebtCustomers(c, h.DB, items)

	c.JSON(http.StatusOK, gin.H{
		"items": result.Items,
		"_meta": result.Meta,
		"total": gin.H{
			"totalAmount":    summary.TotalAmount,
			"totalPaid":      summary.TotalPaid,
			"totalRemaining": summary.TotalRemaining,
		},
	})
}

// attachTotalPeriods gắn total_periods (số kỳ > 0 của dịch vụ) cho các dòng của trang — 1 câu trên ≤ perPage dịch vụ.
func (h *Handler) attachTotalPeriods(items []map[string]interface{}) error {
	if len(items) == 0 {
		return nil
	}
	ids := handlerutil.CollectIDs(items, "order_service_used_id")
	var rows []struct {
		OrderServiceUsedID int64 `gorm:"column:order_service_used_id"`
		TotalPeriods       int64 `gorm:"column:total_periods"`
	}
	if err := h.DB.Table("order_installment").
		Select("order_service_used_id, COUNT(*) AS total_periods").
		Where("period_number > 0 AND order_service_used_id IN ?", ids).
		Group("order_service_used_id").Scan(&rows).Error; err != nil {
		return err
	}
	byID := make(map[int64]int64, len(rows))
	for _, r := range rows {
		byID[r.OrderServiceUsedID] = r.TotalPeriods
	}
	for _, item := range items {
		item["total_periods"] = byID[helpers.ToInt64(item["order_service_used_id"])]
	}
	return nil
}

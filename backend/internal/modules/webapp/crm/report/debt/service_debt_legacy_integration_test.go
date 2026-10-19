//go:build integration

package debt

// BẢN CŨ (HEAD trước bản vá hiệu năng 2026-10-19) của GET /crm/report/debt/service — chép NGUYÊN VĂN,
// chỉ đổi tên hàm và bỏ nhánh `_export` (export đối chiếu riêng bằng tập dòng của câu SQL). Là "đáp án
// cũ" cho service_debt_parity_integration_test.go — KHÔNG sửa logic ở đây.

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
)

func (h *Handler) legacyServiceDebtQuery(c *gin.Context, withCustomer bool, from, to int64) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, h.DB, "order_service_used").Table("order_service_used")
	if withCustomer {
		q = q.Joins("LEFT JOIN customer ON customer.id = order_service_used.customer_id")
	}
	q = q.Joins("INNER JOIN service ON service.id = order_service_used.service_id").
		Where("service.serviceCategory_id IN ?", []int{define.ServiceCategoryService, define.ServiceCategoryTreatment}).
		Where(serviceDebtPredicate).
		// Khoá dịch vụ (isLock=2) đặt totalEnd = payment ⇒ predicate đã tự loại; ghi tường minh cho đúng ý định.
		Where("order_service_used.isLock != ?", define.ServiceUsedIsLock)

	q = query.ApplyFilter(c, q, "branch_id", "order_service_used.branch_id")
	q = query.ApplyFilter(c, q, "service_id", "order_service_used.service_id")
	if from > 0 && to > 0 {
		q = q.Where("order_service_used.created_day BETWEEN ? AND ?", from, to)
	}
	if withCustomer {
		q = query.ApplyKeywordFilter(c, q, serviceDebtKeySearch...)
	}
	// S-10: phạm vi telesale — lỗi đọc phạm vi ⇒ query mang lỗi, không trả dòng.
	return scope.WhereCustomerID(c, h.DB, q, "order_service_used.customer_id")
}

func (h *Handler) legacyServiceDebts(c *gin.Context) {
	from, ok1 := parseUnixParam(c, "_orderCreatedTime[gte]", 0)
	to, ok2 := parseUnixParam(c, "_orderCreatedTime[lte]", 0)
	if !ok1 || !ok2 {
		response.Validate(c, i18n.T(c, "invalid_request"))
		return
	}
	withCustomer := c.Query("_keyword") != ""

	var totals serviceDebtTotals
	if err := h.legacyServiceDebtQuery(c, withCustomer, from, to).Select(`COUNT(*) AS total,
		COALESCE(SUM(order_service_used.totalEnd), 0) AS totalEnd,
		COALESCE(SUM(order_service_used.payment), 0) AS totalPayment,
		COALESCE(SUM(order_service_used.totalEnd - order_service_used.payment), 0) AS totalDebt`).
		Scan(&totals).Error; err != nil {
		pagination.FailQuery(c, err)
		return
	}

	items := []map[string]interface{}{}
	var result pagination.Result
	if page, perPage, offset := pagination.PageParams(c); totals.Total <= int64(offset) {
		result = pagination.ResultForPage(c, &items, totals.Total, page, perPage, "order_service_used")
	} else {
		q := serviceDebtSort(c, h.legacyServiceDebtQuery(c, withCustomer, from, to)).
			Select(`order_service_used.id, order_service_used.code,
				order_service_used.service_id, order_service_used.customer_id,
				order_service_used.totalEnd, order_service_used.payment,
				order_service_used.totalEnd - order_service_used.payment AS debt,
				order_service_used.quantity, order_service_used.branch_id,
				order_service_used.created_day`)
		result = pagination.PaginateWithTotal(c, q, &items, totals.Total)
		if c.IsAborted() {
			return
		}
	}
	for _, item := range items {
		helpers.ParseDecimalFields(item, "debt")
	}
	attachDebtCustomers(c, h.DB, items)

	// totalDeposit — tổng cọc của các KHÁCH (mỗi khách 1 lần) có dịch vụ còn nợ theo ĐÚNG bộ lọc danh sách.
	var totalDeposit decimal.Decimal
	custIDs := h.legacyServiceDebtQuery(c, withCustomer, from, to).Select("order_service_used.customer_id")
	if err := h.DB.Raw("SELECT COALESCE(SUM(dep.totalDeposit), 0) FROM customer AS dep WHERE dep.id IN (?)", custIDs).
		Scan(&totalDeposit).Error; err != nil {
		pagination.FailQuery(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": result.Items,
		"_meta": result.Meta,
		"total": gin.H{
			"totalEnd":     totals.TotalEnd,
			"totalPayment": totals.TotalPayment,
			"totalDebt":    totals.TotalDebt,
			"totalDeposit": totalDeposit,
		},
	})
}

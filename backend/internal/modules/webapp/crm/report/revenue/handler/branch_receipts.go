package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
)

// branchReceiptsSelect — cột nạp cho ĐÚNG ≤ perPage id của trang (deferred join).
const branchReceiptsSelect = `payment.id, payment.code,
	payment.customer_id, payment.serviceUsed_id,
	payment.branch_id,
	payment.method, payment.card_type_id,
	payment.amount, payment.payment_for,
	payment.payment_time, payment.created_at,
	customer.code AS customer_code,
	customer.fullname AS customer_fullname,
	customer.nickname AS customer_nickname,
	customer.address AS customer_address,
	ose.code AS ose_code,
	ose.service_id AS ose_service_id,
	ose.quantity AS ose_quantity`

const (
	branchJoinCustomer = "LEFT JOIN customer ON customer.id = payment.customer_id"
	branchJoinOSE      = "LEFT JOIN order_service_used ose ON ose.id = payment.serviceUsed_id"
	// Chỉ mục dẫn đầu payment_time (migration create_payment). Hint câu tổng khi toàn chi nhánh —
	// số đo ERP /check-scale 2026-09-17 @600.000 phiếu: 1 năm 643–721 → FORCE 86–99ms; 1 chi nhánh
	// optimizer tự chọn fk-payment-branch_id tốt hơn ⇒ không hint.
	branchPaymentIndex = "idx_payment_time_cover"
)

// branchReceiptsKeywordCols — `_keyword` khớp LIKE trên các cột này (CRM không có customer.search_text
// / ngram như ERP ⇒ LIKE thô, cần JOIN customer).
var branchReceiptsKeywordCols = []string{"customer.fullname", "customer.code", "customer.phone", "payment.code"}

var branchReceiptsPaymentFor = []int{
	define.PaymentForOrder, define.PaymentForDeposit, define.PaymentForRefundDeposit,
	define.PaymentForRefundService, define.PaymentForClinicIncome, define.PaymentForClinicExpense,
}

// branchReceiptsFilters — bộ lọc chuẩn + điều kiện cố định của màn (chỉ chạm cột `payment`).
// S-10: kèm phạm vi telesale (`payment.customer_id IN` khách mình phụ trách) — dùng chung cho câu
// tổng, câu tra id và export; lỗi đọc phạm vi ⇒ query mang lỗi, không trả dòng.
func branchReceiptsFilters(c *gin.Context, db *gorm.DB, q *gorm.DB) *gorm.DB {
	q = scope.WhereCustomerID(c, db, q, "payment.customer_id")
	q = query.ApplyFilter(c, q, "branch_id", "payment.branch_id")
	q = query.ApplyFilter(c, q, "method", "payment.method")
	q = query.ApplyFilter(c, q, "card_type_id", "payment.card_type_id")
	q = query.ApplyFilter(c, q, "payment_for", "payment.payment_for")
	q = query.ApplyFilter(c, q, "payment_time", "payment.payment_time")
	q = query.ApplyFilter(c, q, "created_at", "payment.created_at")
	return q.Where("payment.amount != 0").
		Where("payment.payment_for IN ?", branchReceiptsPaymentFor).
		Where("(payment.deposit_type IS NULL OR payment.deposit_type NOT IN ?)", []int{define.PaymentTypeDepositKG})
}

func branchSortClause(c *gin.Context) string {
	return query.SortClause(c, "payment.id DESC", "payment.id", "payment.created_at", "payment.payment_time")
}

// branchScoped — truy vấn đã thu về đúng 1 chi nhánh (phiên đang ở 1 chi nhánh hoặc lọc đúng 1).
func branchScoped(c *gin.Context) bool {
	return middleware.GetBranchActiveID(c) > 0 || len(query.GetFilterValues(c, "branch_id")) == 1
}

func hasKeyword(c *gin.Context) bool {
	return strings.TrimSpace(c.Query("_keyword")) != ""
}

// branchBase — FROM payment (+ hint) + scope chi nhánh của phiên + từ khoá + bộ lọc.
// JOIN customer CHỈ khi có từ khoá (bộ lọc còn lại chỉ chạm cột payment) ⇒ câu tổng/câu tra id rẻ.
func (h *Handler) branchBase(c *gin.Context, table string) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, h.DB, "payment").Table(table)
	if hasKeyword(c) {
		q = query.ApplyKeywordFilter(c, q.Joins(branchJoinCustomer), branchReceiptsKeywordCols...)
	}
	return branchReceiptsFilters(c, h.DB, q)
}

func branchTable(hint bool) string {
	if hint {
		return "payment FORCE INDEX (" + branchPaymentIndex + ")"
	}
	return "payment"
}

type branchSummary struct {
	Total    int64           `gorm:"column:total"`
	TotalSum decimal.Decimal `gorm:"column:total_sum"`
}

func (h *Handler) branchSummary(c *gin.Context) (branchSummary, error) {
	var s branchSummary
	err := h.branchBase(c, branchTable(!branchScoped(c))).
		Select("COUNT(*) AS total, COALESCE(SUM(payment.amount), 0) AS total_sum").
		Scan(&s).Error
	return s, err
}

// ListBranchReceipts GET /crm/report/revenue/branch-receipts — phiếu thu/chi theo chi nhánh.
// `_export=1` ⇒ xuất Excel (export.ExportPaymentBranch, nhiều sheet theo chi nhánh), cùng bộ lọc.
// Bắt buộc `filter[payment_time][gte]/[lte]` + trần 1 năm.
func (h *Handler) ListBranchReceipts(c *gin.Context) {
	if _, _, ok := requireDateRange(c, "filter[payment_time][gte]", "filter[payment_time][lte]"); !ok {
		return
	}

	if c.Query("_export") == "1" {
		// ExportPaymentBranch select cột customer.* + ose.* ⇒ query export LUÔN mang đủ 2 JOIN.
		eq := tenant.ScopedDBNullBranch(c, h.DB, "payment").Table("payment").
			Joins(branchJoinCustomer).Joins(branchJoinOSE)
		eq = query.ApplyKeywordFilter(c, eq, branchReceiptsKeywordCols...)
		eq = branchReceiptsFilters(c, h.DB, eq).Order(branchSortClause(c))
		exportHandlers.ExportPaymentBranch(c, eq, func() (int64, error) {
			s, err := h.branchSummary(c)
			return s.Total, err
		})
		return
	}

	summary, err := h.branchSummary(c)
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}

	page, perPage, offset := pagination.PageParams(c)
	sortClause := branchSortClause(c)
	items := []map[string]interface{}{}
	if summary.Total > int64(offset) {
		// Tra id trước trên riêng payment rồi mới JOIN ≤ perPage dòng (COUNT không bọc câu có JOIN).
		var pageIDs []int64
		if tx := h.branchBase(c, "payment").Order(sortClause).Offset(offset).Limit(perPage).
			Pluck("payment.id", &pageIDs); pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		if len(pageIDs) > 0 {
			if tx := h.DB.Table("payment").Joins(branchJoinCustomer).Joins(branchJoinOSE).
				Select(branchReceiptsSelect).Where("payment.id IN ?", pageIDs).
				Order(sortClause).Find(&items); pagination.IsQueryError(tx.Error) {
				pagination.FailQuery(c, tx.Error)
				return
			}
		}
	}
	result := pagination.ResultForPage(c, &items, summary.Total, page, perPage, "payment", "customer", "order_service_used")

	for _, item := range items {
		item["customer"] = map[string]interface{}{
			"id":       item["customer_id"],
			"code":     item["customer_code"],
			"fullname": item["customer_fullname"],
			"nickname": item["customer_nickname"],
			"address":  item["customer_address"],
		}
		item["orderServiceUsed"] = map[string]interface{}{
			"id":         item["serviceUsed_id"],
			"code":       item["ose_code"],
			"service_id": item["ose_service_id"],
			"quantity":   item["ose_quantity"],
		}
		for _, k := range []string{
			"customer_code", "customer_fullname", "customer_nickname", "customer_address",
			"ose_code", "ose_service_id", "ose_quantity",
		} {
			delete(item, k)
		}
	}

	c.JSON(http.StatusOK, gin.H{"object": result, "totalSum": summary.TotalSum})
}

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/handlerutil"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/tenant"
)

// refundListSelect — cột phiếu mà màn Giảm trừ doanh số hiển thị (khách/phiếu cọc gắn sau).
const refundListSelect = `payment.id, payment.code,
	payment.customer_id, payment.deposit_id, payment.branch_id,
	payment.amount, payment.payment_for, payment.payment_time, payment.note,
	payment.created_at, payment.updated_at, payment.created_by, payment.updated_by`

const (
	refundJoinCustomer  = "LEFT JOIN customer ON customer.id = payment.customer_id"
	refundJoinOrderPlan = "LEFT JOIN order_plan ON order_plan.id = payment.parent_id"
)

// refundKeywordCols — `_keyword` khớp LIKE (CRM không có customer.search_text như ERP).
var refundKeywordCols = []string{"customer.fullname", "customer.code", "customer.phone", "order_plan.code", "payment.code"}

var refundPaymentFor = []int{define.PaymentForRefundDeposit, define.PaymentForRefundService}

// refundBase — payment + scope chi nhánh phiên + từ khoá + bộ lọc + điều kiện cố định, CHƯA sort.
// withCustomer=true ⇒ luôn JOIN customer (export select cột customer.*); còn lại JOIN chỉ khi có từ khoá.
func (h *Handler) refundBase(c *gin.Context, withCustomer bool) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, h.DB, "payment").Table("payment")
	kw := hasKeyword(c)
	if kw || withCustomer {
		q = q.Joins(refundJoinCustomer)
	}
	if kw {
		q = query.ApplyKeywordFilter(c, q.Joins(refundJoinOrderPlan), refundKeywordCols...)
	}
	q = query.ApplyFilter(c, q, "branch_id", "payment.branch_id")
	q = query.ApplyFilter(c, q, "customer_id", "payment.customer_id")
	q = query.ApplyFilter(c, q, "payment_for", "payment.payment_for")
	q = query.ApplyFilter(c, q, "payment_time", "payment.payment_time")
	q = query.ApplyFilter(c, q, "created_by", "payment.created_by")
	// S-10: phạm vi telesale — lỗi đọc phạm vi ⇒ query mang lỗi, không trả dòng.
	q = scope.WhereCustomerID(c, h.DB, q, "payment.customer_id")
	return q.Where("payment.amount != 0").Where("payment.payment_for IN ?", refundPaymentFor)
}

func refundSortClause(c *gin.Context) string {
	return query.SortClause(c, "payment.id DESC", "payment.id", "payment.payment_time")
}

// ListRefundDepositService GET /crm/report/revenue/refund-deposit-service — Giảm trừ doanh số:
// phiếu hoàn dịch vụ (payment_for=1) + hoàn cọc (payment_for=2). `_export=1` ⇒ xuất Excel cùng bộ lọc.
// Bắt buộc `filter[payment_time][gte]/[lte]` + trần 1 năm.
func (h *Handler) ListRefundDepositService(c *gin.Context) {
	if _, _, ok := requireDateRange(c, "filter[payment_time][gte]", "filter[payment_time][lte]"); !ok {
		return
	}

	if c.Query("_export") == "1" {
		exportHandlers.ExportRefundDepositService(c, h.refundBase(c, true).Order(refundSortClause(c)))
		return
	}

	// COUNT + SUM một lượt, không ORDER BY, chỉ JOIN khi từ khoá cần.
	var summary struct {
		Total    int64           `gorm:"column:total"`
		TotalSum decimal.Decimal `gorm:"column:total_sum"`
	}
	if tx := h.refundBase(c, false).
		Select("COUNT(*) AS total, COALESCE(SUM(payment.amount), 0) AS total_sum").
		Scan(&summary); pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	items := []map[string]interface{}{}
	result := pagination.PaginateWithTotal(c,
		h.refundBase(c, false).Select(refundListSelect).Order(refundSortClause(c)), &items, summary.Total)
	if c.Writer.Written() {
		return // Paginate đã trả 500 khi lỗi SQL
	}

	if !h.attachRefundRelations(c, items) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"object": result, "totalRefundDeposit": summary.TotalSum})
}

// attachRefundRelations gắn `customer` + `deposit` (phiếu cọc gốc) cho từng dòng.
// FK lookup CỐ Ý không lọc chi nhánh (khách/phiếu cọc có thể thuộc chi nhánh khác) — CrossBranchLookup.
// Trả false nếu đã ghi response lỗi.
func (h *Handler) attachRefundRelations(c *gin.Context, items []map[string]interface{}) bool {
	if len(items) == 0 {
		return true
	}
	if ids := handlerutil.CollectIDs(items, "customer_id"); len(ids) > 0 {
		var rows []map[string]interface{}
		if tx := tenant.CrossBranchLookup(h.DB, "customer").Table("customer").
			Select("id, code, old_code, fullname, nickname, phone, avatar").
			Where("id IN ?", ids).Find(&rows); pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return false
		}
		storage.ResolveRows(rows, "avatar")
		showPhone := phoneutil.CanSeePhone(c, h.DB)
		byID := make(map[int64]map[string]interface{}, len(rows))
		for _, r := range rows {
			if !showPhone {
				if p, ok := r["phone"].(string); ok {
					r["phone"] = phoneutil.MaskPhone(p)
				}
			}
			byID[helpers.ToInt64(r["id"])] = r
		}
		for _, item := range items {
			item["customer"] = byID[helpers.ToInt64(item["customer_id"])]
		}
	}
	if ids := handlerutil.CollectIDs(items, "deposit_id"); len(ids) > 0 {
		var rows []map[string]interface{}
		if tx := tenant.CrossBranchLookup(h.DB, "payment").Table("payment").
			Select("id, code, note").
			Where("id IN ?", ids).Find(&rows); pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return false
		}
		byID := make(map[int64]map[string]interface{}, len(rows))
		for _, r := range rows {
			byID[helpers.ToInt64(r["id"])] = r
		}
		for _, item := range items {
			item["deposit"] = byID[helpers.ToInt64(item["deposit_id"])]
		}
	}
	return true
}

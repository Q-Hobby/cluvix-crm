package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// PrintReceptionPayment GET /api/print/reception-payment
// In phiếu thu lễ tân (danh sách payment theo filter).
// Query params: filter[payment_time][gte], filter[payment_time][lte], filter[branch_id], filter[payment_for]
func (h *Handler) PrintReceptionPayment(c *gin.Context) {
	q := tenant.ScopedDBNullBranch(c, h.DB, "payment", "p").Table("payment p")

	// Apply filters
	q = query.ApplyFilter(c, q, "payment_time", "p.payment_time")
	q = query.ApplyFilter(c, q, "branch_id", "p.branch_id")
	q = query.ApplyFilter(c, q, "payment_for", "p.payment_for")

	// Lấy tất cả data (không phân trang)
	// method/note là tên cột thật của bảng `payment` (KHÔNG phải payment_method/description — 2 tên
	// đó không tồn tại, gây MySQL 1054 khiến endpoint hỏng ở mọi môi trường trước khi vá). FE
	// (`crm/payment.service.ts` addLabels, `print-reception-payment.html`) đã đọc đúng theo tên thật
	// `method`/`note` — giữ nguyên tên cột, KHÔNG alias.
	var items []map[string]interface{}
	q = q.Select(`p.id, p.code, p.customer_id, p.payment_for, p.amount, p.payment_time,
		p.method, p.note, p.branch_id, p.created_by, p.created_at`)
	if err := q.Limit(2000).Find(&items).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	// Attach customer (scoped tenant)
	if len(items) > 0 {
		customerIds := collectIDs(items, "customer_id")
		if len(customerIds) > 0 {
			h.attachRelation(c, items, "customer_id", "customer",
				"customer", "id", "id, code, fullname, nickname, phone", customerIds)
		}
	}

	// Tính totalAmount
	totalAmount := decimal.Zero
	for _, item := range items {
		totalAmount = totalAmount.Add(helpers.ToDecimal(item["amount"]))
	}

	helpers.ParseDecimalFieldsForTables(items, "payment")
	helpers.ParseJSONFields(items)

	c.JSON(http.StatusOK, gin.H{
		"object": gin.H{
			"items": items,
		},
		"totalAmount": totalAmount,
	})
}

// collectIDs — extract unique int64 values from a field
func collectIDs(items []map[string]interface{}, field string) []int64 {
	seen := make(map[int64]bool)
	var result []int64
	for _, item := range items {
		id := helpers.ToInt64(item[field])
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// attachRelation — load nested object and attach to parent (scoped tenant).
//
// story-33: KHÔNG gộp fork này về handlerutil.AttachRelationScoped dù cùng lớp fork
// attachRelation* (scope ScopedDBNullBranch). Lý do: giữa lúc story-33 chạy, 1 phiên khác đã thêm
// check `pagination.IsQueryError`/`FailQuery` vào ĐÚNG hàm này (swallowed-sql-errors-sweep epic,
// F-G) — hành vi đó KHÔNG có ở `handlerutil.AttachRelationScoped`. Gộp bằng cách thay call site sẽ
// làm MẤT phần check lỗi mới thêm (uncommitted work của phiên khác) — vi phạm AC5 (không đổi hành
// vi) theo hướng tệ hơn giữ nguyên fork. Giữ fork, ghi rõ lý do theo AC4.
func (h *Handler) attachRelation(c *gin.Context, items []map[string]interface{}, foreignKey, targetKey, table, pkField, selectCols string, ids []int64) {
	var rows []map[string]interface{}
	tx := tenant.ScopedDBNullBranch(c, h.DB, table).Table(table).
		Select(selectCols).Where(pkField+" IN ?", ids).Find(&rows)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	rowMap := make(map[int64]map[string]interface{})
	for _, r := range rows {
		rowMap[helpers.ToInt64(r[pkField])] = r
	}
	for _, item := range items {
		item[targetKey] = rowMap[helpers.ToInt64(item[foreignKey])]
	}
}

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
)

type paymentByMethodRow struct {
	MethodID   int             `gorm:"column:method_id"   json:"method_id"`
	MethodName string          `gorm:"column:method_name" json:"method_name"`
	PaymentFor int             `gorm:"column:payment_for" json:"payment_for"`
	Amount     decimal.Decimal `gorm:"column:amount"      json:"amount"`
}

// ReportPaymentByMethod GET /crm/report/revenue/by-method?from=&to=
//
// Trả flat rows (hình thức × loại phiếu → tổng tiền, phiếu hoàn/chi mang dấu ÂM); FE pivot thành bảng
// chéo. Dòng chuyển cọc (payment_for=5) là bút toán nội bộ cọc → dịch vụ, KHÔNG phải tiền mới: vẫn trả
// về để hiện cột riêng, nhưng FE không cộng vào cột Tổng (tiền đó đã tính một lần lúc nhận cọc).
// Ngày tính theo `payment_of_date`, phiếu chưa có thì lấy `payment_time` — viết dạng OR 2 vế thay
// cho `COALESCE(...) BETWEEN` để dùng được chỉ mục (bọc cột trong hàm là quét trọn bảng).
// Bắt buộc from/to + trần 1 năm (ERP mặc định tháng này khi thiếu — CRM bắt buộc như mọi báo cáo).
// Phạm vi: chi nhánh của phiên (ScopedDBNullBranch) + bộ lọc branch_id người dùng chọn.
func (h *Handler) ReportPaymentByMethod(c *gin.Context) {
	from, to, ok := requireDateRange(c, "from", "to")
	if !ok {
		return
	}

	q := tenant.ScopedDBNullBranch(c, h.DB, "payment", "p").
		Table("payment p").
		Joins("LEFT JOIN payment_method pm ON pm.id = p.method").
		Select(`p.method AS method_id,
			COALESCE(pm.name, '') AS method_name,
			COALESCE(p.payment_for, 0) AS payment_for,
			COALESCE(SUM(p.amount), 0) AS amount`).
		Where("((p.payment_of_date BETWEEN ? AND ?) OR (p.payment_of_date IS NULL AND p.payment_time BETWEEN ? AND ?))",
			from, to, from, to)
	q = query.ApplyFilter(c, q, "branch_id", "p.branch_id")
	// lens-money M-12: giữ cả nhóm ÂM (hoàn dịch vụ / hoàn cọc / hoàn ký gửi / chi phòng khám lưu số âm)
	// để tổng ròng đúng — bản cũ `HAVING SUM > 0` bỏ sạch mọi khoản chi ra. Chỉ bỏ nhóm triệt tiêu về 0.
	q = q.Group("p.method, pm.name, p.payment_for").
		Having("COALESCE(SUM(p.amount), 0) <> 0").
		Order("method_name, payment_for")

	rows := []paymentByMethodRow{}
	if tx := q.Scan(&rows); pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	c.JSON(http.StatusOK, rows)
}

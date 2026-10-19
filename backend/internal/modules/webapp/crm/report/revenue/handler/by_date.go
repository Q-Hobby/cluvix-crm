package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
)

// paymentByDateAliases — alias SUM/biểu thức của câu PaymentByDateSQL. MySQL trả DECIMAL ⇒ map-scan
// ra string; alias không thuộc bảng nào nên phải liệt kê tường minh cho ParseDecimalFields.
// `dayCreate` (FLOOR(…/86400)…) cũng là DECIMAL dù không phải tiền — parse cùng để FE nhận number.
var paymentByDateAliases = []string{
	"numAppOk", "totalDoanhSo", "totalThucThu", "totalDeposit",
	"totalRefundDepoit", "totalRefundService", "dayCreate",
}

// ReportPaymentByDate GET /crm/report/revenue/by-date
//
// Doanh thu theo ngày thu tiền: mỗi dòng 1 ngày (theo timezone thương hiệu) gồm số khách đồng ý,
// doanh số, thực thu, đặt cọc, hoàn cọc, hoàn dịch vụ. Bắt buộc `_ngay_thang[gte]/[lte]` + trần 1 năm.
//
// Câu SQL dùng CHUNG với export (export.PaymentByDateSQL) — chạy MỘT lần, dòng tổng + phân trang làm
// trong Go (kết quả 1 dòng/ngày ⇒ ≤ 367 dòng với trần 1 năm).
func (h *Handler) ReportPaymentByDate(c *gin.Context) {
	if _, _, ok := requireDateRange(c, "_ngay_thang[gte]", "_ngay_thang[lte]"); !ok {
		return
	}

	sortedSQL, args := exportHandlers.PaymentByDateSQL(c, h.DB)
	rows := make([]map[string]interface{}, 0)
	if tx := h.DB.Raw(sortedSQL, args...).Scan(&rows); pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	var numAppOk int64
	var totalDoanhSo, totalThucThu, totalDeposit, totalRefundDeposit, totalRefundService decimal.Decimal
	for _, r := range rows {
		helpers.ParseDecimalFields(r, paymentByDateAliases...)
		numAppOk += helpers.ToInt64(r["numAppOk"])
		totalDoanhSo = totalDoanhSo.Add(helpers.ToDecimal(r["totalDoanhSo"]))
		totalThucThu = totalThucThu.Add(helpers.ToDecimal(r["totalThucThu"]))
		totalDeposit = totalDeposit.Add(helpers.ToDecimal(r["totalDeposit"]))
		totalRefundDeposit = totalRefundDeposit.Add(helpers.ToDecimal(r["totalRefundDepoit"]))
		totalRefundService = totalRefundService.Add(helpers.ToDecimal(r["totalRefundService"]))
	}

	page, perPage, offset := pagination.PageParams(c)
	start := min(offset, len(rows))
	end := min(start+perPage, len(rows))
	items := rows[start:end]
	result := pagination.ResultForPage(c, &items, int64(len(rows)), page, perPage)

	c.JSON(http.StatusOK, gin.H{
		"items": result.Items,
		"_meta": result.Meta,
		"summary": gin.H{
			"numAppOk":           numAppOk,
			"totalDoanhSo":       totalDoanhSo,
			"totalThucThu":       totalThucThu,
			"totalDeposit":       totalDeposit,
			"totalRefundDepoit":  totalRefundDeposit,
			"totalRefundService": totalRefundService,
		},
	})
}

// ReportPaymentByDateExport GET /crm/report/revenue/by-date/export — cùng guard dải ngày với list.
func (h *Handler) ReportPaymentByDateExport(c *gin.Context) {
	if _, _, ok := requireDateRange(c, "_ngay_thang[gte]", "_ngay_thang[lte]"); !ok {
		return
	}
	exportHandlers.ExportPaymentByDate(c, h.DB)
}

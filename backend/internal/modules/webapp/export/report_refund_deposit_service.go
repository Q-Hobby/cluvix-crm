package export

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tz"
)

// refundTypeExportLabel — nhãn "Loại giảm trừ" của báo cáo Giảm trừ doanh số (chỉ 2 loại phiếu
// mà báo cáo này lấy: hoàn dịch vụ / hoàn cọc).
func refundTypeExportLabel(v interface{}) string {
	switch int(toFloat(v)) {
	case define.PaymentForRefundService:
		return "Hoàn dịch vụ"
	case define.PaymentForRefundDeposit:
		return "Hoàn cọc"
	}
	return ""
}

// refundDepositServiceColsFor — cột export của báo cáo Giảm trừ doanh số, key khớp 1-1 với
// FE `crm/report/revenue/refund-deposit-service` displayCols[].value (chọn cột qua ?columns=).
// showPhone (option ẩn SĐT) mask số điện thoại như màn hình.
func refundDepositServiceColsFor(loc *time.Location, showPhone bool) []paymentGenCol {
	return []paymentGenCol{
		{"id", "ID khách hàng", func(r map[string]interface{}) interface{} { return r["customer_id"] }},
		{"customer_number", "Mã khách hàng", func(r map[string]interface{}) interface{} { return r["customer_code"] }},
		{"customer_old_code", "Mã khách cũ", func(r map[string]interface{}) interface{} { return r["customer_old_code"] }},
		{"customer_id", "Khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }},
		{"phone", "Số điện thoại", func(r map[string]interface{}) interface{} { return maskPhoneExport(showPhone, r["customer_phone"]) }},
		{"code", "Mã phiếu giảm trừ", func(r map[string]interface{}) interface{} { return r["code"] }},
		{"deposit", "Số tiền giảm trừ", func(r map[string]interface{}) interface{} { return toFloat(r["amount"]) }},
		{"refundType", "Loại giảm trừ", func(r map[string]interface{}) interface{} { return refundTypeExportLabel(r["payment_for"]) }},
		{"content", "Nội dung giảm trừ", func(r map[string]interface{}) interface{} { return stripHTML(r["note"]) }},
		{"codeDeposit", "Mã phiếu cọc", func(r map[string]interface{}) interface{} { return r["deposit_code"] }},
		{"contentDeposit", "Nội dung phiếu cọc", func(r map[string]interface{}) interface{} { return stripHTML(r["deposit_note"]) }},
		{"payment_time", "Ngày giảm trừ", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["payment_time"], loc) }},
		{"branch_id", "Chi nhánh", func(r map[string]interface{}) interface{} { return r["branch_name"] }},
		{"created_by", "Người tạo", func(r map[string]interface{}) interface{} { return r["user_created_fullname"] }},
		{"created_at", "Ngày tạo", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["created_at"], loc) }},
		{"updated_by", "Người cập nhật", func(r map[string]interface{}) interface{} { return r["user_updated_fullname"] }},
		{"updated_at", "Ngày cập nhật", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["updated_at"], loc) }},
	}
}

// ExportRefundDepositService xuất báo cáo Giảm trừ doanh số (phiếu hoàn dịch vụ + hoàn cọc).
// q phải đã có `LEFT JOIN customer ON customer.id = payment.customer_id` + bộ lọc/sort của màn list.
func ExportRefundDepositService(c *gin.Context, q *gorm.DB) {
	loc := tz.LocFromContext(c, q.Session(&gorm.Session{NewDB: true}))
	eq := q.Session(&gorm.Session{}).
		Select(`payment.id, payment.code, payment.amount, payment.note, payment.payment_time,
			payment.payment_for, payment.created_at, payment.updated_at, payment.customer_id,
			customer.code as customer_code, customer.old_code as customer_old_code,
			customer.fullname as customer_fullname, customer.phone as customer_phone,
			dep.code as deposit_code, dep.note as deposit_note,
			br.name as branch_name,
			uc.fullname as user_created_fullname, uu.fullname as user_updated_fullname`).
		Joins("LEFT JOIN payment dep ON dep.id = payment.deposit_id").
		Joins("LEFT JOIN branch br ON br.id = payment.branch_id").
		Joins("LEFT JOIN user_profile uc ON uc.user_id = payment.created_by").
		Joins("LEFT JOIN user_profile uu ON uu.user_id = payment.updated_by")

	showPhone := phoneutil.CanSeePhoneExport(c, q.Session(&gorm.Session{NewDB: true}))
	allCols := refundDepositServiceColsFor(loc, showPhone)
	cols := allCols
	if colParam := c.Query("columns"); colParam != "" {
		selected := map[string]bool{}
		for _, k := range strings.Split(colParam, ",") {
			selected[strings.TrimSpace(k)] = true
		}
		var filtered []paymentGenCol
		for _, col := range allCols {
			if selected[col.key] {
				filtered = append(filtered, col)
			}
		}
		if len(filtered) > 0 {
			cols = filtered
		}
	}

	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = col.header
	}

	url, err := doExport(c, eq, export.Config{
		FileName:    "Giam_tru_doanh_so",
		Headers:     headers,
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		data := make([]interface{}, len(cols))
		for i, col := range cols {
			data[i] = col.extract(row)
		}
		return data
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

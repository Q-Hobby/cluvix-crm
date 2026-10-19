package export

// Export báo cáo công nợ CRM (crm/report/debt) — port từ ERP export/installment.go (trả góp) và
// ExportTreatmentDebts (công nợ điều trị → "công nợ theo dịch vụ", bỏ cột buổi điều trị/% hoàn thành/thương hiệu).

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tz"
)

// installmentDebtExportCols — key khớp FE crm/report/debt/installment-debts displayCols[].value.
// customer_old_code là cột BE-only (chỉ có khi export "all").
func installmentDebtExportCols(showPhone bool) []exportCol {
	return []exportCol{
		{"id", "ID", func(r map[string]interface{}) interface{} { return r["id"] }},
		{"customer_number", "Mã khách hàng", func(r map[string]interface{}) interface{} { return r["customer_code"] }},
		{"customer_old_code", "Mã khách cũ", func(r map[string]interface{}) interface{} { return r["customer_old_code"] }},
		{"customer_id", "Tên khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }},
		{"phone", "Điện thoại", func(r map[string]interface{}) interface{} { return maskPhoneExport(showPhone, r["customer_phone"]) }},
		{"service_used_code", "Mã dịch vụ sử dụng", func(r map[string]interface{}) interface{} { return r["service_used_code"] }},
		{"service_id", "Dịch vụ", func(r map[string]interface{}) interface{} { return r["service_name"] }},
		{"period_number", "Kỳ", func(r map[string]interface{}) interface{} {
			return fmt.Sprintf("%v/%v", r["period_number"], r["total_periods"])
		}},
		{"due_date", "Ngày đến hạn", func(r map[string]interface{}) interface{} { return r["due_date"] }},
		{"amount", "Số tiền", func(r map[string]interface{}) interface{} { return toFloat(r["amount"]) }},
		{"paid_amount", "Đã thanh toán", func(r map[string]interface{}) interface{} { return toFloat(r["paid_amount"]) }},
		{"remaining", "Còn lại", func(r map[string]interface{}) interface{} { return toFloat(r["remaining"]) }},
		{"status", "Trạng thái", func(r map[string]interface{}) interface{} {
			return installmentDebtStatusLabel(r["status"], r["is_overdue"])
		}},
		{"branch_id", "Chi nhánh", func(r map[string]interface{}) interface{} { return r["branch_name"] }},
	}
}

// ExportInstallmentDebts — q đã có FROM order_installment + JOIN osu/otp/customer + WHERE + ORDER của handler.
// `today` = tz.LocalDate của handler để cờ quá hạn trong file khớp màn hình.
func ExportInstallmentDebts(c *gin.Context, q *gorm.DB, today string) {
	showPhone := phoneutil.CanSeePhoneExport(c, q.Session(&gorm.Session{NewDB: true}))
	eq := q.Session(&gorm.Session{}).
		Select(`
			order_installment.id,
			customer.code AS customer_code,
			customer.old_code AS customer_old_code,
			customer.fullname AS customer_fullname,
			customer.phone AS customer_phone,
			osu.code AS service_used_code,
			svc.name AS service_name,
			order_installment.period_number,
			(SELECT NULLIF(COUNT(*), 0) FROM order_installment tpi
				WHERE tpi.order_service_used_id = osu.id AND tpi.period_number > 0) AS total_periods,
			DATE_FORMAT(order_installment.due_date, '%d/%m/%Y') AS due_date,
			order_installment.amount,
			COALESCE(order_installment.paid_amount, 0) AS paid_amount,
			(order_installment.amount - COALESCE(order_installment.paid_amount, 0)) AS remaining,
			order_installment.status,
			(order_installment.due_date < ?) AS is_overdue,
			br.name AS branch_name`, today).
		Joins("LEFT JOIN service svc ON svc.id = osu.service_id").
		Joins("LEFT JOIN branch br ON br.id = osu.branch_id")

	// amount/paid_amount/remaining render cứng trên màn (không isColVisible) — luôn giữ trong export.
	cols := filterExportCols(installmentDebtExportCols(showPhone), c.Query("columns"), "amount", "paid_amount", "remaining")
	headers, rowFn := exportHeadersRow(cols)
	url, err := doExport(c, eq, export.Config{
		FileName:    "Cong_no_tra_gop",
		Headers:     headers,
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, rowFn)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

// installmentDebtStatusLabel — quá hạn (điều kiện NGÀY) ưu tiên trước status.
func installmentDebtStatusLabel(status, isOverdue interface{}) string {
	if fmt.Sprintf("%v", isOverdue) == "1" {
		return "Quá hạn"
	}
	if fmt.Sprintf("%v", status) == strconv.Itoa(define.InstallmentStatusPartial) {
		return "Thanh toán 1 phần"
	}
	return "Chưa thu"
}

// serviceDebtExportCols — key khớp FE crm/report/debt/service-debts displayCols[].value.
func serviceDebtExportCols(loc *time.Location, showPhone bool) []exportCol {
	return []exportCol{
		{"id", "ID", func(r map[string]interface{}) interface{} { return r["id"] }},
		{"customer_id", "Khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }},
		{"customer_number", "Mã khách hàng", func(r map[string]interface{}) interface{} { return r["customer_code"] }},
		{"customer_old_code", "Mã khách cũ", func(r map[string]interface{}) interface{} { return r["customer_old_code"] }},
		{"phone", "Điện thoại", func(r map[string]interface{}) interface{} { return maskPhoneExport(showPhone, r["customer_phone"]) }},
		{"serviceUsedCode", "Mã dịch vụ sử dụng", func(r map[string]interface{}) interface{} { return r["code"] }},
		{"serviceName", "Dịch vụ", func(r map[string]interface{}) interface{} { return r["service_name"] }},
		{"serviceUsedCreate", "Ngày lên đơn", func(r map[string]interface{}) interface{} { return fmtUnixDate(r["created_day"], loc) }},
		{"serviceIncurred", "Phát sinh", func(r map[string]interface{}) interface{} { return toFloat(r["totalEnd"]) }},
		{"servicePaid", "Thanh toán", func(r map[string]interface{}) interface{} { return toFloat(r["payment"]) }},
		{"serviceDebt", "Công nợ", func(r map[string]interface{}) interface{} {
			// Tính bằng decimal; .InexactFloat64() chỉ ở ô Excel (kiểu số của Excel là double).
			return helpers.ToDecimal(r["totalEnd"]).Sub(helpers.ToDecimal(r["payment"])).InexactFloat64()
		}},
		{"customerDeposit", "Cọc còn lại của khách", func(r map[string]interface{}) interface{} { return toFloat(r["totalDeposit"]) }},
		{"branch_id", "Chi nhánh", func(r map[string]interface{}) interface{} { return r["branch_name"] }},
	}
}

// ExportServiceDebts — q đã có FROM order_service_used + LEFT JOIN customer + JOIN service + WHERE + ORDER của handler.
func ExportServiceDebts(c *gin.Context, q *gorm.DB) {
	loc := tz.LocFromContext(c, q.Session(&gorm.Session{NewDB: true}))
	showPhone := phoneutil.CanSeePhoneExport(c, q.Session(&gorm.Session{NewDB: true}))
	eq := q.Session(&gorm.Session{}).
		Select(`order_service_used.id, order_service_used.code,
			order_service_used.totalEnd, order_service_used.payment, order_service_used.created_day,
			customer.code AS customer_code, customer.old_code AS customer_old_code,
			customer.fullname AS customer_fullname, customer.phone AS customer_phone, customer.totalDeposit,
			service.name AS service_name, br.name AS branch_name`).
		Joins("LEFT JOIN branch br ON br.id = order_service_used.branch_id")

	// Phát sinh/Thanh toán/Công nợ/Cọc render cứng trên màn — luôn giữ trong export.
	cols := filterExportCols(serviceDebtExportCols(loc, showPhone), c.Query("columns"),
		"serviceIncurred", "servicePaid", "serviceDebt", "customerDeposit")
	headers, rowFn := exportHeadersRow(cols)
	url, err := doExport(c, eq, export.Config{
		FileName:    "Cong_no_dich_vu",
		Headers:     headers,
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, rowFn)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

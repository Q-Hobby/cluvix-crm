package export

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"
)

// PaymentByDateSQL dựng câu SQL (đã ORDER BY dayCreate DESC) + args của báo cáo Doanh thu theo ngày.
// Dùng CHUNG cho list (`crm/report/revenue` ReportPaymentByDate) và export (ExportPaymentByDate) —
// một nguồn duy nhất để list và file xuất không lệch bộ lọc/phạm vi chi nhánh.
//
// Cột "Đặt cọc" (totalDeposit) = tổng tiền cọc NHẬN trong ngày (`payment.amount` của phiếu cọc),
// KHÔNG phải số dư `deposit_after` (user chốt 2026-10-18, M-19): số dư đổi mỗi khi cọc được chuyển/
// hoàn ⇒ báo cáo của ngày đã qua tự đổi số.
func PaymentByDateSQL(c *gin.Context, db *gorm.DB) (string, []interface{}) {
	from, errF := strconv.ParseInt(c.Query("_ngay_thang[gte]"), 10, 64)
	if errF != nil {
		from = 0
	}
	to, errT := strconv.ParseInt(c.Query("_ngay_thang[lte]"), 10, 64)
	if errT != nil {
		to = 9999999999
	}

	offsetSec := tz.OffsetSec(tz.LocFromContext(c, db))
	dayBucket := func(col string) string {
		return fmt.Sprintf("FLOOR((%s + %d) / 86400) * 86400 - %d", col, offsetSec, offsetSec)
	}

	appCond, appArgs := buildByDateFilter(c, "appointment", true)
	otpCond, otpArgs := buildByDateFilter(c, "order_plan", false)
	payCond, payArgs := buildByDateFilter(c, "payment", false)
	// Phiên đang ở 1 chi nhánh (branchActiveId) ⇒ mọi nhánh UNION đã bị scope về 1 chi nhánh — cùng
	// trường hợp "đúng 1 chi nhánh" của PaymentByDateIndexHints (optimizer tự chọn fk-…-branch_id tốt hơn).
	branchCount := len(query.GetFilterValues(c, "branch_id"))
	if tenant.BoundsFromContext(c).BranchActiveID > 0 {
		branchCount = 1
	}
	appHint, otpHint, payHint := PaymentByDateIndexHints(branchCount, c.Query("_ngay_thang[gte]") != "")

	baseSQL := fmt.Sprintf(`
SELECT dayCreate,
	SUM(numAppointmentOk) as numAppOk,
	SUM(totalDoanhSo) as totalDoanhSo,
	SUM(totalThucThu) as totalThucThu,
	SUM(totalDeposit) as totalDeposit,
	SUM(totalRefundDepoit) as totalRefundDepoit,
	SUM(totalRefundService) as totalRefundService
FROM (
	(SELECT `+dayBucket("App.appointment_date")+` as dayCreate,
		COUNT(*) as numAppointmentOk,
		0 as totalDoanhSo, 0 as totalThucThu, 0 as totalDeposit, 0 as totalRefundDepoit, 0 as totalRefundService
	FROM (
		SELECT appointment.customer_id, MIN(appointment.appointment_date) as appointment_date
		FROM appointment%s
		INNER JOIN appointment_type ON appointment.appointment_type_id = appointment_type.id
		WHERE appointment_type.isTypeRevenue = 1
			AND appointment.appointment_date BETWEEN ? AND ?
			AND appointment.status IN (2,3)
			AND appointment.is_accepted = 1%s
		GROUP BY appointment.customer_id
	) as App
	GROUP BY dayCreate)
	UNION ALL
	(SELECT `+dayBucket("order_plan.created_time")+` as dayCreate,
		0 as numAppointmentOk,
		COALESCE(SUM(osu.totalEnd), 0) as totalDoanhSo,
		0 as totalThucThu, 0 as totalDeposit, 0 as totalRefundDepoit, 0 as totalRefundService
	FROM order_plan%s
	LEFT JOIN order_service_used osu ON osu.order_plan_id = order_plan.id
	WHERE order_plan.created_time BETWEEN ? AND ?%s
	GROUP BY dayCreate)
	UNION ALL
	(SELECT `+dayBucket("payment.payment_time")+` as dayCreate,
		0 as numAppointmentOk, 0 as totalDoanhSo,
		SUM(CASE WHEN payment.payment_for IN (%d, %d) AND payment.deposit_type != %d THEN payment.amount ELSE 0 END) as totalThucThu,
		SUM(CASE WHEN payment.payment_for = %d AND payment.deposit_type != %d THEN payment.amount ELSE 0 END) as totalDeposit,
		SUM(CASE WHEN payment.payment_for = %d THEN payment.amount ELSE 0 END) as totalRefundDepoit,
		SUM(CASE WHEN payment.payment_for = %d THEN payment.amount ELSE 0 END) as totalRefundService
	FROM payment%s
	WHERE payment.payment_time BETWEEN ? AND ?
		AND payment.payment_for IN (%d, %d, %d, %d)%s
	GROUP BY dayCreate)
) X
GROUP BY dayCreate`,
		appHint, appCond, otpHint, otpCond,
		define.PaymentForDeposit, define.PaymentForOrder, define.PaymentTypeDepositKG,
		define.PaymentForDeposit, define.PaymentTypeDepositKG,
		define.PaymentForRefundDeposit,
		define.PaymentForRefundService,
		payHint,
		define.PaymentForDeposit, define.PaymentForOrder, define.PaymentForRefundDeposit, define.PaymentForRefundService,
		payCond,
	)

	sortedSQL := fmt.Sprintf("SELECT * FROM (%s) AS t ORDER BY t.dayCreate DESC", baseSQL)

	args := make([]interface{}, 0)
	args = append(args, from, to)
	args = append(args, appArgs...)
	args = append(args, from, to)
	args = append(args, otpArgs...)
	args = append(args, from, to)
	args = append(args, payArgs...)
	return sortedSQL, args
}

// ExportPaymentByDate xuất báo cáo doanh thu theo ngày ra Excel/CSV.
func ExportPaymentByDate(c *gin.Context, db *gorm.DB) {
	sortedSQL, args := PaymentByDateSQL(c, db)
	loc := tz.LocFromContext(c, db)
	q := db.Raw(sortedSQL, args...)

	url, err := doExport(c, q, export.Config{
		FileName: "Doanh_thu_theo_ngay",
		// 1 dòng/ngày ⇒ luôn nhỏ: bỏ COUNT bọc baseSQL (chạy 3 nhánh UNION lần thứ hai).
		BoundedRows: true,
		Headers:     []string{"Ngày tháng", "Đồng ý", "Doanh số", "Thực thu", "Đặt cọc", "Hoàn cọc", "Hoàn dịch vụ"},
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		ts := toInt(row["dayCreate"])
		dateStr := fmtUnixDateInLoc(int64(ts), loc)
		return []interface{}{
			dateStr,
			toFloat(row["numAppOk"]),
			toFloat(row["totalDoanhSo"]),
			toFloat(row["totalThucThu"]),
			toFloat(row["totalDeposit"]),
			toFloat(row["totalRefundDepoit"]),
			toFloat(row["totalRefundService"]),
		}
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

// PaymentByDateIndexHints trả `FORCE INDEX` cho 3 nhánh baseSQL của báo cáo Doanh thu theo ngày — dùng chung
// cho list (`accountant/report` ReportPaymentByDate) và export ở đây (report import export ⇒ hàm phải nằm ở đây). Không hint, optimizer tra
// `code (company_id, code)` theo company (DB-per-customer mọi dòng cùng company ⇒ đọc TOÀN BỘ bảng) cho
// `order_plan`/`payment`, và dẫn `appointment` từ `appointment_type` qua chỉ mục FK.
//
// Số đo /check-scale 2026-09-14 @600.000 lịch hẹn / 400.000 kế hoạch / 600.000 phiếu thu (bench dựng
// từ migration), `EXPLAIN ANALYZE` từng nhánh, lượt 2:
//   - appointment: 1 năm 400 → 218ms · tháng 15–61 → 3,5ms · không lọc ngày ~hoà (513 / 502–614ms)
//   - order_plan: 1 năm 1.294 → 739ms · không lọc ngày 1.936–2.822 → 1.732–1.844ms · tháng hoà
//   - payment: 1 năm 729 → 540ms · tháng hoà · KHÔNG lọc ngày 939 → 1.178–1.243ms (TỆ hơn: chỉ mục thiếu
//     `deposit_after` nên vẫn tra PK từng dòng) ⇒ chỉ hint khi có mốc ngày bắt đầu
//   - đúng 1 chi nhánh: optimizer tự chọn `fk-…-branch_id` — payment không lọc ngày 74–129ms, ép hint
//     267–486ms (TỆ hơn) ⇒ không hint cả 3 nhánh
//
// LOẠI có số đo: `IGNORE INDEX (code)` (payment rơi sang `id_old (company_id)` — cùng bẫy, 701–840ms);
// chỉ mục mới `order_service_used (order_plan_id, totalEnd)` cho subquery doanh số (739 → 539–605ms,
// không đáng thêm chỉ mục trên bảng ghi nhiều); đổi subquery tương quan sang LEFT JOIN (655–1.859ms, dao động).
func PaymentByDateIndexHints(branchCount int, hasFrom bool) (app, otp, pay string) {
	if branchCount == 1 {
		return "", "", ""
	}
	app = " FORCE INDEX (" + paymentByDateAppointmentIndex + ")"
	otp = " FORCE INDEX (" + paymentByDateOrderPlanIndex + ")"
	if hasFrom {
		pay = " FORCE INDEX (" + paymentByDatePaymentIndex + ")"
	}
	return app, otp, pay
}

const (
	paymentByDateAppointmentIndex = "idx_appointment_date_scope"
	paymentByDateOrderPlanIndex   = "idx_order_plan_created_time_branch"
	paymentByDatePaymentIndex     = "idx_payment_time_cover"
)

// buildByDateFilter xây điều kiện WHERE cho từng bảng trong UNION.
func buildByDateFilter(c *gin.Context, table string, joinCustomer bool) (string, []interface{}) {
	cond := ""
	args := make([]interface{}, 0)

	if vals := query.GetFilterValues(c, "branch_id"); len(vals) > 0 {
		cond += fmt.Sprintf(" AND %s.branch_id IN (%s)", table, placeholderList(len(vals)))
		for _, v := range vals {
			args = append(args, v)
		}
	}

	// Story-38 (D15): bỏ vế company — 1 thương hiệu/DB, không cần lọc.
	// Phạm vi chi nhánh của phiên (branch là trục scope duy nhất) — cùng luật ScopedDBNullBranch:
	// branchActiveId thắng, không thì danh sách chi nhánh được phép; dòng branch_id IS NULL luôn lấy.
	if b := tenant.BoundsFromContext(c); b.BranchActiveID > 0 {
		cond += fmt.Sprintf(" AND (%s.branch_id = ? OR %s.branch_id IS NULL)", table, table)
		args = append(args, b.BranchActiveID)
	} else if len(b.BranchIDs) > 0 {
		cond += fmt.Sprintf(" AND (%s.branch_id IN (%s) OR %s.branch_id IS NULL)", table, placeholderList(len(b.BranchIDs)), table)
		for _, id := range b.BranchIDs {
			args = append(args, id)
		}
	}

	// source filter qua customer — dùng EXISTS tương quan, KHÔNG `IN (SELECT …)`.
	// Lý do + số đo đầy đủ: accountant/report/handler/report_payment_by_date.go
	// (hàm buildPaymentByDateFilter) — hàm này là bản SONG SINH của nó, phải sửa cùng lúc.
	if vals := query.GetFilterValues(c, "source_id"); len(vals) > 0 {
		if joinCustomer {
			cond += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM customer src_c WHERE src_c.id = appointment.customer_id AND src_c.source_id IN (%s))", placeholderList(len(vals)))
		} else {
			cond += fmt.Sprintf(" AND EXISTS (SELECT 1 FROM customer src_c WHERE src_c.id = %s.customer_id AND src_c.source_id IN (%s))", table, placeholderList(len(vals)))
		}
		for _, v := range vals {
			args = append(args, v)
		}
	}

	return cond, args
}

func placeholderList(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

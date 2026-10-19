package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/tz"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// PaymentInDay GET /api/print/payment-in-day
// Trả về mảng doanh thu theo ngày cho trang in.
func (h *Handler) PaymentInDay(c *gin.Context) {
	from := c.Query("_ngay_thang[gte]")
	to := c.Query("_ngay_thang[lte]")
	if from == "" {
		from = "0"
	}
	if to == "" {
		to = fmt.Sprintf("%d", 9999999999)
	}

	fromInt, _ := strconv.ParseInt(from, 10, 64)
	toInt, _ := strconv.ParseInt(to, 10, 64)

	appFilter, appArgs := h.buildFilter(c, "appointment", true)
	otpFilter, otpArgs := h.buildFilter(c, "order_plan", false)
	payFilter, payArgs := h.buildFilter(c, "payment", false)

	offsetSec := tz.OffsetSec(tz.LocFromContext(c, h.DB))
	dayBucket := func(col string) string {
		return fmt.Sprintf("FLOOR((%s + %d) / 86400) * 86400 - %d", col, offsetSec, offsetSec)
	}

	baseSQL := fmt.Sprintf(`
SELECT dayCreate,
	SUM(numAppointmentOk) as numAppOk,
	SUM(totalDoanhSo) as totalDoanhSo,
	SUM(totalThucThu) as totalThucThu,
	SUM(totalDeposit) as totalDeposit
FROM (
	(SELECT `+dayBucket("App.appointment_date")+` as dayCreate,
		COUNT(*) as numAppointmentOk,
		0 as totalDoanhSo, 0 as totalThucThu, 0 as totalDeposit
	FROM (
		SELECT appointment.customer_id, MIN(appointment.appointment_date) as appointment_date
		FROM appointment
		INNER JOIN appointment_type ON appointment.appointment_type_id = appointment_type.id
		WHERE appointment_type.isTypeRevenue = 1
			AND appointment.appointment_date BETWEEN ? AND ?
			AND appointment.status IN (2,3)
			AND appointment.is_accepted = 1%s
		GROUP BY appointment.customer_id
	) as App
	GROUP BY dayCreate)
	UNION ALL
	(SELECT `+dayBucket("totalRevenue.created_time")+` as dayCreate,
		0 as numAppointmentOk,
		SUM(totalRevenue.total) as totalDoanhSo,
		0 as totalThucThu, 0 as totalDeposit
	FROM (
		SELECT order_plan.created_time,
			COALESCE((SELECT SUM(osu.totalEnd) FROM order_service_used osu WHERE osu.order_plan_id = order_plan.id), 0) as total
		FROM order_plan
		WHERE order_plan.created_time BETWEEN ? AND ?%s
	) as totalRevenue
	GROUP BY dayCreate)
	UNION ALL
	(SELECT `+dayBucket("payTT.payment_time")+` as dayCreate,
		0 as numAppointmentOk, 0 as totalDoanhSo,
		SUM(payTT.amount) as totalThucThu,
		0 as totalDeposit
	FROM (
		SELECT payment.payment_time, payment.amount
		FROM payment
		WHERE payment.payment_for IN (%d, %d)
			AND payment.deposit_type != %d
			AND payment.payment_time BETWEEN ? AND ?%s
	) as payTT
	GROUP BY dayCreate)
	UNION ALL
	(SELECT `+dayBucket("payDc.payment_time")+` as dayCreate,
		0 as numAppointmentOk, 0 as totalDoanhSo, 0 as totalThucThu,
		SUM(payDc.deposit_after) as totalDeposit
	FROM (
		SELECT payment.payment_time, COALESCE(payment.deposit_after, 0) as deposit_after
		FROM payment
		WHERE payment.payment_for = %d
			AND payment.deposit_type != %d
			AND payment.payment_time BETWEEN ? AND ?%s
	) as payDc
	GROUP BY dayCreate)
) X
GROUP BY dayCreate
ORDER BY dayCreate DESC`,
		appFilter, otpFilter,
		define.PaymentForDeposit, define.PaymentForOrder, define.PaymentTypeDepositKG, payFilter,
		define.PaymentForDeposit, define.PaymentTypeDepositKG, payFilter,
	)

	args := make([]interface{}, 0)
	args = append(args, fromInt, toInt)
	args = append(args, appArgs...)
	args = append(args, fromInt, toInt)
	args = append(args, otpArgs...)
	args = append(args, fromInt, toInt)
	args = append(args, payArgs...)
	args = append(args, fromInt, toInt)
	args = append(args, payArgs...)

	items := make([]map[string]interface{}, 0)
	if err := h.DB.Raw(baseSQL, args...).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// totalDoanhSo/totalThucThu/totalDeposit là alias của SUM(...) qua nhiều lớp UNION ALL —
	// GORM Raw().Scan(&items) trả các alias này dạng string (không phải []uint8) nên
	// helpers.NormalizeSliceDecimals (chỉ bắt []uint8) là no-op ở đây; dùng ParseDecimalFields
	// (helpers.ToDecimal xử lý được cả string lẫn []uint8) đúng nhánh alias theo architecture.md.
	// story-34 (F-D/F-E): dayCreate (phép chia FLOOR/86400) và numAppOk (SUM(COUNT)) cũng ra JSON
	// string qua đường tương tự dù không phải cột tiền — parse cùng lúc theo alias.
	for _, item := range items {
		helpers.ParseDecimalFields(item, "totalDoanhSo", "totalThucThu", "totalDeposit", "dayCreate", "numAppOk")
	}
	c.JSON(http.StatusOK, items)
}

// buildFilter — story-38 (D15): bỏ vế company (1 thương hiệu/DB, không cần lọc). Endpoint này
// trước đây CŨNG không lọc branch (báo cáo tổng toàn tenant) nên không có vế thay thế.
func (h *Handler) buildFilter(c *gin.Context, table string, joinCustomer bool) (string, []interface{}) {
	return "", nil
}

package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/tz"
)

// ───────────────────────────────────────────────────────────────────────────────
// Tổng quan lịch hẹn theo năm (`/report/appointment-chart-scatter`, request (b)).
//
// Bản cũ LEFT JOIN `appointment` vào CHÍNH NÓ 4 lần theo customer_id (huy/dongy/khongden/
// khongdongy) rồi COUNT(DISTINCT …) ⇒ số dòng trung gian = (số hẹn tư vấn) × (số huỷ) × (số đồng
// ý) × (số không đến) × (số không đồng ý) cho MỖI khách; cộng thêm mọi điều kiện ngày bọc trong
// CONVERT_TZ(FROM_UNIXTIME(col)) nên không index nào dùng được. Bench 2026-10-19 (1,2M lịch hẹn /
// 500k khách): KHÔNG BAO GIỜ trả kết quả (HTTP timeout 120–180s, một câu chạy 816s tới khi bị kill;
// 1 khách đuôi dài 44 hẹn tư vấn = 97 triệu dòng trung gian, 148s).
//
// Ngữ nghĩa GIỮ NGUYÊN của bản cũ (đọc kỹ trước khi "đơn giản hoá"):
//   - Theo từng tháng của lịch hẹn GỐC `a` (appointment_type_id = 1, trong khoảng năm): đếm
//     KHÁCH (DISTINCT customer_id) có lịch gốc trong tháng đó.
//   - Trong SỐ CÁC KHÁCH ĐÓ, đếm khách có BẤT KỲ lịch hẹn nào (mọi loại) trong CẢ KHOẢNG NĂM — KHÔNG
//     chỉ riêng tháng đó — thoả: huỷ (status 4) · đồng ý (is_accepted 1, status 2/3) · không đến
//     (status 1) · không đồng ý (is_accepted != 1, status 2/3; is_accepted NULL KHÔNG tính, vì
//     `NULL != 1` là NULL).
//   - Mọi vế (gốc lẫn 4 cờ) cùng phạm vi chi nhánh NULL-safe (buildAppointmentBranchScopeSQL).
//   - Chỉ trả tháng có ít nhất 1 lịch gốc; khách NULL không đếm nhưng vẫn làm tháng xuất hiện.
//
// ⇒ Bản mới: MỘT lượt quét dải năm trên covering index `idx_appointment_date_scope` (đủ cột
// appointment_date/status/branch_id/appointment_type_id/is_accepted/customer_id), GROUP BY
// customer_id ra 4 cờ (MAX điều kiện) + bitmask tháng có lịch gốc (BIT_OR), rồi nhân với 12 tháng
// theo bit. Khoảng năm + mốc đầu tháng tính ở Go theo múi giờ thương hiệu; cột ngày thô so với số
// unix ⇒ dùng được index.
//
// Số đo EXPLAIN ANALYZE trên bench L, năm 2026 (581k lịch hẹn trong năm), mỗi bản đo MỘT lần:
//   - bản cũ                                             : không xong (816s, bị kill)
//   - 2 lượt quét (bảng cờ dẫn xuất + JOIN lịch gốc)     : 1 chi nhánh 1,02s · toàn bộ 1,62s
//   - 1 lượt quét, optimizer tự chọn                     : 1 chi nhánh 0,71s · toàn bộ 1,08s
//     (1 chi nhánh: optimizer đi `fk-appointment_branch_id` ref_or_null — 481k lần tra dòng)
//   - 1 lượt quét + hint INDEX(idx_appointment_date_scope): 1 chi nhánh 0,37s  ← bản này
//     Dùng optimizer hint chứ KHÔNG `FORCE INDEX`: index bị đổi tên thì hint chỉ sinh warning,
//     còn FORCE INDEX ném lỗi 1176 làm chết endpoint.
//
// ⚠ ĐỔI CÓ CHỦ ĐÍCH so với bản cũ: mốc cuối. Bản cũ so DATETIME với chuỗi ngày
// `BETWEEN 'YYYY-01-01' AND 'YYYY-12-31'` ⇒ MySQL hiểu mốc cuối là 'YYYY-12-31 00:00:00' ⇒ lịch hẹn
// NGÀY CUỐI (31/12, hoặc ngày cuối tháng hiện tại với năm nay) sau 00:00 bị bỏ khỏi báo cáo — đã
// kiểm trên MySQL 9.7. Bản mới lấy trọn ngày cuối (mốc cuối là đầu ngày kế tiếp, loại trừ).
// ───────────────────────────────────────────────────────────────────────────────

// appointmentOverviewBaseTypeID — loại lịch hẹn "gốc" mà báo cáo đếm theo tháng (giữ nguyên bản cũ;
// pkg/define chưa có hằng cho loại lịch hẹn).
const appointmentOverviewBaseTypeID = 1

// appointmentOverviewRange — khoảng [start, endExcl) (unix giây) của báo cáo năm `year` theo múi
// giờ thương hiệu, và 11 mốc đầu tháng 2..12 (đối số của INTERVAL() để chia tháng).
// Năm hiện tại: tới hết tháng hiện tại (mốc cuối = đầu tháng sau), như bản cũ.
func appointmentOverviewRange(loc *time.Location, year int, now time.Time) (start, endExcl int64, monthStarts []any) {
	start = time.Date(year, time.January, 1, 0, 0, 0, 0, loc).Unix()
	if n := now.In(loc); n.Year() == year {
		endExcl = time.Date(year, n.Month()+1, 1, 0, 0, 0, 0, loc).Unix()
	} else {
		endExcl = time.Date(year+1, time.January, 1, 0, 0, 0, 0, loc).Unix()
	}
	monthStarts = make([]any, 0, 11)
	for m := time.February; m <= time.December; m++ {
		monthStarts = append(monthStarts, time.Date(year, m, 1, 0, 0, 0, 0, loc).Unix())
	}
	return start, endExcl, monthStarts
}

// appointmentOverviewSQL dựng câu tổng quan. `scope`/`scopeArgs` là fragment phạm vi chi nhánh cho
// alias `x` (buildAppointmentBranchScopeSQL). Tách hàm để test đối chiếu chạy đúng câu production.
func appointmentOverviewSQL(start, endExcl int64, monthStarts []any, scope string, scopeArgs []any) (string, []any) {
	sql := fmt.Sprintf(`SELECT /*+ MAX_EXECUTION_TIME(%d) */
		m.thang_lh,
		COUNT(t.customer_id) AS lich_hen,
		COUNT(CASE WHEN t.huy = 1 THEN t.customer_id END) AS lich_huy,
		COUNT(CASE WHEN t.dongy = 1 THEN t.customer_id END) AS dong_y,
		COUNT(CASE WHEN t.khongden = 1 THEN t.customer_id END) AS khong_den,
		COUNT(CASE WHEN t.khongdongy = 1 THEN t.customer_id END) AS khong_dong_y
	FROM (
		SELECT /*+ INDEX(x idx_appointment_date_scope) */
			x.customer_id,
			BIT_OR(CASE WHEN x.appointment_type_id = %d
				THEN 1 << INTERVAL(x.appointment_date, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ELSE 0 END) AS mask,
			MAX(x.status = %d) AS huy,
			MAX(x.is_accepted = %d AND x.status IN (%d, %d)) AS dongy,
			MAX(x.status = %d) AS khongden,
			MAX(x.is_accepted != %d AND x.status IN (%d, %d)) AS khongdongy
		FROM appointment x
		WHERE x.appointment_date >= ? AND x.appointment_date < ?%s
		GROUP BY x.customer_id
	) t
	JOIN (VALUES ROW(1), ROW(2), ROW(3), ROW(4), ROW(5), ROW(6),
		ROW(7), ROW(8), ROW(9), ROW(10), ROW(11), ROW(12)) AS m (thang_lh)
		ON t.mask & (1 << (m.thang_lh - 1))
	GROUP BY m.thang_lh
	ORDER BY m.thang_lh DESC`,
		define.AppointmentOverviewMaxExecMs,
		appointmentOverviewBaseTypeID,
		define.AppointmentStatusCancel,
		define.IsAccepted, define.AppointmentStatusCheckedIn, define.AppointmentStatusCheckedOut,
		define.AppointmentStatusNotCheckin,
		define.IsAccepted, define.AppointmentStatusCheckedIn, define.AppointmentStatusCheckedOut,
		scope)

	args := make([]any, 0, len(monthStarts)+2+len(scopeArgs))
	args = append(args, monthStarts...)
	args = append(args, start, endExcl)
	args = append(args, scopeArgs...)
	return sql, args
}

// AppointmentOverview GET /crm/appointment/appointment-overview?year=2026
func (h *Handler) AppointmentOverview(c *gin.Context) {
	loc := tz.LocFromContext(c, h.DB)
	now := time.Now().In(loc)

	year := now.Year()
	if v := c.Query("year"); v != "" {
		y, err := strconv.Atoi(v)
		if err != nil || y < 1970 || y > 9999 {
			// Bản cũ ghép thẳng chuỗi vào ngày ("abc-01-01") ⇒ không khớp dòng nào ⇒ 200 + [].
			// Giữ nguyên hợp đồng đó cho tham số rác.
			c.JSON(http.StatusOK, []map[string]any{})
			return
		}
		year = y
	}

	start, endExcl, monthStarts := appointmentOverviewRange(loc, year, now)
	scope, scopeArgs := buildAppointmentBranchScopeSQL(c, "x")
	sql, args := appointmentOverviewSQL(start, endExcl, monthStarts, scope, scopeArgs)

	// WithContext: client ngắt ⇒ giải phóng goroutine + connection ngay (driver đóng socket).
	// Câu phía MySQL thì chỉ dừng nhờ MAX_EXECUTION_TIME — xem define.AppointmentOverviewMaxExecMs.
	rows := make([]map[string]any, 0)
	tx := h.DB.WithContext(c.Request.Context()).Raw(sql, args...).Find(&rows)
	if tx.Error != nil && c.Request.Context().Err() != nil {
		// Client đã ngắt: không còn ai nhận response — KHÔNG đi FailQuery (logger.Error ⇒ cảnh báo
		// Telegram cho mỗi lần người dùng đóng tab/reload).
		c.Abort()
		return
	}
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	c.JSON(http.StatusOK, rows)
}

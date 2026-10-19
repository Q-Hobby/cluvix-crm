package handler

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"
)

// dashboardFilter — bộ lọc chung của dashboard CRM (port từ ERP epic crm-dashboard-customizable).
//
// Hợp đồng query param: `from`/`to` là Unix giây, nửa mở [from, to) — `to` là mốc LOẠI TRỪ (đầu ngày
// kế tiếp). Phần lớn câu SQL cũ dùng `BETWEEN ? AND ?` với mốc cuối BAO GỒM (tz.MonthRange trả 23:59:59)
// ⇒ câu `BETWEEN` truyền ToIncl(), câu `<` truyền To. Giữ đúng toán tử cũ của từng câu thì gọi KHÔNG
// tham số ra kết quả giống hệt trước khi có bộ lọc.
type dashboardFilter struct {
	From     int64 // mốc đầu, bao gồm
	To       int64 // mốc cuối, LOẠI TRỪ
	RefDay   int64 // đầu "ngày tham chiếu" theo loc: ngày của min(To-1, now)
	BranchID int   // 0 = giữ scope chi nhánh của middleware
	Explicit bool  // request có gửi from/to
}

// ToIncl — mốc cuối bao gồm, dùng cho câu `BETWEEN ? AND ?`.
func (f dashboardFilter) ToIncl() int64 { return f.To - 1 }

// cacheSuffix — PHẢI nối vào mọi khoá cache/singleflight của endpoint đã nhận bộ lọc: thiếu kỳ hoặc chi
// nhánh trong khoá = 2 phạm vi dùng chung kết quả (lẫn số giữa kỳ / rò chéo chi nhánh). RefDay có mặt vì số
// liệu "ngày tham chiếu" đổi mỗi ngày dù kỳ mặc định (tháng này) chưa đổi.
func (f dashboardFilter) cacheSuffix() string {
	return fmt.Sprintf("_f%d_t%d_r%d_b%d", f.From, f.To, f.RefDay, f.BranchID)
}

// dashCacheKey — khoá cache DUY NHẤT cho endpoint đã nhận bộ lọc: prefix + phạm vi tenant + bộ lọc. Handler
// không tự Sprintf khoá ⇒ test khoá ở đây là test đúng khoá handler dùng.
func dashCacheKey(c *gin.Context, prefix string, f dashboardFilter) string {
	return prefix + "_" + tenantCacheKey(c) + f.cacheSuffix()
}

// helperCacheKey — khoá cho 5 helper tiền (queryDoanhSo…): cửa sổ [from,to] đã quy đổi + chi nhánh.
func helperCacheKey(c *gin.Context, prefix string, from, to int64, branchID int) string {
	return fmt.Sprintf("%s_%s_%d_%d_b%d", prefix, tenantCacheKey(c), from, to, branchID)
}

// parseDashboardFilter đọc from/to/branch_id. defFrom/defToExcl là cửa sổ mặc định RIÊNG của từng handler
// (đúng phép tính code cũ) khi request không gửi from/to. Lỗi thì đã ghi response và trả ok=false:
// from/to sai ⇒ 422; branch_id không phải số nguyên ≥ 0 ⇒ 422; chi nhánh ngoài quyền ⇒ 403.
func (h *Handler) parseDashboardFilter(c *gin.Context, loc *time.Location, defFrom, defToExcl int64) (dashboardFilter, bool) {
	f, err := parseDashboardRange(c.Query("from"), c.Query("to"), defFrom, defToExcl)
	if err != nil {
		response.Validate(c, i18n.T(c, err.Error()))
		return dashboardFilter{}, false
	}
	f.RefDay = refDayStart(f.To, time.Now().Unix(), loc)

	// Hard Rule id param: parse ra số trước khi vào query — '5abc' không được ép thành 5 (dashboardBranchParam
	// coi lỗi parse là "không chọn", nên phải chặn định dạng ở đây trước). "0" = không chọn chi nhánh.
	if raw := c.Query("branch_id"); raw != "" {
		if id, perr := strconv.Atoi(raw); perr != nil || id < 0 {
			response.Validate(c, i18n.T(c, "crm_dashboard_branch_invalid"))
			return dashboardFilter{}, false
		}
	}
	branchID, ok := dashboardBranchParam(c)
	if !ok {
		response.Forbidden(c)
		return dashboardFilter{}, false
	}
	f.BranchID = branchID
	return f, true
}

// parseDashboardRange — phần thuần (không DB, không response) để unit test. Trả error mang i18n key.
func parseDashboardRange(rawFrom, rawTo string, defFrom, defToExcl int64) (dashboardFilter, error) {
	if rawFrom == "" && rawTo == "" {
		return dashboardFilter{From: defFrom, To: defToExcl}, nil
	}
	if rawFrom == "" || rawTo == "" {
		return dashboardFilter{}, errors.New("crm_dashboard_range_invalid")
	}
	from, e1 := strconv.ParseInt(rawFrom, 10, 64)
	to, e2 := strconv.ParseInt(rawTo, 10, 64)
	if e1 != nil || e2 != nil || from <= 0 || to <= from {
		return dashboardFilter{}, errors.New("crm_dashboard_range_invalid")
	}
	if to-from > int64(define.CrmDashboardRangeMaxDays)*86400 {
		return dashboardFilter{}, errors.New("crm_dashboard_range_too_long")
	}
	return dashboardFilter{From: from, To: to, Explicit: true}, nil
}

// refDayStart — đầu ngày (theo loc) của min(toExcl-1, now). Kỳ chứa hôm nay ⇒ hôm nay; kỳ đã qua ⇒ ngày
// cuối kỳ; kỳ tương lai ⇒ hôm nay. FE tính nhãn bằng CÙNG công thức này.
func refDayStart(toExcl, now int64, loc *time.Location) int64 {
	t := toExcl - 1
	if now < t {
		t = now
	}
	d := time.Unix(t, 0).In(loc)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc).Unix()
}

// lastPeriod — kỳ so sánh liền trước: kỳ là ĐÚNG 1 tháng dương lịch ⇒ tháng dương lịch trước (giữ hành vi
// cũ, tháng dài ngắn khác nhau); ngược lại ⇒ đoạn cùng độ dài ngay trước From. Trả [from, toExcl).
func lastPeriod(from, toExcl int64, loc *time.Location) (int64, int64) {
	start := time.Unix(from, 0).In(loc)
	if start.Day() == 1 && start.Hour() == 0 && start.Minute() == 0 && start.Second() == 0 &&
		start.AddDate(0, 1, 0).Unix() == toExcl {
		return start.AddDate(0, -1, 0).Unix(), from
	}
	return from - (toExcl - from), from
}

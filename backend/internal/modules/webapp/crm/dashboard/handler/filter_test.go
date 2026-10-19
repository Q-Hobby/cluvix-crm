package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

var hcm = func() *time.Location {
	l, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic(err)
	}
	return l
}()

func unix(y int, m time.Month, d int) int64 { return time.Date(y, m, d, 0, 0, 0, 0, hcm).Unix() }

func TestParseDashboardRange(t *testing.T) {
	defFrom, defTo := unix(2026, 10, 1), unix(2026, 11, 1)
	cases := []struct {
		name, from, to string
		wantErr        string
		wantFrom       int64
		wantTo         int64
		wantExplicit   bool
	}{
		{"không tham số ⇒ default", "", "", "", defFrom, defTo, false},
		{"đủ, hợp lệ", "1759251600", "1759338000", "", 1759251600, 1759338000, true},
		{"thiếu to", "1759251600", "", "crm_dashboard_range_invalid", 0, 0, false},
		{"thiếu from", "", "1759338000", "crm_dashboard_range_invalid", 0, 0, false},
		{"to = from", "1759251600", "1759251600", "crm_dashboard_range_invalid", 0, 0, false},
		{"to < from", "1759338000", "1759251600", "crm_dashboard_range_invalid", 0, 0, false},
		{"không phải số", "5abc", "1759338000", "crm_dashboard_range_invalid", 0, 0, false},
		{"from âm", "-1", "1759338000", "crm_dashboard_range_invalid", 0, 0, false},
		{"đúng 366 ngày", "1700000000", "1731622400", "", 1700000000, 1731622400, true},
		{"367 ngày", "1700000000", "1731708800", "crm_dashboard_range_too_long", 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parseDashboardRange(tc.from, tc.to, defFrom, defTo)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("muốn lỗi %q, nhận %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("không muốn lỗi, nhận %v", err)
			}
			if f.From != tc.wantFrom || f.To != tc.wantTo || f.Explicit != tc.wantExplicit {
				t.Fatalf("nhận %+v", f)
			}
		})
	}
}

func TestRefDayStart(t *testing.T) {
	now := time.Date(2026, 10, 8, 15, 30, 0, 0, hcm).Unix()
	cases := []struct {
		name string
		to   int64
		want int64
	}{
		{"kỳ chứa hôm nay ⇒ hôm nay", unix(2026, 11, 1), unix(2026, 10, 8)},
		{"tháng trước ⇒ ngày cuối tháng trước", unix(2026, 10, 1), unix(2026, 9, 30)},
		{"kỳ tương lai ⇒ hôm nay", unix(2027, 1, 1), unix(2026, 10, 8)},
		{"kỳ = đúng hôm nay", unix(2026, 10, 9), unix(2026, 10, 8)},
	}
	for _, tc := range cases {
		if got := refDayStart(tc.to, now, hcm); got != tc.want {
			t.Errorf("%s: nhận %d muốn %d", tc.name, got, tc.want)
		}
	}
}

func TestLastPeriod(t *testing.T) {
	// Tháng dương lịch ⇒ tháng trước (độ dài khác nhau: 10/2026 có 31 ngày, 9/2026 có 30).
	f, to := lastPeriod(unix(2026, 10, 1), unix(2026, 11, 1), hcm)
	if f != unix(2026, 9, 1) || to != unix(2026, 10, 1) {
		t.Fatalf("tháng: nhận [%d,%d)", f, to)
	}
	// Tháng 3 ⇒ tháng 2 (28 ngày) — không được lấy "31 ngày trước".
	f, to = lastPeriod(unix(2026, 3, 1), unix(2026, 4, 1), hcm)
	if f != unix(2026, 2, 1) || to != unix(2026, 3, 1) {
		t.Fatalf("tháng 3: nhận [%d,%d)", f, to)
	}
	// Kỳ lẻ 7 ngày ⇒ 7 ngày ngay trước.
	f, to = lastPeriod(unix(2026, 10, 2), unix(2026, 10, 9), hcm)
	if f != unix(2026, 9, 25) || to != unix(2026, 10, 2) {
		t.Fatalf("7 ngày: nhận [%d,%d)", f, to)
	}
	// Bắt đầu ngày 1 nhưng không trọn tháng ⇒ cùng độ dài.
	f, to = lastPeriod(unix(2026, 10, 1), unix(2026, 10, 16), hcm)
	if f != unix(2026, 9, 16) || to != unix(2026, 10, 1) {
		t.Fatalf("nửa tháng: nhận [%d,%d)", f, to)
	}
}

func TestCacheSuffixDistinct(t *testing.T) {
	base := dashboardFilter{From: 1, To: 2, RefDay: 3, BranchID: 0}
	variants := []dashboardFilter{
		{From: 9, To: 2, RefDay: 3},
		{From: 1, To: 9, RefDay: 3},
		{From: 1, To: 2, RefDay: 9},
		{From: 1, To: 2, RefDay: 3, BranchID: 4},
	}
	for i, v := range variants {
		if v.cacheSuffix() == base.cacheSuffix() {
			t.Errorf("biến thể %d trùng khoá với base", i)
		}
	}
}

// newFilterCtx dựng gin context như middleware auth set (CRM: 1 thương hiệu/DB, không có company).
func newFilterCtx(q string, branchIDs []uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/x?"+q, nil)
	c.Set("branchActiveId", uint(0))
	c.Set("branchIds", branchIDs)
	return c, w
}

func TestParseDashboardFilterBranch(t *testing.T) {
	// CRM: kiểm quyền chi nhánh qua dashboardBranchParam (middleware.IsBranchAllowed) — không cần DB.
	h := &Handler{}
	def0, def1 := unix(2026, 10, 1), unix(2026, 11, 1)

	c, w := newFilterCtx("branch_id=7", []uint{1, 2, 3})
	if _, ok := h.parseDashboardFilter(c, hcm, def0, def1); ok || w.Code != http.StatusForbidden {
		t.Fatalf("chi nhánh ngoài phạm vi phải 403, nhận ok=%v code=%d", ok, w.Code)
	}

	for _, bad := range []string{"5abc", "-3", "x", "1.5"} {
		c, w := newFilterCtx("branch_id="+bad, []uint{1, 2, 3})
		if _, ok := h.parseDashboardFilter(c, hcm, def0, def1); ok || w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("branch_id=%q phải 422, nhận ok=%v code=%d", bad, ok, w.Code)
		}
	}

	// Trong phạm vi ⇒ qua, BranchID = chi nhánh chọn. "0"/vắng ⇒ không chọn (giữ scope phiên).
	c, _ = newFilterCtx("branch_id=2", []uint{1, 2, 3})
	if f, ok := h.parseDashboardFilter(c, hcm, def0, def1); !ok || f.BranchID != 2 {
		t.Fatalf("chi nhánh trong phạm vi phải qua với BranchID=2, nhận ok=%v f=%+v", ok, f)
	}
	for _, q := range []string{"", "branch_id=0"} {
		c, _ = newFilterCtx(q, []uint{1, 2, 3})
		if f, ok := h.parseDashboardFilter(c, hcm, def0, def1); !ok || f.BranchID != 0 || f.From != def0 || f.To != def1 || f.Explicit {
			t.Fatalf("%q phải giữ mặc định, nhận ok=%v f=%+v", q, ok, f)
		}
	}

	c, w = newFilterCtx("from=1759251600", []uint{1})
	if _, ok := h.parseDashboardFilter(c, hcm, def0, def1); ok || w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("kỳ thiếu to phải 422, nhận ok=%v code=%d", ok, w.Code)
	}
}

// Khoá cache handler/helper dựng qua dashCacheKey/helperCacheKey (QA pass 1, P1): phạm vi tenant, kỳ,
// ngày tham chiếu, chi nhánh khác nhau ⇒ PHẢI khác khoá; cùng đầu vào ⇒ cùng khoá.
func TestDashCacheKeySeparatesScopes(t *testing.T) {
	base := dashboardFilter{From: 100, To: 200, RefDay: 150, BranchID: 0}
	c1, _ := newFilterCtx("", []uint{1, 2})
	c2, _ := newFilterCtx("", []uint{1, 3}) // user khác phạm vi chi nhánh
	k := dashCacheKey(c1, "crm_x", base)
	if k != dashCacheKey(c1, "crm_x", base) {
		t.Fatal("cùng đầu vào phải cùng khoá")
	}
	diff := map[string]string{
		"tenant": dashCacheKey(c2, "crm_x", base),
		"from":   dashCacheKey(c1, "crm_x", dashboardFilter{From: 101, To: 200, RefDay: 150}),
		"to":     dashCacheKey(c1, "crm_x", dashboardFilter{From: 100, To: 201, RefDay: 150}),
		"refDay": dashCacheKey(c1, "crm_x", dashboardFilter{From: 100, To: 200, RefDay: 151}),
		"branch": dashCacheKey(c1, "crm_x", dashboardFilter{From: 100, To: 200, RefDay: 150, BranchID: 2}),
		"prefix": dashCacheKey(c1, "crm_y", base),
	}
	for name, other := range diff {
		if other == k {
			t.Errorf("khác %s mà trùng khoá: %s", name, k)
		}
	}
	h := helperCacheKey(c1, "crm_doanh_so", 1, 2, 0)
	for name, other := range map[string]string{
		"branch": helperCacheKey(c1, "crm_doanh_so", 1, 2, 5),
		"tenant": helperCacheKey(c2, "crm_doanh_so", 1, 2, 0),
		"window": helperCacheKey(c1, "crm_doanh_so", 1, 3, 0),
	} {
		if other == h {
			t.Errorf("helper khác %s mà trùng khoá", name)
		}
	}
}

package debt

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Neo bản vá hiệu năng 2026-10-19 của GET /crm/report/debt/service (số đo ở serviceDebtTable). Cả hai thứ
// khoá ở đây trả ĐÚNG cùng kết quả khi bị revert — chỉ đổi plan — nên test so số không bắt được.
// Hàm thuần, không DB. Khuôn SQL (STRAIGHT_JOIN + hint trong câu thật) neo ở
// service_debt_parity_integration_test.go::TestServiceDebtQuery_OsuLuonDanTruocService (cần DB vì
// serviceDebtQuery đọc phạm vi telesale — DryRun không chạy được Raw Scan).

func debtHintCtx(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x?"+rawQuery, nil)
	c.Set("branchActiveId", uint(0))
	c.Set("branchIds", []uint{1, 2})
	return c
}

func TestServiceDebtTable_CongForceIndex(t *testing.T) {
	const month = 19 * 86400
	forced := "order_service_used FORCE INDEX (" + serviceDebtDateIndex + ")"
	cases := []struct {
		name     string
		q        string
		from, to int64
		noHint   bool
		want     string
	}{
		{"tháng", "", 1790787600, 1790787600 + month, false, forced},
		{"đúng 31 ngày", "", 1000, 1000 + serviceDebtHintMaxRangeSec, false, forced},
		{"31 ngày + 1 giây", "", 1000, 1001 + serviceDebtHintMaxRangeSec, false, "order_service_used"},
		{"1 năm", "", 1760893200, 1792429199, false, "order_service_used"},
		{"không lọc ngày", "", 0, 0, false, "order_service_used"},
		{"tháng + lọc dịch vụ", "filter[service_id][0]=5", 1790787600, 1790787600 + month, false, "order_service_used"},
		{"tháng, caller cấm hint (câu trang / telesale)", "", 1790787600, 1790787600 + month, true, "order_service_used"},
		{"tháng + từ khoá + 1 chi nhánh", "_keyword=a&filter[branch_id][0]=1", 1790787600, 1790787600 + month, false, forced},
	}
	for _, tc := range cases {
		if got := serviceDebtTable(debtHintCtx(tc.q), tc.from, tc.to, !tc.noHint); got != tc.want {
			t.Errorf("%s: want %q, got %q", tc.name, tc.want, got)
		}
	}
}

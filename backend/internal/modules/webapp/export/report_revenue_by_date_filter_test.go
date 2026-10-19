package export

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Bản SONG SINH của buildPaymentByDateFilter (accountant/report) — cùng bản vá EXISTS
// 2026-09-09. Test này tồn tại để một trong hai bị revert thì lưới vẫn đỏ.
func TestBuildByDateFilter_SourceDungEXISTS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		table    string
		joinCust bool
		want     string
	}{
		{"nhánh appointment", "appointment", true, "src_c.id = appointment.customer_id"},
		{"nhánh payment", "payment", false, "src_c.id = payment.customer_id"},
		{"nhánh order_plan", "order_plan", false, "src_c.id = order_plan.customer_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?filter[source_id][0]=7", nil)
			cond, args := buildByDateFilter(c, tc.table, tc.joinCust)
			if strings.Contains(cond, "customer_id IN (SELECT") {
				t.Fatalf("bản vá bị revert — còn IN-subquery: %s", cond)
			}
			if !strings.Contains(cond, "EXISTS (SELECT 1 FROM customer src_c") || !strings.Contains(cond, tc.want) {
				t.Fatalf("thiếu EXISTS tương quan đúng cột: %s", cond)
			}
			if len(args) != 1 {
				t.Fatalf("phải giữ 1 placeholder, got %d", len(args))
			}
		})
	}
}

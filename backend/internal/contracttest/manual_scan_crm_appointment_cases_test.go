//go:build integration

// Epic swallowed-sql-errors-sweep, story-04a (cụm `crm/appointment`) — lưới FR5.
//
// Cơ chế dùng chung (`pagination.FailQuery`) đã có lưới ở `manual_scan_failquery_cases_test.go`
// (story-02) + case đặc thù derived-table ở `manual_scan_accountant_cases_test.go` (story-03).
// File này thêm 1 case đại diện hình dạng đặc thù `crm/appointment`: query có `.Group(...)` (pivot
// lịch làm việc theo giờ/theo bác sĩ) — khẳng định lỗi GROUP BY vẫn bị bắt đúng.
package contracttest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/pagination"
)

func TestFailQuery_CrmAppointmentGroupByError(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	h := func(c *gin.Context) {
		q := testDB.Table("appointment").
			Select("appointment.branch_id, appointment.cot_khong_ton_tai, COUNT(*) as booked").
			Where("appointment.status != 4")
		var rows []map[string]interface{}
		tx := q.Group("appointment.branch_id, appointment.cot_khong_ton_tai").Find(&rows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		c.JSON(http.StatusOK, gin.H{"columns": []int{}, "rows": []int{}})
	}

	w := runFailQueryRoute(t, h)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GROUP BY lỗi SQL nhưng không trả 500, nhận %d\nbody: %s", w.Code, w.Body.String())
	}
	obj := decodeSingleJSON(t, w)
	if _, ok := obj["columns"]; ok {
		t.Fatalf("response lỗi KHÔNG được chứa `columns` — c.JSON thứ 2 của handler đã lọt ra\nbody: %s", w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"cot_khong_ton_tai", "Unknown column", "GROUP BY"} {
		if strings.Contains(body, leak) {
			t.Fatalf("message rò rỉ mảnh SQL thô (%q)\nbody: %s", leak, body)
		}
	}
}

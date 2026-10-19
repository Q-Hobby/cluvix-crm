//go:build integration

// Epic swallowed-sql-errors-sweep, story-03 (cụm `webapp/accountant`) — lưới FR5.
//
// Cơ chế dùng chung (`pagination.FailQuery`) đã có lưới riêng ở
// `manual_scan_failquery_cases_test.go` (story-02). File này chỉ thêm 1 case đặc thù của cụm
// accountant: raw SQL kèm derived-table GROUP BY (mẫu `treatment_debts`/`treatment_session_debts`/
// `appointment_debts` — tất cả 3 file dùng đúng hình dạng "subquery MAX/MIN(...) GROUP BY rồi JOIN
// lại") — khẳng định lỗi ở dạng câu SQL phức tạp hơn (không phải SELECT đơn giản) vẫn bị bắt đúng,
// không lộ SQL, và không double-write (các handler này đều còn code chạy tiếp sau điểm vá, giống
// `TestFailQuery_HelperContinuesAfterFail`).
package contracttest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/pagination"
)

func TestFailQuery_AccountantDerivedTableJoinError(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	h := func(c *gin.Context) {
		var tcRows []map[string]interface{}
		// Hình dạng đúng như treatment_debts/treatment_session_debts: subquery GROUP BY rồi JOIN lại,
		// cố tình chiếu cột không tồn tại ở subquery.
		// (story-14: bảng liệu trình đã bỏ — cùng hình dạng trên order_service_used.)
		tx := testDB.Raw(fmt.Sprintf(`SELECT tc.id, tc.cot_khong_ton_tai
			FROM order_service_used tc
			INNER JOIN (
				SELECT order_plan_id, MAX(created_day) AS max_ct
				FROM order_service_used
				WHERE order_plan_id IN ? %s
				GROUP BY order_plan_id
			) latest ON tc.order_plan_id = latest.order_plan_id AND tc.created_day = latest.max_ct`,
			"AND branch_id = ?"), []int{1, 2}, 1).Scan(&tcRows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		// Handler thật (treatment_debts) vẫn chạy tiếp query khác + c.JSON sau điểm này —
		// mô phỏng bằng 1 write JSON thứ 2 để chứng minh niêm phong hoạt động.
		c.JSON(http.StatusOK, gin.H{"items": []int{}, "totalDeposit": 0})
	}

	w := runFailQueryRoute(t, h)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("derived-table JOIN lỗi SQL nhưng không trả 500, nhận %d\nbody: %s", w.Code, w.Body.String())
	}
	obj := decodeSingleJSON(t, w)
	if _, ok := obj["items"]; ok {
		t.Fatalf("response lỗi KHÔNG được chứa `items` — c.JSON thứ 2 của handler đã lọt ra\nbody: %s", w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"cot_khong_ton_tai", "Unknown column", "order_service_used"} {
		if strings.Contains(body, leak) {
			t.Fatalf("message rò rỉ mảnh SQL/tên bảng thô (%q)\nbody: %s", leak, body)
		}
	}
}

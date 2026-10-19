//go:build integration

// Epic swallowed-sql-errors-sweep, story-05 (module rải rác: config/customer-care/loyalty/zalo/hr/
// sale/med/emr/auth/ai/history/mobile/v1) — lưới FR5.
//
// Cơ chế dùng chung đã có lưới ở story-02/03/04a. File này thêm 1 case đại diện hình dạng MỚI xuất
// hiện lần đầu ở cụm này: service method KHÔNG có `*gin.Context` (vd `RbacService.GetRolesDropdown`,
// `UserService.GetAllUsers`) tự trả `(rows, error)`, caller (handler, có `c`) mới gọi
// `pagination.FailQuery`. Khác story-04b (aggregator gộp 13 builder) — ở đây là 1 service method
// đơn giản, chỉ cần đổi chữ ký trả thêm `error`, không cần đổi kiến trúc.
package contracttest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/pagination"
)

func TestFailQuery_ServiceLayerNoContext(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	// serviceCall mô phỏng 1 service method thật (không có c), tự check lỗi và trả error lên caller.
	serviceCall := func() ([]map[string]interface{}, error) {
		var rows []map[string]interface{}
		tx := testDB.Table("rbac_auth_item").Select("cot_khong_ton_tai").Find(&rows)
		if pagination.IsQueryError(tx.Error) {
			return nil, tx.Error
		}
		return rows, nil
	}

	// handler (có c) gọi service, xử lý lỗi qua FailQuery — đúng pattern story-05.
	h := func(c *gin.Context) {
		rows, err := serviceCall()
		if pagination.IsQueryError(err) {
			pagination.FailQuery(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": rows})
	}

	w := runFailQueryRoute(t, h)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("service layer lỗi SQL nhưng handler không trả 500 — lỗi bị nuốt qua tầng service, nhận %d\nbody: %s", w.Code, w.Body.String())
	}
	obj := decodeSingleJSON(t, w)
	if _, ok := obj["data"]; ok {
		t.Fatalf("response lỗi KHÔNG được chứa `data`\nbody: %s", w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "cot_khong_ton_tai") || strings.Contains(body, "Unknown column") {
		t.Fatalf("message rò rỉ mảnh SQL thô\nbody: %s", body)
	}
}

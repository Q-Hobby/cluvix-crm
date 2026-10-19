package pagination

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
)

// story-27 AC2 — failList/FailQuery (đường ĐỌC — SELECT/list) phải LUÔN trả 500 sạch, không rẽ
// nhánh 422 + echo mảnh SQL như response.ServerError làm cho đường GHI (INSERT/UPDATE), kể cả khi
// lỗi query mang đúng dạng mã lỗi MySQL mà ServerError vẫn nhận diện (1062/1451/1452/1364/1048/1406).
func TestFailQuery_NeverEchoesRawSQLOrBranches422(t *testing.T) {
	logger.Init(logger.Config{Env: "test", AppCode: "pagination_test"})

	// Mô phỏng lỗi 1062 (Duplicate entry) phát sinh từ 1 SELECT (vd unique key trùng trong
	// subquery lồng) — dạng lỗi y hệt cái response.ServerError rẽ nhánh 422 + echo message thô.
	sqlErr := errors.New("Error 1062 (23000): Duplicate entry 'secret-column-value' for key 'uniq_x' -- SELECT c.id FROM customer c WHERE c.code = 'X'")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/_test/list", nil)

	FailQuery(c, sqlErr)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("FailQuery trên lỗi 1062 phát sinh từ SELECT phải LUÔN trả 500, được HTTP %d\nbody: %s",
			w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "secret-column-value") || strings.Contains(body, "SELECT c.id") ||
		strings.Contains(body, "Duplicate entry") {
		t.Fatalf("FailQuery không được echo mảnh SQL/message driver thô ra response, được: %s", body)
	}
}

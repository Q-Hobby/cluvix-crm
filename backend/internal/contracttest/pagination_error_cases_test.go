//go:build integration

// story-20 — lưới canh: `pagination.Paginate` / `PaginateWithTotal` KHÔNG được biến lỗi SQL
// thành "HTTP 200 + list rỗng".
//
// Bối cảnh: trước story-20 hai hàm này không đọc `tx.Error` ở bất kỳ đâu ⇒ cột không tồn tại
// (MySQL 1054), only_full_group_by (1055), JOIN hỏng, deadlock… đều ra `200 {"items": []}`.
// Bug thật đã bị che rất lâu: `GET /accountant/payment/report-telesale` (story-16 §I.6.2).
//
// ⚠ Lưới này có HAI nửa và phải giữ CẢ HAI:
//   - nửa "lỗi ⇒ KHÔNG 200-rỗng" (AC4)
//   - nửa "0 dòng hợp lệ ⇒ VẪN 200 + `[]`, không phải null, không phải 500" (AC6) — đây là chỗ
//     dễ vá quá tay nhất.
package contracttest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/pagination"
)

// runPaginateRoute chạy 1 handler qua ENGINE gin thật (không phải CreateTestContext trần):
// story-20 niêm phong `c.Writer` sau khi trả 500, nên phải đi qua engine để chứng minh
// `c.JSON(200, result)` mà handler gọi SAU đó thật sự không nối thêm body.
func runPaginateRoute(t *testing.T, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/_s20/list", h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_s20/list?page=1&per-page=20", nil))
	return w
}

// decodeSingleJSON khẳng định body là MỘT object JSON hợp lệ và không còn rác phía sau.
// Đây là bảo chứng cho cơ chế niêm phong writer: nếu chỉ Abort mà không niêm phong thì body sẽ là
// `{"message":…}{"items":[]…}` — hai object dính nhau, decode được cái đầu nhưng còn dư.
func decodeSingleJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	dec := json.NewDecoder(w.Body)
	var obj map[string]interface{}
	if err := dec.Decode(&obj); err != nil {
		t.Fatalf("body không phải JSON object hợp lệ: %v\nbody: %s", err, w.Body.String())
	}
	if dec.More() {
		t.Fatalf("body có NHIỀU HƠN 1 giá trị JSON (writer chưa được niêm phong ⇒ response hỏng)\nbody: %s", w.Body.String())
	}
	return obj
}

// TestPaginateSQLErrorIsNotSwallowed — AC4.
// Query cố tình chiếu cột không tồn tại ⇒ phải KHÔNG còn là 200-rỗng.
func TestPaginateSQLErrorIsNotSwallowed(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	cases := []struct {
		name string
		h    gin.HandlerFunc
	}{
		{
			// Hình dạng ĐÚNG như bug thật report-telesale: SELECT cột đã bị rename/drop ⇒ 1054.
			name: "Paginate_unknown_column_in_select",
			h: func(c *gin.Context) {
				q := testDB.Table("payment").Select("payment.id, payment.cot_khong_ton_tai")
				var items []map[string]interface{}
				result := pagination.Paginate(c, q, &items)
				c.JSON(http.StatusOK, result) // handler chuẩn của repo — CỐ Ý giữ nguyên
			},
		},
		{
			name: "Paginate_unknown_column_in_order",
			h: func(c *gin.Context) {
				q := testDB.Table("payment").Select("payment.id").Order("cot_khong_ton_tai DESC")
				var items []map[string]interface{}
				result := pagination.Paginate(c, q, &items)
				c.JSON(http.StatusOK, result)
			},
		},
		{
			// AC3: COUNT chạy trên derived table `SELECT COUNT(*) FROM (?) AS cnt`. Cột trùng tên
			// là HỢP LỆ với SELECT thường nhưng LỖI (1060 Duplicate column name) khi làm derived
			// table ⇒ đúng ca "COUNT lỗi mà Find không lỗi" mà AC3 đòi phải bắt.
			name: "Paginate_count_only_error_duplicate_column",
			h: func(c *gin.Context) {
				q := testDB.Table("payment").
					Joins("LEFT JOIN customer ON customer.id = payment.customer_id").
					Select("payment.id, customer.id")
				var items []map[string]interface{}
				result := pagination.Paginate(c, q, &items)
				c.JSON(http.StatusOK, result)
			},
		},
		{
			name: "PaginateWithTotal_unknown_column_in_select",
			h: func(c *gin.Context) {
				q := testDB.Table("payment").Select("payment.id, payment.cot_khong_ton_tai")
				var items []map[string]interface{}
				result := pagination.PaginateWithTotal(c, q, &items, 0)
				c.JSON(http.StatusOK, result)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := runPaginateRoute(t, tc.h)

			if w.Code == http.StatusOK {
				t.Fatalf("query LỖI SQL nhưng vẫn trả 200 — lỗi bị nuốt\nbody: %s", w.Body.String())
			}
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("muốn HTTP 500, nhận %d\nbody: %s", w.Code, w.Body.String())
			}

			obj := decodeSingleJSON(t, w)
			if _, ok := obj["message"]; !ok {
				t.Fatalf("response 500 phải có field `message` (response.ServerError + i18n)\nbody: %s", w.Body.String())
			}
			if _, ok := obj["items"]; ok {
				t.Fatalf("response lỗi KHÔNG được chứa `items` — body của handler đã lọt ra\nbody: %s", w.Body.String())
			}
		})
	}
}

// TestPaginateEmptyListStaysOK — AC6 (canh regression "vá quá tay").
// Query CHẠY ĐÚNG và thật sự 0 dòng ⇒ vẫn phải 200 + `items: []` (không null, không 500).
func TestPaginateEmptyListStaysOK(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	newQuery := func(db *gorm.DB) *gorm.DB {
		// Điều kiện không bao giờ khớp ⇒ 0 dòng, nhưng SQL hoàn toàn hợp lệ.
		return db.Table("payment").Select("payment.id, payment.amount").Where("payment.id < 0")
	}

	cases := []struct {
		name string
		h    gin.HandlerFunc
	}{
		{
			name: "Paginate_valid_query_zero_rows",
			h: func(c *gin.Context) {
				q := newQuery(testDB)
				var items []map[string]interface{}
				c.JSON(http.StatusOK, pagination.Paginate(c, q, &items))
			},
		},
		{
			name: "PaginateWithTotal_valid_query_zero_rows",
			h: func(c *gin.Context) {
				q := newQuery(testDB)
				var items []map[string]interface{}
				c.JSON(http.StatusOK, pagination.PaginateWithTotal(c, q, &items, 0))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := runPaginateRoute(t, tc.h)

			if w.Code != http.StatusOK {
				t.Fatalf("list rỗng HỢP LỆ phải là 200, nhận %d\nbody: %s", w.Code, w.Body.String())
			}
			obj := decodeSingleJSON(t, w)

			raw, ok := obj["items"]
			if !ok {
				t.Fatalf("thiếu `items`\nbody: %s", w.Body.String())
			}
			if raw == nil {
				t.Fatalf("`items` là null — phải là `[]` (JSON array rỗng)\nbody: %s", w.Body.String())
			}
			arr, ok := raw.([]interface{})
			if !ok {
				t.Fatalf("`items` là %T — phải là JSON array\nbody: %s", raw, w.Body.String())
			}
			if len(arr) != 0 {
				t.Fatalf("`items` phải rỗng, nhận %d phần tử\nbody: %s", len(arr), w.Body.String())
			}
			if _, ok := obj["_meta"]; !ok {
				t.Fatalf("thiếu `_meta` — contract phân trang phải giữ nguyên\nbody: %s", w.Body.String())
			}
		})
	}
}

//go:build integration

// Epic swallowed-sql-errors-sweep, story-02 (cụm print+export) — lưới FR5 cho `pagination.FailQuery`,
// bản export dùng NGOÀI package `pagination` cho handler map-scan viết tay (`.Find`/`.Scan`/`.Take`
// vào `map`/`[]map`, KHÔNG đi qua `Paginate`/`PaginateWithTotal`).
//
// `FailQuery` tái dùng NGUYÊN mảnh code "response.ServerErrorReadOnly + Abort + niêm phong writer" mà
// `failList` (story-20) đã dùng cho `Paginate` — lưới này thay vì lặp lại lưới `pagination_error_cases_test.go`
// cho từng điểm trong ~40 điểm đã vá ở story-02, kiểm CHÍNH cơ chế dùng chung 1 lần, theo đúng 2 hình
// dạng handler thật đã vá (single-query trả ngay / helper phụ để caller tiếp tục chạy sau đó — double-write
// risk). Vá đúng theo mẫu này ở từng điểm cụ thể do Dev tự kiểm bằng `go build` + review tay (không phải
// lưới runtime riêng cho 40 điểm — quy mô không hợp lý).
package contracttest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/pagination"
)

// runFailQueryRoute — giống runPaginateRoute (pagination_error_cases_test.go) nhưng không ràng buộc
// path/query cụ thể, vì FailQuery không đọc page/per-page.
func runFailQueryRoute(t *testing.T, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/_sweep/item", h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/_sweep/item", nil))
	return w
}

// TestFailQuery_SingleQueryHandler — hình dạng phổ biến nhất đã vá ở story-02
// (vd print/labo_order, print/order_plan): 1 hàm, lỗi ở đâu return luôn.
func TestFailQuery_SingleQueryHandler(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	h := func(c *gin.Context) {
		var item map[string]interface{}
		tx := testDB.Table("customer").Select("id, cot_khong_ton_tai").Where("id = ?", 1).Limit(1).Scan(&item)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	}

	w := runFailQueryRoute(t, h)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("query LỖI SQL nhưng không trả 500 — lỗi bị nuốt, nhận %d\nbody: %s", w.Code, w.Body.String())
	}
	obj := decodeSingleJSON(t, w)
	if _, ok := obj["message"]; !ok {
		t.Fatalf("response 500 phải có field `message`\nbody: %s", w.Body.String())
	}
	if _, ok := obj["data"]; ok {
		t.Fatalf("response lỗi KHÔNG được chứa `data` — body handler đã lọt ra\nbody: %s", w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"cot_khong_ton_tai", "Unknown column", "SQLSTATE"} {
		if strings.Contains(body, leak) {
			t.Fatalf("message rò rỉ mảnh SQL/lỗi MySQL thô (%q)\nbody: %s", leak, body)
		}
	}
}

// TestFailQuery_HelperContinuesAfterFail — hình dạng "helper phụ, caller vẫn chạy tiếp sau đó" đã
// vá ở story-02 (vd print/reception_payment attachRelation → PrintReceptionPayment vẫn tính
// totalAmount + c.JSON sau khi attachRelation đã gọi FailQuery). Đây là ca double-write risk thật —
// PHẢI niêm phong writer, không chỉ Abort, nếu không body sẽ dính 2 JSON.
func TestFailQuery_HelperContinuesAfterFail(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	attachRelation := func(c *gin.Context) {
		var rows []map[string]interface{}
		tx := testDB.Table("customer").Select("id, cot_khong_ton_tai_2").Find(&rows)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
	}

	h := func(c *gin.Context) {
		attachRelation(c)
		// Handler CHUẨN của story-02 vẫn chạy tiếp (tính tổng, gọi c.JSON) — sealedWriter phải
		// nuốt lần ghi thứ 2 này, không được nối thêm vào body.
		total := 0
		_ = total
		c.JSON(http.StatusOK, gin.H{"object": gin.H{"items": []int{}}, "totalAmount": 0})
	}

	w := runFailQueryRoute(t, h)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("muốn 500 (lỗi từ attachRelation), nhận %d\nbody: %s", w.Code, w.Body.String())
	}
	obj := decodeSingleJSON(t, w) // Fatal nếu có > 1 JSON value trong body — chứng minh niêm phong hoạt động
	if _, ok := obj["object"]; ok {
		t.Fatalf("response lỗi KHÔNG được chứa `object` — c.JSON thứ 2 của handler đã lọt ra\nbody: %s", w.Body.String())
	}
	if _, ok := obj["totalAmount"]; ok {
		t.Fatalf("response lỗi KHÔNG được chứa `totalAmount` — c.JSON thứ 2 của handler đã lọt ra\nbody: %s", w.Body.String())
	}
}

// TestFailQuery_ValidEmptyResultStaysOK — canh regression AC6-style: query HỢP LỆ 0 dòng KHÔNG được
// đi qua FailQuery (không gọi tới nó) — endpoint vẫn 200 bình thường như trước khi vá.
func TestFailQuery_ValidEmptyResultStaysOK(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	h := func(c *gin.Context) {
		var item map[string]interface{}
		tx := testDB.Table("customer").Select("id, fullname").Where("id = ?", -1).Limit(1).Scan(&item)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": item})
	}

	w := runFailQueryRoute(t, h)

	if w.Code != http.StatusOK {
		t.Fatalf("query hợp lệ 0 dòng phải là 200, nhận %d\nbody: %s", w.Code, w.Body.String())
	}
	decodeSingleJSON(t, w)
}

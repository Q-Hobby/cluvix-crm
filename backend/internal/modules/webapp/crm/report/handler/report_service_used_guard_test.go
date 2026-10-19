package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/i18n"
)

// Neo guard dải ngày của GET /crm/order-service-used/report-service-used (2026-09-09).
// Không có dải `created_day` ⇒ aggregate TRỌN bảng `order_service_used`, 3 lượt quét mỗi lần
// mở màn (đo ~14–18s @898.720 dòng, KỂ CẢ sau khi đã vá GROUP BY/COUNT/gộp totals). Guard chạy
// TRƯỚC mọi truy vấn nên test không cần DB — `Deps{DB: nil}` cố ý: chạm DB là panic ⇒ đỏ.
func TestReportServiceUsed_DateRangeGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := New(Deps{DB: nil})

	const day = int64(86400)
	base := int64(1788195600) // 2026-09-01 00:00 +07

	cases := []struct {
		name string
		qs   string
		msg  string
	}{
		{"thiếu cả hai đầu (nút × xoá ngày trên FE)", "", "report_date_range_required"},
		{"chỉ có đầu gte", "filter[created_day][gte]=1788195600", "report_date_range_required"},
		{"chỉ có đầu lte", "filter[created_day][lte]=1788195600", "report_date_range_required"},
		{"giá trị rác", "filter[created_day][gte]=abc&filter[created_day][lte]=1788195600", "report_date_range_required"},
		{"đảo đầu cuối", "filter[created_day][gte]=1788195600&filter[created_day][lte]=1757350800", "report_date_range_required"},
		{"vượt trần 1 năm 1 ngày", rsuRangeQS(base, base+define.ReportMaxRangeSec+day), "report_date_range_max_one_year"},
		// Guard phải chặn CẢ đường export — export chạy đúng câu này qua con trỏ.
		{"export cũng bị chặn", "_export=1", "report_date_range_required"},
		{"export vượt trần", "_export=1&" + rsuRangeQS(base, base+define.ReportMaxRangeSec+day), "report_date_range_max_one_year"},
		// `created_at` là bộ lọc phụ, KHÔNG thay được `created_day` cho guard.
		{"chỉ lọc created_at không cứu được", "filter[created_at][gte]=1788195600&filter[created_at][lte]=1788282000", "report_date_range_required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/x?"+tc.qs, nil)

			h.ReportServiceUsed(c)

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("HTTP = %d, muốn 422 (body: %s)", w.Code, w.Body.String())
			}
			var body struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("body không phải JSON: %v", err)
			}
			want := i18n.T(c, tc.msg)
			if want == "" || want == tc.msg {
				t.Fatalf("khoá i18n %q chưa có bản dịch", tc.msg)
			}
			if body.Message != want {
				t.Fatalf("message = %q, muốn %q", body.Message, want)
			}
		})
	}
}

// Đối chứng: dải ngày ĐÚNG trần phải đi tiếp tới truy vấn — với DB nil, "đi tiếp" biểu hiện
// bằng panic. Không có ca này thì 9 ca 422 ở trên vẫn xanh kể cả khi handler luôn trả 422.
func TestReportServiceUsed_GuardChoQuaDungTran(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("dải ngày đúng trần đáng lẽ đi tiếp tới truy vấn (DB nil ⇒ panic), nhưng không")
		}
	}()
	h := New(Deps{DB: nil})
	base := int64(1788195600)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+rsuRangeQS(base, base+define.ReportMaxRangeSec), nil)
	h.ReportServiceUsed(c)
}

// Khuôn `<key>[gte]` (không bọc `filter[...]`) cũng phải được chấp nhận — `getOperatorValue`
// của pkg/query đọc cả hai, guard không được hẹp hơn bộ lọc mà nó gác.
func TestReportServiceUsed_GuardChapNhanCaHaiKhuonParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("khuôn `created_day[gte]` đáng lẽ được chấp nhận và đi tiếp tới truy vấn")
		}
	}()
	h := New(Deps{DB: nil})
	base := int64(1788195600)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet,
		"/x?created_day[gte]="+strconv.FormatInt(base, 10)+"&created_day[lte]="+strconv.FormatInt(base+86400, 10), nil)
	h.ReportServiceUsed(c)
}

func rsuRangeQS(from, to int64) string {
	return "filter[created_day][gte]=" + strconv.FormatInt(from, 10) +
		"&filter[created_day][lte]=" + strconv.FormatInt(to, 10)
}

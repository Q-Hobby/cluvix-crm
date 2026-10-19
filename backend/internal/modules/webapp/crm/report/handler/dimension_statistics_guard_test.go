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

// Neo guard dải ngày của `dimensionStatistics` (2026-09-09) — hàm dùng chung cho các endpoint
// `*-statistics` (5; `pathology-statistics`/`campain-event-statistics` đã BỎ HẲN route+handler ở
// story-48 crm-standalone: dimension bệnh lý/chiến dịch không còn tồn tại trong
// schema, xem declinic-plan.md mục 4; story-50 dọn nốt các JOIN/field chiến dịch còn sót).
// Guard đặt trong hàm chung nên phủ tất cả còn lại; test chạy trên toàn bộ để nếu ai đó tách một
// endpoint ra khỏi hàm chung thì lưới vẫn đỏ.
// Guard chạy TRƯỚC mọi truy vấn ⇒ `Deps{DB: nil}` cố ý: chạm DB là panic.
func TestDimensionStatistics_DateRangeGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := New(Deps{DB: nil})

	endpoints := map[string]func(*gin.Context){
		"source-statistics":     h.SourceStatistics,
		"province-statistics":   h.ProvinceStatistics,
		"country-statistics":    h.CountryStatistics,
		"category-statistics":   h.CategoryStatistics,
		"interested-statistics": h.InterestedStatistics,
	}
	const day = int64(86400)
	base := int64(1788195600)
	cases := []struct{ name, qs, msg string }{
		{"thiếu cả hai đầu", "", "report_date_range_required"},
		{"chỉ có đầu gte", "filter[created_at][gte]=1788195600", "report_date_range_required"},
		{"giá trị rác", "filter[created_at][gte]=abc&filter[created_at][lte]=1788195600", "report_date_range_required"},
		{"đảo đầu cuối", "filter[created_at][gte]=1788195600&filter[created_at][lte]=1757350800", "report_date_range_required"},
		{"vượt trần 1 năm 1 ngày", dimRangeQS(base, base+define.ReportMaxRangeSec+day), "report_date_range_max_one_year"},
	}

	for epName, fn := range endpoints {
		for _, tc := range cases {
			t.Run(epName+"/"+tc.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodGet, "/x?"+tc.qs, nil)

				fn(c)

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
}

// Đối chứng: đúng trần thì đi tiếp tới truy vấn (DB nil ⇒ panic). Thiếu ca này thì 40 ca 422
// ở trên vẫn xanh kể cả khi hàm luôn trả 422.
func TestDimensionStatistics_GuardChoQuaDungTran(t *testing.T) {
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
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+dimRangeQS(base, base+define.ReportMaxRangeSec), nil)
	h.SourceStatistics(c)
}

func dimRangeQS(from, to int64) string {
	return "filter[created_at][gte]=" + strconv.FormatInt(from, 10) +
		"&filter[created_at][lte]=" + strconv.FormatInt(to, 10)
}

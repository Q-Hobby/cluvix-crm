package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/i18n"

	"github.com/gin-gonic/gin"
)

// Neo guard dải ngày của GET /crm/report/appointment-statistics (2026-09-09).
// Trước đây thiếu ngày ⇒ handler tự đặt from=0/to=9999999999 ⇒ quét cạn `appointment`
// (đo @1.090.355 lịch hẹn: 2.729.935 dòng InnoDB). Guard chạy TRƯỚC mọi truy vấn nên
// test này không cần DB — Deps{DB: nil} cố ý, chạm DB là panic và test đỏ.
func TestAppointmentStatistics_DateRangeGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := New(Deps{DB: nil})

	const day = int64(86400)
	base := int64(1788195600) // 2026-09-01 00:00 +07

	cases := []struct {
		name string
		qs   string
		msg  string
	}{
		{"thiếu cả hai đầu (nút xoá ngày trên FE)", "", "report_date_range_required"},
		{"chỉ có đầu gte", "_created_at[gte]=1788195600", "report_date_range_required"},
		{"chỉ có đầu lte", "_created_at[lte]=1788195600", "report_date_range_required"},
		{"giá trị rác", "_created_at[gte]=abc&_created_at[lte]=1788195600", "report_date_range_required"},
		{"đảo đầu cuối", "_created_at[gte]=1788195600&_created_at[lte]=1757350800", "report_date_range_required"},
		{"vượt trần 1 năm 1 ngày", rangeQS(base, base+define.ReportMaxRangeSec+day), "report_date_range_max_one_year"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/x?"+tc.qs, nil)

			h.AppointmentStatistics(c)

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

// TestAppointmentStatistics_GuardChặnTrướcKhiChạmDB — dải ngày ĐÚNG trần thì guard phải cho
// qua; với DB nil, "cho qua" biểu hiện bằng panic ở bước truy vấn. Đây là đối chứng chứng
// minh 6 ca 422 ở trên bị chặn thật, không phải vì handler luôn trả 422.
func TestAppointmentStatistics_GuardChoQuaDungTran(t *testing.T) {
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
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+rangeQS(base, base+define.ReportMaxRangeSec), nil)
	h.AppointmentStatistics(c)
}

func rangeQS(from, to int64) string {
	return "_created_at[gte]=" + strconv.FormatInt(from, 10) +
		"&_created_at[lte]=" + strconv.FormatInt(to, 10)
}

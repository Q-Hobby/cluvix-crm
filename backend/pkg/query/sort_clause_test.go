package query

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Neo `SortClause` + `ReverseOrderClause` (tách ra 2026-09-16 cho phân trang sâu đọc ngược).
//
// `SortClause` phải trả ĐÚNG mệnh đề mà `ApplySort` áp — hai bên lệch nhau nghĩa là câu tra id và
// câu nạp dữ liệu sắp khác nhau ⇒ trang trả nhầm dòng mà không có lỗi nào bật ra.

func sortCtx(t *testing.T, rawQuery string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x?"+rawQuery, nil)
	return c
}

func TestSortClause(t *testing.T) {
	const def = "tct.confirm_time DESC"
	allowed := []string{"tct.confirm_time", "tct.id"}

	cases := []struct{ ten, qs, want string }{
		{"không có sort ⇒ mặc định + tiebreaker", "", "tct.confirm_time DESC, tct.id DESC"},
		{"sort tăng dần", "sort=confirm_time", "tct.confirm_time ASC, tct.id DESC"},
		{"sort giảm dần", "sort=-confirm_time", "tct.confirm_time DESC, tct.id DESC"},
		{"sort theo chính pk ⇒ không nhân đôi tiebreaker", "sort=-id", "tct.id DESC"},
		{"sort ngoài whitelist ⇒ rơi về mặc định", "sort=zzz", "tct.confirm_time DESC, tct.id DESC"},
	}
	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			if got := SortClause(sortCtx(t, tc.qs), def, allowed...); got != tc.want {
				t.Fatalf("SortClause = %q, muốn %q", got, tc.want)
			}
		})
	}
}

func TestReverseOrderClause(t *testing.T) {
	cases := []struct{ in, want string }{
		{"tct.confirm_time DESC, tct.id DESC", "tct.confirm_time ASC, tct.id ASC"},
		{"tct.confirm_time ASC, tct.id DESC", "tct.confirm_time DESC, tct.id ASC"},
		{"tct.id DESC", "tct.id ASC"},
		{"tct.id", "tct.id DESC"}, // không ghi chiều = ASC
		// Không đảo được ⇒ "" ⇒ caller phải đi đường thường, KHÔNG đoán bừa.
		{"SUM(tct.point) DESC", ""},
		{"tct.confirm_time DESC, COUNT(*) ASC", ""},
		{"tct.confirm_time RANDOM", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ReverseOrderClause(tc.in); got != tc.want {
			t.Errorf("ReverseOrderClause(%q) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}

// Đảo hai lần phải về đúng mệnh đề ban đầu — nếu không, đường đọc ngược đang đổi nghĩa thứ tự.
func TestReverseOrderClause_DaoHaiLan(t *testing.T) {
	for _, in := range []string{"tct.confirm_time DESC, tct.id DESC", "tct.confirm_time ASC, tct.id DESC"} {
		if got := ReverseOrderClause(ReverseOrderClause(in)); got != in {
			t.Errorf("đảo 2 lần %q ra %q", in, got)
		}
	}
}

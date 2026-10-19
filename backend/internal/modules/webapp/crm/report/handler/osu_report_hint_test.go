package handler

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Neo bản vá hiệu năng 2026-10-19 của `report-service-used` + `service-type-statistics` (bench 982.900
// dòng osu — số đo ở osuLockedReportTable / serviceTypeStatsQuery). GORM DryRun: không đụng DB nào.
//
// Ba thứ khoá ở đây trả ĐÚNG cùng kết quả khi bị revert (chỉ đổi plan) ⇒ test so số không bắt được:
//   - cổng hint theo dải ngày (FORCE khi ≤ 31 ngày, IGNORE khi rộng/không lọc, không hint khi dạng lạ);
//   - JOIN `customer` chỉ khi có bộ lọc đọc cột customer;
//   - service-type gom theo khoá số `service.service_type_id` (không `GROUP BY st.id, st.name`).

func hintCtx(t *testing.T, rawQuery string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x?"+rawQuery, nil)
	c.Set("branchActiveId", uint(1))
	c.Set("branchIds", []uint{1, 2})
	return c
}

const (
	hintMonth = "filter[created_day][gte]=1790787600&filter[created_day][lte]=1792429199" // 19 ngày
	hintYear  = "filter[created_day][gte]=1760893200&filter[created_day][lte]=1792429199" // 365 ngày
)

func TestOsuLockedReportTable_CongTheoDaiNgay(t *testing.T) {
	force := "FORCE INDEX (" + osuDayCoverIndex + ")"
	ignore := "IGNORE INDEX (" + osuWideIgnoreIndexes + ")"
	cases := []struct {
		name, q, want string
	}{
		{"tháng, khuôn filter[]", hintMonth, force},
		{"tháng, khuôn tiền tố _", "created_day[gte]=1790787600&created_day[lte]=1792429199", force},
		{"đúng 31 ngày", "filter[created_day][gte]=1000&filter[created_day][lte]=" + itoa(1000+osuHintNarrowRangeSec), force},
		{"31 ngày + 1 giây", "filter[created_day][gte]=1000&filter[created_day][lte]=" + itoa(1001+osuHintNarrowRangeSec), ignore},
		{"1 năm", hintYear, ignore},
		{"không lọc ngày", "", ignore},
		{"không lọc ngày nhưng có bộ lọc khác", "filter[branch_id][0]=1", ignore},
		{"chỉ 1 cận", "filter[created_day][gte]=1790787600", ""},
		{"eq", "filter[created_day]=1790787600", ""},
		{"không phải số", "filter[created_day][gte]=abc&filter[created_day][lte]=1792429199", ""},
	}
	for _, tc := range cases {
		got := osuLockedReportTable(hintCtx(t, tc.q), "osu")
		if tc.want == "" {
			if got != "order_service_used osu" {
				t.Errorf("%s: không được hint, got %q", tc.name, got)
			}
			continue
		}
		if got != "order_service_used osu "+tc.want {
			t.Errorf("%s: want hint %q, got %q", tc.name, tc.want, got)
		}
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func rsuSQL(t *testing.T, rawQuery string) string {
	t.Helper()
	h := New(Deps{DB: dryRunDBStats(t)})
	var rows []map[string]interface{}
	return h.buildReportServiceUsedQuery(hintCtx(t, rawQuery)).Session(&gorm.Session{DryRun: true}).
		Find(&rows).Statement.SQL.String()
}

func TestReportServiceUsed_JoinCustomerChiKhiCanLoc(t *testing.T) {
	if s := rsuSQL(t, hintMonth); strings.Contains(s, "JOIN customer") {
		t.Fatalf("không lọc theo cột customer thì KHÔNG được JOIN customer (tra PK ~1,8s/câu @454k dòng).\nSQL: %s", s)
	} else if !strings.Contains(s, "FORCE INDEX ("+osuDayCoverIndex+")") {
		t.Fatalf("dải tháng phải ép %s.\nSQL: %s", osuDayCoverIndex, s)
	}
	for _, q := range []string{
		hintMonth + "&filter[source_id][0]=3",
		hintMonth + "&filter[source_id]=3",
		hintMonth + "&filter[source_id][is]=1",
		hintMonth + "&_groupLocal[0]=1",
		hintMonth + "&_groupCountry[0]=1",
	} {
		if s := rsuSQL(t, q); !strings.Contains(s, "LEFT JOIN customer ON customer.id = order_service_used.customer_id") {
			t.Fatalf("%s: bộ lọc đọc cột customer ⇒ PHẢI JOIN customer.\nSQL: %s", q, s)
		}
	}
}

func TestServiceTypeStats_GomTheoKhoaSoRoiMoiJoinTen(t *testing.T) {
	h := New(Deps{DB: dryRunDBStats(t)})
	var rows []map[string]interface{}
	s := h.serviceTypeStatsQuery(hintCtx(t, "")).Session(&gorm.Session{DryRun: true}).
		Find(&rows).Statement.SQL.String()
	bare := strings.ReplaceAll(s, "`", "")
	for _, want := range []string{
		"GROUP BY service.service_type_id WITH ROLLUP",
		"LEFT JOIN service_type st ON st.id = a.tid",
		// không lọc ngày ⇒ hash join: danh mục đứng trước, osu mất mọi chỉ mục theo service_id
		"service_type st STRAIGHT_JOIN service ON service.service_type_id = st.id STRAIGHT_JOIN order_service_used osu IGNORE INDEX",
		"osu.branch_id", // scope tenant nằm TRONG bảng dẫn xuất
		"IGNORE INDEX (" + strings.ReplaceAll(osuWideIgnoreIndexes, "`", "") + ")",
	} {
		if !strings.Contains(bare, want) {
			t.Fatalf("SQL thiếu %q.\nSQL: %s", want, s)
		}
	}
	if strings.Contains(bare, "GROUP BY st.id") {
		t.Fatalf("GROUP BY theo cột service_type (khoá VARCHAR) đã bị đưa lại — sort 858k dòng 11,7s.\nSQL: %s", s)
	}
}

// Dải hẹp (FORCE range) giữ osu dẫn + tra PK danh mục; dải rộng đảo sang hash join.
func TestServiceTypeStats_ThuTuJoinTheoDaiNgay(t *testing.T) {
	h := New(Deps{DB: dryRunDBStats(t)})
	if s := statsSQL(t, h, hintCtx(t, hintMonth)); !strings.Contains(s, "FROM order_service_used osu FORCE INDEX ("+osuDayCoverIndex+") STRAIGHT_JOIN service ON service.id = osu.service_id") {
		t.Fatalf("dải tháng: osu phải dẫn (FORCE range) rồi tra PK service.\nSQL: %s", s)
	}
	if s := statsSQL(t, h, hintCtx(t, hintYear)); !strings.Contains(s, "FROM service_type st STRAIGHT_JOIN service") {
		t.Fatalf("dải 1 năm: phải đảo sang hash join (danh mục đứng trước).\nSQL: %s", s)
	}
	// Dạng lọc ngày lạ (không hint) ⇒ KHÔNG được đảo: osu còn fk service_id ⇒ plan lật 5s.
	if s := statsSQL(t, h, hintCtx(t, "filter[created_day][gte]=1790787600")); !strings.Contains(s, "FROM order_service_used osu STRAIGHT_JOIN service") {
		t.Fatalf("không hint thì osu phải dẫn.\nSQL: %s", s)
	}
}

// Đường 1 lượt: gom osu TRƯỚC, không JOIN danh mục/khách cho từng dòng khi bộ lọc không cần.
func TestReportServiceUsed_RollupChiJoinKhiCan(t *testing.T) {
	h := New(Deps{DB: dryRunDBStats(t)})
	sqlOf := func(q string) string {
		var rows []map[string]interface{}
		return strings.ReplaceAll(h.reportServiceUsedRollupQuery(hintCtx(t, q)).Session(&gorm.Session{DryRun: true}).
			Find(&rows).Statement.SQL.String(), "`", "")
	}
	inner := func(s string) string { return s[strings.Index(s, "FROM (")+6 : strings.Index(s, ") AS a")] }
	s := sqlOf(hintYear)
	for _, want := range []string{"GROUP BY order_service_used.service_id WITH ROLLUP", "LEFT JOIN service ON service.id = a.id", "order_service_used.branch_id"} {
		if !strings.Contains(s, want) {
			t.Fatalf("thiếu %q.\nSQL: %s", want, s)
		}
	}
	if in := inner(s); strings.Contains(in, "JOIN") {
		t.Fatalf("không lọc theo service/customer thì bảng dẫn xuất KHÔNG được JOIN gì.\nSQL: %s", in)
	}
	if in := inner(sqlOf(hintYear + "&_keyword=a")); !strings.Contains(in, "LEFT JOIN service ON service.id = order_service_used.service_id") {
		t.Fatalf("có từ khoá ⇒ phải JOIN service trong bảng dẫn xuất.\nSQL: %s", in)
	}
	if in := inner(sqlOf(hintYear + "&filter[service_type_id][0]=1&filter[source_id][0]=2")); !strings.Contains(in, "JOIN service") || !strings.Contains(in, "JOIN customer") {
		t.Fatalf("lọc loại dịch vụ + nguồn ⇒ phải JOIN service + customer.\nSQL: %s", in)
	}
}

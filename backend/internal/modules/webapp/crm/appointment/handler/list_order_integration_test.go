//go:build integration

package handler

// Neo cho appointmentListPageHint + index idx_appointment_date_id: hai thứ đó chỉ được đổi PLAN,
// không được đổi TẬP DÒNG hay THỨ TỰ. Kỳ vọng tính ĐỘC LẬP với SQL: lấy tập dòng trong phạm vi
// (không ORDER BY) rồi sắp bằng Go theo đúng luật ORDER BY + tie-breaker id DESC, so với các trang
// handler trả về nối lại (per-page nhỏ ⇒ đi qua cả nhánh OFFSET).

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/tz"
)

type listRow struct {
	ID   int64
	Date int64
}

func listAllPages(t *testing.T, branchActive uint, branchIDs []uint, params url.Values) ([]int64, int64) {
	t.Helper()
	const perPage = 4
	var ids []int64
	var total int64
	for page := 1; page <= 100; page++ {
		p := url.Values{}
		for k, v := range params {
			p[k] = v
		}
		p.Set("page", fmt.Sprint(page))
		p.Set("per-page", fmt.Sprint(perPage))
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/crm/appointment?"+p.Encode(), nil)
		if branchActive > 0 {
			c.Set("branchActiveId", branchActive)
		}
		c.Set("branchIds", branchIDs)
		c.Set("userID", uint(1))
		(&Handler{DB: testDB}).ListAppointment(c)
		require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
		var body struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
			Meta struct {
				TotalCount int64 `json:"totalCount"`
			} `json:"_meta"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
		total = body.Meta.TotalCount
		for _, it := range body.Items {
			ids = append(ids, it.ID)
		}
		if len(body.Items) < perPage {
			break
		}
	}
	return ids, total
}

func TestListAppointment_ThuTuKhongDoi(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	brA := tn.BranchID
	brB := testhelpers.CreateBranch(t, testDB)
	loc := tz.LocFromContext(nil, testDB)

	// 5 mốc ngày, mỗi mốc nhiều lịch TRÙNG ngày giờ (để tie-breaker id quyết định), xen chi nhánh
	// A / NULL / B (B nằm ngoài phạm vi khi active A).
	base := time.Date(2039, 5, 1, 0, 0, 0, 0, loc)
	dates := []int64{
		base.Add(9 * time.Hour).Unix(), base.Add(9 * time.Hour).Unix(),
		base.AddDate(0, 0, 3).Unix(), base.Add(-1 * time.Hour).Unix(), base.AddDate(0, 1, 0).Unix(),
	}
	branches := []any{brA, nil, brB, brA, brA, nil, brA}
	for i := 0; i < 23; i++ {
		seedAppointment(t, tn, map[string]any{
			"appointment_date": dates[i%len(dates)],
			"branch_id":        branches[i%len(branches)],
		})
	}

	from, to := base.AddDate(0, 0, -1).Unix(), base.AddDate(0, 0, 10).Unix()
	inScope := func(scopeIDs []uint, withDate bool) []listRow {
		q := testDB.Table("appointment").Select("id, appointment_date").
			Where("(branch_id IN ? OR branch_id IS NULL)", scopeIDs)
		if withDate {
			q = q.Where("appointment_date >= ? AND appointment_date <= ?", from, to)
		}
		var rows []struct {
			ID              int64
			AppointmentDate int64
		}
		require.NoError(t, q.Find(&rows).Error)
		out := make([]listRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, listRow{r.ID, r.AppointmentDate})
		}
		return out
	}
	sorted := func(rows []listRow, less func(a, b listRow) bool) []int64 {
		sort.Slice(rows, func(i, j int) bool { return less(rows[i], rows[j]) })
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		return ids
	}
	dateAscIDDesc := func(a, b listRow) bool {
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.ID > b.ID
	}
	dateDescIDDesc := func(a, b listRow) bool {
		if a.Date != b.Date {
			return a.Date > b.Date
		}
		return a.ID > b.ID
	}
	idDesc := func(a, b listRow) bool { return a.ID > b.ID }

	dateFilter := url.Values{
		"filter[appointment_date][gte]": {fmt.Sprint(from)},
		"filter[appointment_date][lte]": {fmt.Sprint(to)},
	}
	withSort := func(v url.Values, s string) url.Values {
		out := url.Values{}
		for k, vv := range v {
			out[k] = vv
		}
		if s != "" {
			out.Set("sort", s)
		}
		return out
	}

	cases := []struct {
		name     string
		params   url.Values
		withDate bool
		less     func(a, b listRow) bool
	}{
		{"mặc định FE: sort=appointment_date + lọc ngày", withSort(dateFilter, "appointment_date"), true, dateAscIDDesc},
		{"sort=appointment_date, không lọc ngày", withSort(url.Values{}, "appointment_date"), false, dateAscIDDesc},
		{"sort=-appointment_date + lọc ngày", withSort(dateFilter, "-appointment_date"), true, dateDescIDDesc},
		{"không sort (id DESC), không lọc ngày", url.Values{}, false, idDesc},
	}
	// 2 phạm vi: nhân viên 1 chi nhánh (branchActive = A ⇒ `branch_id = A OR IS NULL`) và admin nhiều
	// chi nhánh (không active, branchIds = [A, B] ⇒ `IN (A, B) OR IS NULL`).
	scopes := []struct {
		name     string
		active   uint
		ids      []uint
		expectIn []uint
	}{
		{"1 chi nhánh", brA, []uint{brA, brB}, []uint{brA}},
		{"nhiều chi nhánh", 0, []uint{brA, brB}, []uint{brA, brB}},
	}
	for _, sc := range scopes {
		for _, tc := range cases {
			t.Run(sc.name+"/"+tc.name, func(t *testing.T) {
				want := sorted(inScope(sc.expectIn, tc.withDate), tc.less)
				require.NotEmpty(t, want, "đối chứng: fixture phải có dòng trong phạm vi")
				got, total := listAllPages(t, sc.active, sc.ids, tc.params)
				require.Equal(t, want, got, "tập dòng / thứ tự trang lệch luật ORDER BY")
				require.EqualValues(t, len(want), total, "_meta.totalCount lệch")
			})
		}
	}
}

// appointmentListCountQuery chỉ được đổi bản SAO: COUNT không ORDER BY, câu trang vẫn giữ ORDER BY.
func TestAppointmentListCountQuery_KhongDungQueryTrang(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/crm/appointment?sort=appointment_date", nil)
	c.Set("branchActiveId", uint(1))
	q := (&Handler{DB: testDB}).BuildAppointmentQuery(c)
	var total int64
	countSQL := testDB.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Raw("SELECT COUNT(*) FROM (?) AS cnt", appointmentListCountQuery(q)).Scan(&total)
	})
	var items []map[string]any
	// Dựng SAU câu COUNT: chứng minh appointmentListCountQuery không xoá ORDER BY của chính `q`.
	pageSQL := q.Session(&gorm.Session{DryRun: true}).Limit(15).Find(&items).Statement.SQL.String()
	require.NotContains(t, countSQL, "ORDER BY", countSQL)
	require.Contains(t, countSQL, "SELECT appointment.id FROM", countSQL)
	require.Contains(t, pageSQL, "ORDER BY appointment.appointment_date ASC, appointment.id DESC", pageSQL)
}

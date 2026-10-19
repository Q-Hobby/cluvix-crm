//go:build integration

package handler

// Viết lại AppointmentOverview (bench 2026-10-19: bản cũ 4 self-join không bao giờ trả kết quả ở
// 1,2M lịch hẹn). Bộ test này chứng minh bản mới cho ĐÚNG kết quả của bản cũ:
//   - số đếm tay trên fixture có đủ ca khó (cờ tính trên CẢ năm chứ không riêng tháng, lịch gốc
//     chi nhánh khác, cờ ở chi nhánh khác, chi nhánh NULL, is_accepted NULL, rìa tháng theo múi giờ,
//     lịch ngoài năm, khách NULL, khách trùng nhiều lịch gốc trong 1 tháng);
//   - và BẰNG output của chính câu SQL cũ (giữ nguyên văn ở oldAppointmentOverviewSQL).
// Khác biệt CÓ CHỦ ĐÍCH duy nhất (mốc cuối nay lấy trọn ngày cuối) neo riêng ở
// TestAppointmentOverview_NgayCuoiNam.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/tz"
)

// ovYear — năm riêng của fixture (khác năm hiện tại ⇒ khoảng = trọn năm, không dính "tới hết tháng
// này"), đủ xa để dữ liệu test khác không lọt vào.
const ovYear = 2037

type ovRow struct {
	Thang, LichHen, LichHuy, DongY, KhongDen, KhongDongY int64
}

func toOvRows(rows []map[string]any) []ovRow {
	out := make([]ovRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ovRow{
			Thang:      helpers.ToInt64(r["thang_lh"]),
			LichHen:    helpers.ToInt64(r["lich_hen"]),
			LichHuy:    helpers.ToInt64(r["lich_huy"]),
			DongY:      helpers.ToInt64(r["dong_y"]),
			KhongDen:   helpers.ToInt64(r["khong_den"]),
			KhongDongY: helpers.ToInt64(r["khong_dong_y"]),
		})
	}
	return out
}

// ovCtx — gin context mang phạm vi chi nhánh (branchActive = 0 / branchIDs nil ⇒ không đặt khoá).
func ovCtx(branchActive uint, branchIDs []uint, year int) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", fmt.Sprintf("/crm/appointment/appointment-overview?year=%d", year), nil)
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
	}
	if branchIDs != nil {
		c.Set("branchIds", branchIDs)
	}
	return c, w
}

// newOverview gọi handler production.
func newOverview(t *testing.T, branchActive uint, branchIDs []uint, year int) []ovRow {
	t.Helper()
	c, w := ovCtx(branchActive, branchIDs, year)
	(&Handler{DB: testDB}).AppointmentOverview(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows), "body: %s", w.Body.String())
	return toOvRows(rows)
}

// oldAppointmentOverviewSQL — NGUYÊN VĂN câu SQL của bản cũ (trước 2026-10-19), chỉ để đối chiếu.
// startDate/endDate là chuỗi ngày theo giờ thương hiệu như bản cũ truyền vào.
func oldAppointmentOverviewSQL(c *gin.Context, startDate, endDate string) (string, []any) {
	aliasScope := func(alias string) (string, []any) { return buildAppointmentBranchScopeSQL(c, alias) }
	huyScope, huyArgs := aliasScope("huy")
	dongyScope, dongyArgs := aliasScope("dongy")
	kdScope, kdArgs := aliasScope("khongden")
	kdyScope, kdyArgs := aliasScope("khongdongy")
	aScope, aArgs := aliasScope("a")

	offset := tz.Offset(tz.LocFromContext(c, testDB))
	dtA := tz.SqlFromUnix("a.appointment_date", offset)
	dtHuy := tz.SqlFromUnix("huy.appointment_date", offset)
	dtDongy := tz.SqlFromUnix("dongy.appointment_date", offset)
	dtKd := tz.SqlFromUnix("khongden.appointment_date", offset)
	dtKdy := tz.SqlFromUnix("khongdongy.appointment_date", offset)

	sql := fmt.Sprintf(`SELECT
		MONTH(`+dtA+`) AS thang_lh,
		COUNT(DISTINCT a.customer_id) AS lich_hen,
		COUNT(DISTINCT CASE WHEN huy.customer_id IS NOT NULL THEN huy.customer_id END) AS lich_huy,
		COUNT(DISTINCT CASE WHEN dongy.customer_id IS NOT NULL THEN dongy.customer_id END) AS dong_y,
		COUNT(DISTINCT CASE WHEN khongden.customer_id IS NOT NULL THEN khongden.customer_id END) AS khong_den,
		COUNT(DISTINCT CASE WHEN khongdongy.customer_id IS NOT NULL THEN khongdongy.customer_id END) AS khong_dong_y
	FROM appointment a
	LEFT JOIN appointment huy
		ON huy.customer_id = a.customer_id AND huy.status = 4
		AND `+dtHuy+` BETWEEN ? AND ?%s
	LEFT JOIN appointment dongy
		ON dongy.customer_id = a.customer_id AND dongy.is_accepted = 1 AND dongy.status IN (2,3)
		AND `+dtDongy+` BETWEEN ? AND ?%s
	LEFT JOIN appointment khongden
		ON khongden.customer_id = a.customer_id AND khongden.status = 1
		AND `+dtKd+` BETWEEN ? AND ?%s
	LEFT JOIN appointment khongdongy
		ON khongdongy.customer_id = a.customer_id AND khongdongy.is_accepted != 1 AND khongdongy.status IN (2,3)
		AND `+dtKdy+` BETWEEN ? AND ?%s
	WHERE a.appointment_type_id = 1
		AND `+dtA+` BETWEEN ? AND ?%s
	GROUP BY MONTH(`+dtA+`)
	ORDER BY thang_lh DESC`, huyScope, dongyScope, kdScope, kdyScope, aScope)

	args := make([]any, 0)
	for _, sa := range [][]any{huyArgs, dongyArgs, kdArgs, kdyArgs, aArgs} {
		args = append(args, startDate, endDate)
		args = append(args, sa...)
	}
	return sql, args
}

func oldOverview(t *testing.T, branchActive uint, branchIDs []uint, year int) []ovRow {
	t.Helper()
	c, _ := ovCtx(branchActive, branchIDs, year)
	sql, args := oldAppointmentOverviewSQL(c, fmt.Sprintf("%d-01-01", year), fmt.Sprintf("%d-12-31", year))
	var rows []map[string]any
	require.NoError(t, testDB.Raw(sql, args...).Find(&rows).Error)
	return toOvRows(rows)
}

// seedOv chèn 1 lịch hẹn; customer/branch/isAccepted = nil ⇒ NULL.
func seedOv(t *testing.T, customer any, typeID int, branch any, date int64, status int, isAccepted any) {
	t.Helper()
	row := map[string]any{
		"code":                fmt.Sprintf("OV%d", time.Now().UnixNano()),
		"customer_id":         customer,
		"appointment_type_id": typeID,
		"branch_id":           branch,
		"appointment_date":    date,
		"status":              status,
		"is_accepted":         isAccepted,
		"created_at":          date,
		"updated_at":          date,
	}
	require.NoError(t, testDB.Table("appointment").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", id) })
}

func TestAppointmentOverview_KhopTayVaBanCu(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	brA := tn.BranchID
	brB := testhelpers.CreateBranch(t, testDB)
	loc := tz.LocFromContext(nil, testDB)
	d := func(y int, m time.Month, day, hour, min int) int64 {
		return time.Date(y, m, day, hour, min, 0, 0, loc).Unix()
	}
	cust := func() uint64 { return testhelpers.CreateScopedCustomer(t, testDB, tn, &brA) }
	c1, c2, c3, c4, c5, c6, c7, c8 := cust(), cust(), cust(), cust(), cust(), cust(), cust(), cust()
	const t1, t2, t3 = 1, 2, 3
	Y := ovYear

	// C1: gốc T1 + T3; huỷ ở T6 (khác tháng, vẫn tính cho cả T1 và T3 — cờ theo CẢ năm).
	seedOv(t, c1, t1, brA, d(Y, 1, 10, 9, 0), 1, nil)
	seedOv(t, c1, t1, brA, d(Y, 3, 5, 9, 0), 3, 1)
	seedOv(t, c1, t2, brA, d(Y, 6, 1, 9, 0), 4, nil)
	// C2: gốc T3 đồng ý; không đồng ý ở T11 (loại khác); huỷ ở NĂM SAU — không được tính.
	seedOv(t, c2, t1, brA, d(Y, 3, 7, 9, 0), 3, 1)
	seedOv(t, c2, t3, brA, d(Y, 11, 3, 9, 0), 2, 2)
	seedOv(t, c2, t2, brA, d(Y+1, 1, 5, 9, 0), 4, nil)
	// C3: 2 lịch gốc cùng T3 (đếm 1 khách), is_accepted NULL ⇒ không đồng ý cũng KHÔNG.
	seedOv(t, c3, t1, brA, d(Y, 3, 8, 9, 0), 2, nil)
	seedOv(t, c3, t1, brA, d(Y, 3, 20, 9, 0), 2, nil)
	// C4: không có lịch gốc ⇒ không xuất hiện ở đâu.
	seedOv(t, c4, t2, brA, d(Y, 4, 1, 9, 0), 4, nil)
	// C5: lịch gốc ở chi nhánh B ⇒ chỉ tính khi không lọc chi nhánh.
	seedOv(t, c5, t1, brB, d(Y, 1, 12, 9, 0), 1, nil)
	seedOv(t, c5, t2, brA, d(Y, 2, 2, 9, 0), 4, nil)
	// C6: gốc ở A, huỷ ở B ⇒ cờ huỷ chỉ tính khi không lọc chi nhánh.
	seedOv(t, c6, t1, brA, d(Y, 2, 3, 9, 0), 3, 2)
	seedOv(t, c6, t2, brB, d(Y, 2, 4, 9, 0), 4, nil)
	// C7: lịch gốc chi nhánh NULL ⇒ luôn hiện.
	seedOv(t, c7, t1, nil, d(Y, 2, 9, 9, 0), 4, nil)
	// C8: 01/02 00:30 giờ thương hiệu (= 31/01 theo UTC) ⇒ phải vào T2.
	seedOv(t, c8, t1, brA, d(Y, 2, 1, 0, 30), 1, nil)
	// Khách NULL: tháng 12 vẫn xuất hiện với toàn 0.
	seedOv(t, nil, t1, brA, d(Y, 12, 15, 9, 0), 1, nil)
	// Rìa đầu năm: 31/12 năm trước 23:30 — ngoài khoảng.
	seedOv(t, c4, t1, brA, d(Y-1, 12, 31, 23, 30), 1, nil)

	scopedWant := []ovRow{
		{12, 0, 0, 0, 0, 0},
		{3, 3, 1, 2, 1, 1}, // C1,C2,C3 · huỷ C1 · đồng ý C1,C2 · không đến C1 · không đồng ý C2
		{2, 3, 1, 0, 1, 1}, // C6,C7,C8 · huỷ C7 · không đến C8 · không đồng ý C6
		{1, 1, 1, 1, 1, 0}, // C1
	}
	allWant := []ovRow{
		{12, 0, 0, 0, 0, 0},
		{3, 3, 1, 2, 1, 1},
		{2, 3, 2, 0, 1, 1}, // + huỷ C6 (chi nhánh B)
		{1, 2, 2, 1, 2, 0}, // C1,C5 · huỷ C1,C5 · đồng ý C1 · không đến C1,C5
	}

	cases := []struct {
		name         string
		branchActive uint
		branchIDs    []uint
		want         []ovRow
	}{
		{"chi nhánh A active", brA, []uint{brA, brB}, scopedWant},
		{"không active, branchIds = [A]", 0, []uint{brA}, scopedWant},
		{"không phạm vi chi nhánh", 0, nil, allWant},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newOverview(t, tc.branchActive, tc.branchIDs, Y)
			require.Equal(t, tc.want, got, "bản mới lệch số đếm tay")
			require.Equal(t, oldOverview(t, tc.branchActive, tc.branchIDs, Y), got, "bản mới lệch output câu SQL cũ")
		})
	}
}

// Khác biệt CÓ CHỦ ĐÍCH: bản cũ so DATETIME với '<năm>-12-31' ⇒ mốc cuối là 31/12 00:00:00 ⇒ lịch
// hẹn ngày cuối bị bỏ. Bản mới lấy trọn ngày cuối.
func TestAppointmentOverview_NgayCuoiNam(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	loc := tz.LocFromContext(nil, testDB)
	Y := ovYear + 1
	c := testhelpers.CreateScopedCustomer(t, testDB, tn, &tn.BranchID)
	seedOv(t, c, 1, tn.BranchID, time.Date(Y, 12, 31, 10, 0, 0, 0, loc).Unix(), 4, nil)

	require.Equal(t, []ovRow{{12, 1, 1, 0, 0, 0}}, newOverview(t, tn.BranchID, []uint{tn.BranchID}, Y))
	require.Empty(t, oldOverview(t, tn.BranchID, []uint{tn.BranchID}, Y),
		"đối chứng: bản cũ đúng là bỏ lịch hẹn ngày 31/12 sau 00:00")
}

// Tham số year rác: giữ hợp đồng cũ 200 + [].
func TestAppointmentOverview_YearRac(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/crm/appointment/appointment-overview?year=abc", nil)
	(&Handler{DB: testDB}).AppointmentOverview(c)
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, "[]", w.Body.String())
}

// Bằng chứng cho comment ở define.AppointmentOverviewMaxExecMs: context huỷ ⇒ Go trả ngay, nhưng câu phía
// MySQL VẪN CHẠY (driver chỉ đóng socket, không gửi KILL QUERY); MAX_EXECUTION_TIME mới dừng được
// nó phía server. Test đỏ ở vế "vẫn chạy" ⇒ driver/MySQL đã đổi hành vi, sửa lại comment đó.
func TestAppointmentOverview_ContextCancel(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	marker := fmt.Sprintf("fixa_cancel_probe_%d", time.Now().UnixNano())
	killProbe := func() []int64 {
		var ids []int64
		testDB.Raw("SELECT id FROM information_schema.PROCESSLIST WHERE info LIKE ? AND info NOT LIKE ?",
			"%"+marker+"%", "%PROCESSLIST%").Scan(&ids)
		for _, id := range ids {
			testDB.Exec(fmt.Sprintf("KILL QUERY %d", id))
		}
		return ids
	}
	t.Cleanup(func() { killProbe() })

	// Câu thăm dò ĐỌC dòng (CTE đệ quy dài) — KHÔNG dùng BENCHMARK(): khi bị ngắt nó lặng lẽ trả 0
	// thay vì lỗi 3024 nên không chứng minh được gì về MAX_EXECUTION_TIME.
	probe := func(hint string) string {
		return "SELECT /*+ " + hint + "SET_VAR(cte_max_recursion_depth = 4294967295) */ COUNT(*) FROM (" +
			"WITH RECURSIVE r(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM r WHERE n < 4000000000) " +
			"SELECT n FROM r) AS t /* " + marker + " */"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	var x int64
	err := testDB.WithContext(ctx).Raw(probe("")).Scan(&x).Error
	require.Error(t, err, "câu thăm dò phải bị huỷ phía Go")
	require.Less(t, time.Since(start), 5*time.Second, "context huỷ phải trả về ngay")

	time.Sleep(700 * time.Millisecond)
	require.Len(t, killProbe(), 1, "câu phía MySQL vẫn phải còn chạy sau khi client huỷ (driver không KILL)")

	start = time.Now()
	err = testDB.Raw(probe("MAX_EXECUTION_TIME(300) ")).Scan(&x).Error
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "3024"), "MAX_EXECUTION_TIME phải dừng câu phía server, nhận: %v", err)
	require.Less(t, time.Since(start), 5*time.Second)
}

// Client đã ngắt trước khi câu chạy xong ⇒ handler không ghi response (và không đi FailQuery —
// đường đó logger.Error ⇒ cảnh báo Telegram cho mỗi lần người dùng đóng tab).
func TestAppointmentOverview_ClientDaNgat(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	c, w := ovCtx(0, nil, ovYear)
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	(&Handler{DB: testDB}).AppointmentOverview(c)
	require.False(t, c.Writer.Written(), "không được ghi response khi client đã đi, body: %s", w.Body.String())
	require.True(t, c.IsAborted())
}

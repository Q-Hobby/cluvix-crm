//go:build integration

package handler

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
)

// Đường HTTP (phạm vi từ session) và đường tường minh (phạm vi truyền vào) PHẢI cho cùng một
// tập kết quả. Đây là tính chất bảo mật của việc tách BuildAppointmentQueryIn: cron dựng lại
// query danh sách từ bộ lọc đã lưu, nếu hai đường lệch nhau thì lịch hẹn của phòng khám này
// có thể rơi vào Google Calendar của người thuộc phòng khám khác.
func TestBuildAppointmentQuery_HTTPvsBounds(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	gin.SetMode(gin.TestMode)

	mine := testhelpers.FreshTenant(t, testDB)
	other := testhelpers.FreshTenant(t, testDB)

	const revenueA, revenueB = 9001, 9002
	// 3 lịch của nhân sự A + 1 của nhân sự B trong tenant mình, 2 của nhân sự A ở tenant KHÁC.
	seed := []struct {
		tn        *testhelpers.TestTenant
		revenueID int
	}{
		{mine, revenueA}, {mine, revenueA}, {mine, revenueA},
		{mine, revenueB},
		{other, revenueA}, {other, revenueA},
	}
	now := time.Now().Unix()
	var wantMineRevenueA []uint
	for i, s := range seed {
		row := map[string]any{
			"code":                fmt.Sprintf("APT-BND-%d-%d", now, i),
			"branch_id":           s.tn.BranchID,
			"customer_id":         s.tn.CustomerID,
			"appointment_type_id": 1,
			"revenue_id":          s.revenueID,
			"appointment_date":    now + int64(i*3600),
			"status":              1,
			"created_at":          now,
			"updated_at":          now,
		}
		if err := testDB.Table("appointment").Create(&row).Error; err != nil {
			t.Fatalf("seed appointment %d: %v", i, err)
		}
		id := testhelpers.LastInsertID(testDB)
		t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", id) })
		if s.tn == mine && s.revenueID == revenueA {
			wantMineRevenueA = append(wantMineRevenueA, id)
		}
	}

	const filterRaw = "filter[revenue_id]=9001"

	// Đường HTTP: phạm vi nằm trong session.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/crm/appointment?"+filterRaw, nil)
	c.Set("branchActiveId", uint(mine.BranchID))

	h := &Handler{DB: testDB}
	viaHTTP, err := pluckIDs(h.BuildAppointmentQuery(c))
	if err != nil {
		t.Fatalf("đường HTTP: %v", err)
	}

	// Đường tường minh: phạm vi truyền vào, tham số lọc parse từ chuỗi đã lưu.
	b := tenant.Bounds{BranchActiveID: uint(mine.BranchID)}
	viaBounds, err := pluckIDs(h.BuildAppointmentQueryIn(query.ParseValues(filterRaw), b))
	if err != nil {
		t.Fatalf("đường Bounds: %v", err)
	}

	if len(viaHTTP) != len(wantMineRevenueA) {
		t.Fatalf("đường HTTP lấy %d dòng, mong đợi %d (%v)", len(viaHTTP), len(wantMineRevenueA), wantMineRevenueA)
	}
	if !sameSet(viaHTTP, viaBounds) {
		t.Errorf("hai đường lệch nhau:\n  HTTP   = %v\n  Bounds = %v", viaHTTP, viaBounds)
	}
	for _, id := range viaBounds {
		for _, w := range wantMineRevenueA {
			if id == w {
				goto ok
			}
		}
		t.Errorf("id %d không thuộc tập mong đợi %v — rò rỉ ngoài phạm vi", id, wantMineRevenueA)
	ok:
	}
}

// Story-41 (E6/D15): TestBoundsEmpty đã xoá cùng `Bounds.Empty()` + 2 field company — Empty() định
// nghĩa là "không có company" (lưới chống cron đọc xuyên thương hiệu), 0 caller non-test; với 1
// thương hiệu/DB, Bounds không có chi nhánh = mọi chi nhánh là hợp lệ, không còn trạng thái cần chặn.

// pluckIDs — query đã mang sẵn Select(appointmentListColumns) nên phải ghi đè Select
// trên một Session clone, không dùng Pluck (Pluck va với danh sách cột có sẵn).
func pluckIDs(q *gorm.DB) ([]uint, error) {
	var rows []struct{ ID uint }
	if err := q.Session(&gorm.Session{}).Select("appointment.id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]uint, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, nil
}

func sameSet(a, b []uint) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[uint]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
		if m[x] < 0 {
			return false
		}
	}
	return true
}

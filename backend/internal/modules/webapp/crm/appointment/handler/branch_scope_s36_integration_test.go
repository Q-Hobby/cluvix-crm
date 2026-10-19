//go:build integration

package handler

// Story-36 (E6/D15): các raw SQL tổng hợp của lịch hẹn (biểu đồ trạng thái, tổng quan
// năm, ngưỡng đông/vắng) chỉ còn vế chi nhánh, vế BRANCH phải còn nguyên: branchActive thắng branchIds,
// luôn kèm `branch_id IS NULL`, và được BỌC NGOẶC — fragment nối thẳng vào `WHERE ... AND`, mất
// ngoặc thì `OR branch_id IS NULL` thoát khỏi mọi điều kiện khác (khoảng ngày, loại lịch).
// Test viết TRƯỚC khi sửa (xanh trên code cũ), giữ xanh sau khi sửa.

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
)

// chartStatusLichHen gọi GetAppointmentChartStatus và trả TỔNG số lịch hẹn (nhóm `lichHen`).
// branchActive = 0 ⇒ không đặt khoá; branchIDs nil ⇒ không đặt khoá.
func chartStatusLichHen(t *testing.T, branchActive uint, branchIDs []uint, from, to int64) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET",
		fmt.Sprintf("/crm/appointment/get-appointment-chart-status?_time[gte]=%d&_time[lte]=%d", from, to), nil)
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
	}
	if branchIDs != nil {
		c.Set("branchIds", branchIDs)
	}
	h := &Handler{DB: testDB}
	h.GetAppointmentChartStatus(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var body struct {
		LichHen []struct {
			Y int64 `json:"y"`
		} `json:"lichHen"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	var sum int64
	for _, p := range body.LichHen {
		sum += p.Y
	}
	return sum
}

// seedRevenueType tạo 1 loại lịch hẹn có isTypeRevenue = 1 (biểu đồ chỉ đếm loại này; schema test
// không seed appointment_type) và trả id.
func seedRevenueType(t *testing.T) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{"name": fmt.Sprintf("S36 revenue %d", now), "isTypeRevenue": 1, "status": 1, "created_at": now, "updated_at": now}
	require.NoError(t, testDB.Table("appointment_type").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment_type WHERE id = ?", id) })
	return id
}

func TestAppointmentChartStatus_BranchScopeGiuNguyen(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	typeID := seedRevenueType(t)
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	branchA := tn.BranchID

	// Cửa sổ thời gian riêng (năm 2031) để dữ liệu test khác không lọt vào tổng.
	day := time.Date(2031, 3, 10, 5, 0, 0, 0, time.UTC).Unix()
	from, to := day-86400, day+86400
	seed := func(branch any, date int64) {
		seedAppointment(t, tn, map[string]any{"branch_id": branch, "appointment_type_id": typeID, "appointment_date": date})
	}
	seed(branchA, day)
	seed(branchB, day)
	seed(nil, day)
	// Mồi NGOÀI khoảng ngày, chưa gán chi nhánh: chỉ lọt vào tổng khi vế branch MẤT NGOẶC
	// (`... AND branch_id = ? OR branch_id IS NULL` ⇒ nhánh OR bỏ qua điều kiện ngày).
	seed(nil, day+30*86400)

	require.EqualValues(t, 2, chartStatusLichHen(t, branchA, []uint{branchA, branchB}, from, to),
		"chi nhánh A: A + chưa gán")
	require.EqualValues(t, 2, chartStatusLichHen(t, branchB, []uint{branchA, branchB}, from, to),
		"chi nhánh B: B + chưa gán")
	require.EqualValues(t, 2, chartStatusLichHen(t, 0, []uint{branchB}, from, to),
		"không có branch active ⇒ lọc theo branchIds")
	require.EqualValues(t, 3, chartStatusLichHen(t, 0, nil, from, to),
		"không có phạm vi chi nhánh ⇒ không lọc chi nhánh (đối chứng: dữ liệu seed đủ 3 dòng trong khoảng)")
}

// Story-36 (patch QA-C m2): attach khách vào lịch hẹn CỐ Ý không lọc chi nhánh
// (tenant.CrossBranchLookup) — khách đăng ký ở chi nhánh B vẫn đặt hẹn/được xem ở chi nhánh A.
// Đổi về scope có lọc branch thì field `customer` rỗng ⇒ test đỏ.
func TestAppointment_AttachKhachChiNhanhKhac(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	require.NoError(t, testDB.Exec("UPDATE customer SET branch_id = ? WHERE id = ?", branchB, tn.CustomerID).Error)
	apptID := seedAppointment(t, tn, nil) // lịch hẹn ở chi nhánh A (tn.BranchID)

	t.Run("ListAppointment", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", fmt.Sprintf("/crm/appointment?filter[customer_id]=%d&per-page=50", tn.CustomerID), nil)
		c.Set("branchActiveId", tn.BranchID)
		c.Set("branchIds", []uint{tn.BranchID})
		c.Set("userID", uint(1))
		(&Handler{DB: testDB}).ListAppointment(c)
		require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
		var body struct {
			Items []map[string]any `json:"items"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		var found map[string]any
		for _, it := range body.Items {
			if v, ok := it["id"].(float64); ok && uint(v) == apptID {
				found = it
			}
		}
		require.NotNil(t, found, "đối chứng: lịch hẹn chi nhánh A phải có trong list (body: %s)", w.Body.String())
		cust, ok := found["customer"].(map[string]any)
		require.True(t, ok && cust != nil, "khách chi nhánh khác phải được attach, nhận: %v", found["customer"])
		require.EqualValues(t, tn.CustomerID, cust["id"])
	})

	t.Run("GetAppointment", func(t *testing.T) {
		code, body := callApptDetail(t, tn.BranchID, fmt.Sprintf("/%d", apptID))
		require.Equal(t, 200, code)
		data, _ := body["data"].(map[string]any)
		cust, ok := data["customer"].(map[string]any)
		require.True(t, ok && cust != nil, "chi tiết lịch hẹn phải attach khách chi nhánh khác, nhận: %v", data["customer"])
	})
}

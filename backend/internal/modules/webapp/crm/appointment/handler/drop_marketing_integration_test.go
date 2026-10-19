//go:build integration

package handler

// User chốt 2026-10-18: bỏ hẳn field Marketing (`appointment.marketing_id`) — migration
// 20261018152846 xoá cột. Neo: (1) tạo lịch hẹn vẫn chạy trên schema KHÔNG còn cột, kể cả khi FE
// bản cũ còn gửi `marketing_id` trong body (bị bỏ qua, không lỗi 1054); (2) response không còn
// key `marketing_id`; (3) list với query `?marketing_id=` cũ không còn lọc/không lỗi SQL.

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	branchHandler "erpCluvix/internal/modules/webapp/config/branch/handler"
	"erpCluvix/internal/testhelpers"
)

func TestAppointment_BoMarketing_TaoVaListVanChay(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	var colCount int64
	require.NoError(t, testDB.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'appointment' AND COLUMN_NAME = 'marketing_id'`).Scan(&colCount).Error)
	require.Zero(t, colCount, "migration phải đã xoá cột appointment.marketing_id")

	typeID := seedRevenueType(t)
	tn := testhelpers.FreshTenant(t, testDB)
	// Tạo lịch hẹn gen mã theo config_code của chi nhánh — FreshTenant không seed (production seed lúc tạo chi nhánh).
	require.NoError(t, branchHandler.SeedConfigCodeForBranch(testDB, tn.BranchID, 1))
	t.Cleanup(func() { testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", tn.BranchID) })

	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(tn.BranchID, 1))
	r.POST("/create", h.CreateAppointment)
	r.GET("/list", h.ListAppointment)

	date := time.Date(2032, 5, 12, 3, 0, 0, 0, time.UTC).Unix()
	body := fmt.Sprintf(`{"customer_id": %d, "branch_id": %d, "appointment_type_id": %d,
		"appointment_date": %d, "marketing_id": 1}`, tn.CustomerID, tn.BranchID, typeID, date)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Less(t, w.Code, 300, "body: %s", w.Body.String())
	require.NotContains(t, w.Body.String(), "marketing_id")

	var resp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "body: %s", w.Body.String())
	require.NotZero(t, resp.Data.ID, "body: %s", w.Body.String())
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", resp.Data.ID) })

	var n int64
	require.NoError(t, testDB.Table("appointment").Where("id = ? AND customer_id = ?", resp.Data.ID, tn.CustomerID).Count(&n).Error)
	require.EqualValues(t, 1, n)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/list?customer_id=%d&marketing_id=999999", tn.CustomerID), nil))
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	require.NotContains(t, w.Body.String(), "marketing_id")
}

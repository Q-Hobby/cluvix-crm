//go:build integration

package handler

// Story-37 (E6/D15): danh sách dịch vụ đã dùng kèm lịch hẹn
// (ListServiceUsedWithAppointment) — vế BRANCH của
// order_service_used phải còn nguyên (branchActive thắng branchIds, luôn kèm branch_id IS NULL).
// Attach customer/appointment/order_plan vào list CỐ Ý không lọc chi nhánh (tenant.CrossBranchLookup):
// khách/lịch hẹn/đơn ở chi nhánh khác vẫn phải hiện đủ trên dòng dịch vụ của chi nhánh đang xem.
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

func seedApptS37(t *testing.T, tn *testhelpers.TestTenant, branch any, date int64) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{
		"code":             fmt.Sprintf("LH%d", time.Now().UnixNano()),
		"customer_id":      tn.CustomerID,
		"branch_id":        branch,
		"appointment_date": date,
		"status":           1,
		"created_at":       now,
		"updated_at":       now,
	}
	require.NoError(t, testDB.Table("appointment").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", id) })
	return id
}

func seedPlanS37(t *testing.T, tn *testhelpers.TestTenant, branch any) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{
		"code": fmt.Sprintf("DH%d", time.Now().UnixNano()), "customer_id": tn.CustomerID,
		"branch_id": branch, "orderPlanType": 1,
		"created_time": now, "status": 1, "created_at": now, "updated_at": now,
	}
	require.NoError(t, testDB.Table("order_plan").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM order_plan WHERE id = ?", id) })
	return id
}

// listSUWithAppt gọi ListServiceUsedWithAppointment (lọc ngày hẹn ⇒ có JOIN appointment) và trả items.
// branchActive = 0 ⇒ không đặt khoá; branchIDs nil ⇒ không đặt khoá.
func listSUWithAppt(t *testing.T, tn *testhelpers.TestTenant, branchActive uint, branchIDs []uint, from, to int64) []map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", fmt.Sprintf(
		"/crm/order-service-used/service-used-with-appointment?filter[appointment_date][gte]=%d&filter[appointment_date][lte]=%d&per-page=50",
		from, to), nil)
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
	}
	if branchIDs != nil {
		c.Set("branchIds", branchIDs)
	}
	c.Set("userID", uint(1))
	(&Handler{DB: testDB}).ListServiceUsedWithAppointment(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var body struct {
		Object struct {
			Items []map[string]any `json:"items"`
		} `json:"object"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	return body.Object.Items
}

func TestListServiceUsedWithAppointment_BranchScopeGiuNguyen(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchA := tn.BranchID
	branchB := testhelpers.CreateBranch(t, testDB)

	// Cửa sổ riêng (năm 2032) để dữ liệu test khác không lọt vào.
	day := time.Date(2032, 5, 10, 5, 0, 0, 0, time.UTC).Unix()
	from, to := day-86400, day+86400
	seed := func(branch any, date int64) uint {
		apptID := seedApptS37(t, tn, branch, date)
		return seedOrderServiceUsed(t, tn, map[string]any{"branch_id": branch, "appointment_id": apptID})
	}
	idA := seed(branchA, day)
	idB := seed(branchB, day)
	idNull := seed(nil, day)
	// Mồi NGOÀI khoảng ngày, chưa gán chi nhánh: chỉ lọt vào khi vế branch mất ngoặc.
	idOut := seed(nil, day+30*86400)

	ids := func(items []map[string]any) map[uint]bool {
		m := map[uint]bool{}
		for _, it := range items {
			m[uint(it["id"].(float64))] = true
		}
		return m
	}
	want := func(keys ...uint) map[uint]bool {
		m := map[uint]bool{}
		for _, k := range keys {
			m[k] = true
		}
		return m
	}
	_ = idOut

	require.Equal(t, want(idA, idNull), ids(listSUWithAppt(t, tn, branchA, []uint{branchA, branchB}, from, to)),
		"chi nhánh A: A + chưa gán")
	require.Equal(t, want(idB, idNull), ids(listSUWithAppt(t, tn, branchB, []uint{branchA, branchB}, from, to)),
		"chi nhánh B: B + chưa gán")
	require.Equal(t, want(idB, idNull), ids(listSUWithAppt(t, tn, 0, []uint{branchB}, from, to)),
		"không có branch active ⇒ lọc theo branchIds")
	require.Equal(t, want(idA, idB, idNull), ids(listSUWithAppt(t, tn, 0, nil, from, to)),
		"không có phạm vi chi nhánh ⇒ không lọc (đối chứng: đủ 3 dòng trong khoảng)")
}

// Dòng dịch vụ ở chi nhánh A, còn khách + lịch hẹn + đơn ở chi nhánh B, cross-branch tắt:
// attach vẫn phải có dữ liệu (đổi attach về scope có lọc branch ⇒ field rỗng ⇒ đỏ).
func TestOrderServiceUsedList_AttachChiNhanhKhac(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	require.NoError(t, testDB.Exec("UPDATE customer SET branch_id = ? WHERE id = ?", branchB, tn.CustomerID).Error)

	day := time.Date(2032, 6, 10, 5, 0, 0, 0, time.UTC).Unix()
	apptID := seedApptS37(t, tn, branchB, day)
	planID := seedPlanS37(t, tn, branchB)
	suID := seedOrderServiceUsed(t, tn, map[string]any{"appointment_id": apptID, "order_plan_id": planID})

	find := func(items []map[string]any) map[string]any {
		for _, it := range items {
			if uint(it["id"].(float64)) == suID {
				return it
			}
		}
		return nil
	}
	requireAttached := func(t *testing.T, item map[string]any, keys ...string) {
		t.Helper()
		require.NotNil(t, item, "đối chứng: dòng dịch vụ chi nhánh A phải có trong list")
		for _, k := range keys {
			rel, ok := item[k].(map[string]any)
			require.True(t, ok && rel["id"] != nil, "%s chi nhánh khác phải được attach, item=%v", k, item)
		}
	}

	t.Run("ListServiceUsedWithAppointment", func(t *testing.T) {
		items := listSUWithAppt(t, tn, tn.BranchID, []uint{tn.BranchID}, day-86400, day+86400)
		requireAttached(t, find(items), "customer", "appointment")
	})

	t.Run("ListTreatmentService", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", fmt.Sprintf("/crm/order-service-used?filter[appointment_id]=%d&per-page=50", apptID), nil)
		c.Set("branchActiveId", tn.BranchID)
		c.Set("branchIds", []uint{tn.BranchID})
		c.Set("userID", uint(1))
		(&Handler{DB: testDB}).ListTreatmentService(c)
		require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
		var body struct {
			Object struct {
				Items []map[string]any `json:"items"`
			} `json:"object"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
		requireAttached(t, find(body.Object.Items), "customer", "treatmentPlan", "appointment")
	})
}

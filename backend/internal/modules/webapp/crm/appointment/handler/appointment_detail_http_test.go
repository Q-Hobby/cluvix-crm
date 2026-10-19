//go:build integration

package handler

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
)

// Integration test cho GET /crm/appointment/:id.
//
// Neo 2 thứ đã sửa ở endpoint này: (1) customer attach có `birthday` (ngày sinh + tuổi);
// (2) attach quan hệ chỉ màn chi tiết cần (dịch vụ điều trị). Cả 2 đều là loại hỏng không có lỗi
// nào báo ra. (Nhóm tiếp nhận/sinh hiệu EMR + quan hệ khoa/phòng/bệnh án đã bỏ ở story-12
// crm-standalone.)

// ⚠ `customer.birthday` là kiểu DATE, KHÔNG phải bigint unix như phần lớn cột thời gian khác trong
// repo (`created_at`, `appointment_date`…). Ghi số unix vào đây MySQL báo "Incorrect date value".
func seedApptCustomerBirthday(t *testing.T, tenant *testhelpers.TestTenant, birthday string) {
	t.Helper()
	require.NoError(t, testDB.Exec("UPDATE customer SET birthday = ? WHERE id = ?", birthday, tenant.CustomerID).Error)
}

func seedOrderServiceUsedForAppt(t *testing.T, tenant *testhelpers.TestTenant) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":          fmt.Sprintf("SV%d", time.Now().UnixNano()),
		"customer_id":   tenant.CustomerID,
		"order_plan_id": 1,
		"service_id":    1,
		"quantity":      1,
		"created_at":    now,
		"updated_at":    now,
	})
	require.NoError(t, testDB.Table("order_service_used").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM order_service_used WHERE id = ?", id) })
	return id
}

// seedAppointment tạo lịch hẹn kèm FK sang các bản ghi vừa seed.
func seedAppointment(t *testing.T, tenant *testhelpers.TestTenant, extra map[string]any) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":             fmt.Sprintf("LH%d", time.Now().UnixNano()),
		"customer_id":      tenant.CustomerID,
		"appointment_date": now,
		"status":           1,
		"content":          "nội dung hẹn",
		"created_at":       now,
		"updated_at":       now,
	})
	for k, v := range extra {
		row[k] = v
	}
	require.NoError(t, testDB.Table("appointment").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", id) })
	return id
}

func callApptDetail(t *testing.T, branchID uint, path string) (int, map[string]any) {
	t.Helper()
	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(branchID, 1))
	r.GET("/:id", h.GetAppointment)

	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body map[string]any
	if w.Body.Len() > 0 {
		dec := json.NewDecoder(strings.NewReader(w.Body.String()))
		dec.UseNumber()
		_ = dec.Decode(&body)
	}
	return w.Code, body
}

// --- tests ------------------------------------------------------------------

// Đường chính: lịch hẹn có customer (kèm birthday) + quan hệ dịch vụ điều trị.
func TestGetAppointment_ReturnsCustomerAndRelations(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	const birthday = "1990-01-01"
	seedApptCustomerBirthday(t, tenant, birthday)

	serviceUsedID := seedOrderServiceUsedForAppt(t, tenant)

	apptID := seedAppointment(t, tenant, map[string]any{
		"order_service_used_id": serviceUsedID,
	})

	code, body := callApptDetail(t, tenant.BranchID, fmt.Sprintf("/%d", apptID))
	require.Equal(t, 200, code)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	// (1) customer attach phải có birthday (chi tiết hiển thị ngày sinh + tuổi)
	customer, ok := data["customer"].(map[string]any)
	require.True(t, ok, "phải attach customer")
	require.Contains(t, customer, "birthday", "thiếu birthday ⇒ dòng Ngày sinh và tuổi luôn rỗng")
	assert.Contains(t, fmt.Sprint(customer["birthday"]), birthday)

	// (2) quan hệ chỉ màn chi tiết cần
	used, ok := data["orderServiceUsed"].(map[string]any)
	require.True(t, ok, "phải attach orderServiceUsed")
	assert.Equal(t, fmt.Sprint(serviceUsedID), fmt.Sprint(used["id"]))
}

// Lịch hẹn KHÔNG có FK quan hệ: khoá quan hệ vắng mặt, nhưng request vẫn 200 và vẫn có
// customer — đối chứng cho ca trên (chứng minh khoá kia đến từ dữ liệu, không phải luôn có).
func TestGetAppointment_WithoutRelations_StillOk(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	apptID := seedAppointment(t, tenant, nil)

	code, body := callApptDetail(t, tenant.BranchID, fmt.Sprintf("/%d", apptID))
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)

	assert.Contains(t, data, "customer")
	assert.NotContains(t, data, "orderServiceUsed", "không có FK thì không được bịa ra quan hệ rỗng")
}

func TestGetAppointment_InvalidID_Returns422(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	code, _ := callApptDetail(t, 1, "/undefined")
	assert.Equal(t, 422, code)
}

func TestGetAppointment_NotFound_Returns404(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	code, _ := callApptDetail(t, 1, "/99999999")
	assert.Equal(t, 404, code)
}

func TestGetAppointment_OtherTenant_Returns404(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	owner := testhelpers.FreshTenant(t, testDB)
	apptID := seedAppointment(t, owner, nil)

	other := testhelpers.FreshTenant(t, testDB)
	code, _ := callApptDetail(t, other.BranchID, fmt.Sprintf("/%d", apptID))
	assert.Equal(t, 404, code)

	// ĐỐI CHỨNG: đúng tenant thì 200 — chứng minh 404 đến từ scope, không phải fixture hỏng.
	codeOwner, _ := callApptDetail(t, owner.BranchID, fmt.Sprintf("/%d", apptID))
	require.Equal(t, 200, codeOwner)
}

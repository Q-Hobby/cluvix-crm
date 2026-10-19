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

// Integration test cho GET /crm/order-service-used/:id.
//
// Neo 2 thứ đã sửa: (1) chuỗi Select tường minh thay `SELECT *` — và hệ quả quan trọng nhất là nhóm
// hoa hồng CTV KHÔNG còn lọt ra response; (2) id sai trả 422 chứ không phải 404.
// Đồng thời khoá phần mà màn Thanh toán phụ thuộc (`totalEnd`, `payment`, `refundServiceUsed`,
// `created_day`) — cắt nhầm một trong 4 cột đó là form thanh toán tính sai tiền, không lỗi nào báo.

func seedOrderServiceUsed(t *testing.T, tenant *testhelpers.TestTenant, extra map[string]any) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":              fmt.Sprintf("SV%d", time.Now().UnixNano()),
		"customer_id":       tenant.CustomerID,
		"order_plan_id":     1,
		"service_id":        1,
		"quantity":          2,
		"unit_price":        500_000,
		"totalEnd":          1_000_000,
		"payment":           300_000,
		"refundServiceUsed": 50_000,
		"isLock":            1,
		"note":              "ghi chú dịch vụ",
		"created_day":       now,
		// Cố ý seed nhóm hoa hồng: test dưới khẳng định chúng KHÔNG lọt ra response.
		"commission_telesale_snap": 12.5,
		"created_at":               now,
		"updated_at":               now,
	})
	for k, v := range extra {
		row[k] = v
	}
	require.NoError(t, testDB.Table("order_service_used").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM order_service_used WHERE id = ?", id) })
	return id
}

func callServiceDetail(t *testing.T, branchID uint, path string) (int, map[string]any) {
	t.Helper()
	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(branchID, 1))
	r.GET("/:id", h.GetTreatmentService)

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

func TestGetTreatmentService_ReturnsFieldsPanelAndPaymentFormNeed(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	id := seedOrderServiceUsed(t, tenant, nil)

	code, body := callServiceDetail(t, tenant.BranchID, fmt.Sprintf("/%d", id))
	require.Equal(t, 200, code)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	// 4 cột màn Thanh toán đọc — cắt nhầm là tính sai tiền mà không có lỗi nào báo.
	for _, col := range []string{"totalEnd", "payment", "refundServiceUsed", "created_day"} {
		require.Contains(t, data, col, "cột %s bị thiếu ⇒ form thanh toán tính sai", col)
	}
	assert.Equal(t, "1000000", fmt.Sprint(data["totalEnd"]))
	assert.Equal(t, "300000", fmt.Sprint(data["payment"]))
	assert.Equal(t, "50000", fmt.Sprint(data["refundServiceUsed"]))

	// Cột panel chi tiết dùng
	// story-14: cột liệu trình (completion_percent/numTreatment*/applied_position) đã bỏ khỏi response.
	for _, col := range []string{"completion_percent", "numTreatment", "numTreatmentOfService", "applied_position"} {
		assert.NotContains(t, data, col, "cột liệu trình %s không còn trong response", col)
	}
	assert.Equal(t, "ghi chú dịch vụ", data["note"])
}

// Nhóm hoa hồng CTV không được lọt ra — đây là lý do endpoint bỏ `SELECT *`.
func TestGetTreatmentService_DoesNotLeakCommissionFields(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	id := seedOrderServiceUsed(t, tenant, nil)

	code, body := callServiceDetail(t, tenant.BranchID, fmt.Sprintf("/%d", id))
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)

	// ĐỐI CHỨNG: khẳng định response CÓ dữ liệu thật, để các assert "không chứa" không xanh vô nghĩa.
	require.Contains(t, data, "totalEnd")

	for _, leaked := range []string{"commission_rate_snap", "status_commission", "broker_id", "commission_telesale_snap"} {
		assert.NotContains(t, data, leaked, "cột nhạy cảm/không dùng không được trả về")
	}
}

func TestGetTreatmentService_InvalidID_Returns422(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	code, _ := callServiceDetail(t, 1, "/undefined")
	assert.Equal(t, 422, code)
}

func TestGetTreatmentService_NotFound_Returns404(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	code, _ := callServiceDetail(t, 1, "/99999999")
	assert.Equal(t, 404, code)
}

func TestGetTreatmentService_OtherTenant_Returns404(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	owner := testhelpers.FreshTenant(t, testDB)
	id := seedOrderServiceUsed(t, owner, nil)

	other := testhelpers.FreshTenant(t, testDB)
	code, _ := callServiceDetail(t, other.BranchID, fmt.Sprintf("/%d", id))
	assert.Equal(t, 404, code)

	// ĐỐI CHỨNG: đúng tenant phải 200.
	codeOwner, _ := callServiceDetail(t, owner.BranchID, fmt.Sprintf("/%d", id))
	require.Equal(t, 200, codeOwner)
}

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
	"erpCluvix/pkg/define"
)

// Integration test cho GET /crm/payment/:id/detail.
//
// Điểm cần khoá chặt nhất là **bảng phân bổ**: hệ thống KHÔNG có bảng phân bổ riêng — thu tiền cho
// đơn tạo NHIỀU dòng `payment`, mỗi dòng một dịch vụ. `allocations` vì vậy phải là **đúng tập phiếu
// cùng `parent_id`**, không nhiều không ít: gom thừa (theo thời gian, theo khách) hay thiếu đều làm
// người dùng đọc sai số tiền của đơn.

func seedPaymentOrderRefs(t *testing.T, tenant *testhelpers.TestTenant) (planID, serviceUsedID uint) {
	t.Helper()
	now := time.Now().Unix()
	plan := testhelpers.WithTenantCols(tenant, map[string]any{
		"code": fmt.Sprintf("DH%d", time.Now().UnixNano()), "customer_id": tenant.CustomerID,
		"orderPlanType": 1,
		"created_time":  now, "status": 1, "created_at": now, "updated_at": now,
	})
	require.NoError(t, testDB.Table("order_plan").Create(&plan).Error)
	planID = testhelpers.LastInsertID(testDB)

	su := testhelpers.WithTenantCols(tenant, map[string]any{
		"code": fmt.Sprintf("SV%d", time.Now().UnixNano()), "customer_id": tenant.CustomerID,
		"order_plan_id": planID, "service_id": 1,
		"quantity": 1, "totalEnd": 1_000_000, "payment": 300_000,
		"created_at": now, "updated_at": now,
	})
	require.NoError(t, testDB.Table("order_service_used").Create(&su).Error)
	serviceUsedID = testhelpers.LastInsertID(testDB)

	t.Cleanup(func() {
		testDB.Exec("DELETE FROM order_service_used WHERE id = ?", serviceUsedID)
		testDB.Exec("DELETE FROM order_plan WHERE id = ?", planID)
	})
	return
}

func seedPaymentRow(t *testing.T, tenant *testhelpers.TestTenant, parentID, serviceUsedID uint, amount int64, paymentFor int) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":         fmt.Sprintf("PT%d", time.Now().UnixNano()),
		"customer_id":  tenant.CustomerID,
		"amount":       amount,
		"method":       1,
		"payment_for":  paymentFor,
		"payment_time": now,
		"note":         "ghi chú phiếu",
		"created_at":   now,
		"updated_at":   now,
	})
	if parentID > 0 {
		row["parent_id"] = parentID
	}
	if serviceUsedID > 0 {
		row["serviceUsed_id"] = serviceUsedID
	}
	require.NoError(t, testDB.Table("payment").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM payment WHERE id = ?", id) })
	return id
}

func callPaymentDetail(t *testing.T, branchID uint, path string) (int, map[string]any) {
	t.Helper()
	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(branchID, 1))
	r.GET("/:id/detail", h.GetPaymentDetail)

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

func TestGetPaymentDetail_ReturnsPaymentWithRelations(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	planID, serviceUsedID := seedPaymentOrderRefs(t, tenant)
	payID := seedPaymentRow(t, tenant, planID, serviceUsedID, 300_000, define.PaymentForOrder)

	code, body := callPaymentDetail(t, tenant.BranchID, fmt.Sprintf("/%d/detail", payID))
	require.Equal(t, 200, code)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "300000", fmt.Sprint(data["amount"]))
	assert.Equal(t, fmt.Sprint(define.PaymentForOrder), fmt.Sprint(data["payment_for"]))
	assert.Equal(t, "ghi chú phiếu", data["note"])

	customer, ok := data["customer"].(map[string]any)
	require.True(t, ok, "phải attach customer")
	assert.Equal(t, fmt.Sprint(tenant.CustomerID), fmt.Sprint(customer["id"]))

	plan, ok := data["orderPlan"].(map[string]any)
	require.True(t, ok, "phải attach orderPlan (FK `parent_id`)")
	assert.Equal(t, fmt.Sprint(planID), fmt.Sprint(plan["id"]))

	su, ok := data["serviceUsed"].(map[string]any)
	require.True(t, ok, "phải attach serviceUsed")
	assert.Equal(t, fmt.Sprint(serviceUsedID), fmt.Sprint(su["id"]))
}

// `allocations` = ĐÚNG tập phiếu cùng đơn hàng.
func TestGetPaymentDetail_AllocationsCoverExactlyTheOrdersReceipts(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	planID, serviceUsedID := seedPaymentOrderRefs(t, tenant)

	// 2 phiếu của ĐƠN NÀY (đúng cái phải xuất hiện trong allocations)
	payA := seedPaymentRow(t, tenant, planID, serviceUsedID, 300_000, define.PaymentForOrder)
	payB := seedPaymentRow(t, tenant, planID, serviceUsedID, 200_000, define.PaymentForMoveDeposit)

	// 1 phiếu của ĐƠN KHÁC + 1 phiếu KHÔNG gắn đơn, cùng khách cùng thời điểm — đối chứng: nếu gom
	// theo khách hay theo thời gian (thay vì theo parent_id) thì 2 dòng này lọt vào và test đỏ.
	otherPlanID, otherSuID := seedPaymentOrderRefs(t, tenant)
	seedPaymentRow(t, tenant, otherPlanID, otherSuID, 999_000, define.PaymentForOrder)
	seedPaymentRow(t, tenant, 0, 0, 111_000, define.PaymentForDeposit)

	code, body := callPaymentDetail(t, tenant.BranchID, fmt.Sprintf("/%d/detail", payA))
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)

	rows, ok := data["allocations"].([]any)
	require.True(t, ok, "phải có mảng allocations")
	require.Len(t, rows, 2, "chỉ được gồm phiếu CÙNG parent_id")

	ids := map[string]bool{}
	total := int64(0)
	for _, r := range rows {
		row := r.(map[string]any)
		ids[fmt.Sprint(row["id"])] = true
		require.Contains(t, row, "code")
		require.Contains(t, row, "payment_for")
		var amt int64
		fmt.Sscan(fmt.Sprint(row["amount"]), &amt)
		total += amt
	}
	assert.True(t, ids[fmt.Sprint(payA)], "phải chứa chính phiếu đang xem")
	assert.True(t, ids[fmt.Sprint(payB)], "phải chứa phiếu còn lại của đơn")
	assert.Equal(t, int64(500_000), total, "tổng phân bổ = tổng phiếu của đơn")
}

// Phiếu không gắn đơn (thu/chi phòng khám, cọc lẻ) → mảng RỖNG, không phải null.
// Phân bổ phải trả ĐỦ mọi loại phiếu của đơn (để nhìn được toàn cảnh dòng tiền) và kèm `service_id`
// cho từng dòng. Con số "đã thu" thì FE chỉ cộng loại 4+5 — test này khoá phần dữ liệu, phần cộng
// nằm ở panel; hai vế phải khớp nhau nên seed đủ 3 loại: thanh toán, đặt cọc, hoàn dịch vụ.
func TestGetPaymentDetail_AllocationsIncludeDepositAndRefundWithServiceId(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	planID, serviceUsedID := seedPaymentOrderRefs(t, tenant)

	payID := seedPaymentRow(t, tenant, planID, serviceUsedID, 300_000, define.PaymentForOrder)
	seedPaymentRow(t, tenant, planID, 0, 100_000, define.PaymentForDeposit)
	// Hoàn dịch vụ: BE lưu amount ÂM (negate ở beforeSave) — seed đúng như DB thật.
	seedPaymentRow(t, tenant, planID, serviceUsedID, -50_000, define.PaymentForRefundService)

	code, body := callPaymentDetail(t, tenant.BranchID, fmt.Sprintf("/%d/detail", payID))
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)

	rows, ok := data["allocations"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 3, "phải liệt kê đủ 3 loại phiếu của đơn")

	byFor := map[string]map[string]any{}
	for _, r := range rows {
		row := r.(map[string]any)
		byFor[fmt.Sprint(row["payment_for"])] = row
	}

	// Dòng gắn dịch vụ phải kèm `service_id` — thiếu là cột "dịch vụ" của bảng phân bổ trống.
	paid := byFor[fmt.Sprint(define.PaymentForOrder)]
	require.NotNil(t, paid)
	require.Contains(t, paid, "service_id", "dòng phân bổ phải kèm service_id để FE tra tên dịch vụ")
	assert.Equal(t, "1", fmt.Sprint(paid["service_id"]))

	// Dòng không gắn dịch vụ (đặt cọc cấp đơn) vẫn có mặt, `service_id` để trống.
	deposit := byFor[fmt.Sprint(define.PaymentForDeposit)]
	require.NotNil(t, deposit, "phiếu đặt cọc của đơn phải nằm trong danh sách")
	assert.Nil(t, deposit["service_id"])

	// Hoàn tiền giữ nguyên dấu ÂM để phía hiển thị tự quyết cách trình bày.
	refund := byFor[fmt.Sprint(define.PaymentForRefundService)]
	require.NotNil(t, refund)
	assert.Equal(t, "-50000", fmt.Sprint(refund["amount"]))
}

func TestGetPaymentDetail_NoOrder_ReturnsEmptyAllocations(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	payID := seedPaymentRow(t, tenant, 0, 0, 500_000, define.PaymentForClinicIncome)

	code, body := callPaymentDetail(t, tenant.BranchID, fmt.Sprintf("/%d/detail", payID))
	require.Equal(t, 200, code)
	data := body["data"].(map[string]any)

	rows, ok := data["allocations"].([]any)
	require.True(t, ok, "allocations phải là mảng kể cả khi rỗng — FE luôn .map() được")
	assert.Len(t, rows, 0)
	assert.NotContains(t, data, "orderPlan", "không có đơn thì không được bịa quan hệ rỗng")
}

func TestGetPaymentDetail_InvalidID_Returns422(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	code, _ := callPaymentDetail(t, 1, "/undefined/detail")
	assert.Equal(t, 422, code)
}

func TestGetPaymentDetail_NotFound_Returns404(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	code, _ := callPaymentDetail(t, 1, "/99999999/detail")
	assert.Equal(t, 404, code)
}

func TestGetPaymentDetail_OtherTenant_Returns404(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}

	owner := testhelpers.FreshTenant(t, testDB)
	planID, suID := seedPaymentOrderRefs(t, owner)
	payID := seedPaymentRow(t, owner, planID, suID, 300_000, define.PaymentForOrder)

	other := testhelpers.FreshTenant(t, testDB)
	code, _ := callPaymentDetail(t, other.BranchID, fmt.Sprintf("/%d/detail", payID))
	assert.Equal(t, 404, code)

	// ĐỐI CHỨNG: đúng tenant phải 200.
	codeOwner, _ := callPaymentDetail(t, owner.BranchID, fmt.Sprintf("/%d/detail", payID))
	require.Equal(t, 200, codeOwner)
}

//go:build integration

package handler

// Story-37 (E6/D15): dịch vụ con của đơn nạp qua tenant.CrossBranchLookup — dịch vụ trong CÙNG đơn có thể ở chi nhánh khác (điều trị chéo chi nhánh),
// lọc branch sẽ làm mất dòng của chính đơn đang xem. Còn bản thân đơn vẫn lọc chi nhánh như cũ.
// Test viết TRƯỚC khi sửa (xanh trên code cũ), giữ xanh sau khi sửa.

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func callGetOrderPlan(t *testing.T, branchID uint, path string) (int, map[string]any) {
	t.Helper()
	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(branchID, 1))
	r.GET("/:id", h.GetOrderPlan)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	var body map[string]any
	if w.Body.Len() > 0 {
		dec := json.NewDecoder(strings.NewReader(w.Body.String()))
		dec.UseNumber()
		_ = dec.Decode(&body)
	}
	return w.Code, body
}

func TestOrderPlan_DichVuChiNhanhKhacVanHien(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	orderID := seedOrderPlan(t, tn) // đơn ở chi nhánh A
	seedOrderService(t, tn, orderID, 1_000_000, 0, 0)
	svcB := seedOrderService(t, tn, orderID, 500_000, 0, 0)
	require.NoError(t, testDB.Exec("UPDATE order_service_used SET branch_id = ? WHERE id = ?", branchB, svcB).Error)
	seedOrderPayment(t, tn, orderID, 200_000, define.PaymentForOrder)

	check := func(t *testing.T, code int, body map[string]any) {
		t.Helper()
		require.Equal(t, 200, code, "body=%v", body)
		data := body["data"].(map[string]any)
		svcs := data["orderServiceUseds"].([]any)
		require.Len(t, svcs, 2, "dịch vụ chi nhánh khác của CÙNG đơn phải hiện")
		require.Equal(t, "1500000", fmt.Sprint(data["total"]), "tổng = cả 2 dịch vụ")
		require.Equal(t, "200000", fmt.Sprint(data["payment"]))
	}
	t.Run("GetOrderPlanDetail", func(t *testing.T) {
		code, body := callDetail(t, tn.BranchID, fmt.Sprintf("/%d/detail", orderID))
		check(t, code, body)
	})
	t.Run("GetOrderPlan", func(t *testing.T) {
		code, body := callGetOrderPlan(t, tn.BranchID, fmt.Sprintf("/%d", orderID))
		check(t, code, body)
	})

	// Branch scope của ĐƠN còn nguyên: user chỉ ở chi nhánh B (cross-branch tắt) không đọc được đơn A.
	t.Run("DonChiNhanhKhac404", func(t *testing.T) {
		code, _ := callDetail(t, branchB, fmt.Sprintf("/%d/detail", orderID))
		require.Equal(t, 404, code)
		code, _ = callGetOrderPlan(t, branchB, fmt.Sprintf("/%d", orderID))
		require.Equal(t, 404, code)
	})
}

// Story-37 (D15): subquery tiền của đơn chỉ lọc theo đơn — mọi phiếu thu của CHÍNH đơn này đều được
// cộng (story-62: bảng payment không còn khoá thương hiệu).
func TestOrderPlanMoney_CongDuMoiPhieuCuaDon(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	orderID := seedOrderPlan(t, tn)
	seedOrderService(t, tn, orderID, 1_000_000, 0, 0)
	seedOrderPayment(t, tn, orderID, 200_000, define.PaymentForOrder)
	seedOrderPayment(t, tn, orderID, 50_000, define.PaymentForOrder)

	code, body := callDetail(t, tn.BranchID, fmt.Sprintf("/%d/detail", orderID))
	require.Equal(t, 200, code)
	require.Equal(t, "250000", fmt.Sprint(body["data"].(map[string]any)["payment"]))
	code, body = callGetOrderPlan(t, tn.BranchID, fmt.Sprintf("/%d", orderID))
	require.Equal(t, 200, code)
	require.Equal(t, "250000", fmt.Sprint(body["data"].(map[string]any)["payment"]))
}

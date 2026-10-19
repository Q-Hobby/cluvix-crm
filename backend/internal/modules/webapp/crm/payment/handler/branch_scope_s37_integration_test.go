//go:build integration

package handler

// Story-37 (E6/D15): các tra cứu FK của phiếu thu CỐ Ý không lọc chi nhánh
// (tenant.CrossBranchLookup): đơn/dịch vụ/khách/lịch hẹn có thể ở chi
// nhánh khác chi nhánh của phiếu (điều trị/thanh toán chéo chi nhánh). Đổi về scope có lọc branch
// thì chi tiết phiếu mất quan hệ, còn tạo phiếu cho đơn chi nhánh khác bị 404 ⇒ test đỏ.
// Test viết TRƯỚC khi sửa (xanh trên code cũ), giữ xanh sau khi sửa.

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func TestGetPaymentDetail_AttachChiNhanhKhac(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	planID, suID := seedPaymentOrderRefs(t, tn)
	now := time.Now().Unix()
	appt := map[string]any{
		"code": fmt.Sprintf("LH%d", time.Now().UnixNano()), "customer_id": tn.CustomerID,
		"branch_id": branchB, "appointment_date": now, "status": 1,
		"created_at": now, "updated_at": now,
	}
	require.NoError(t, testDB.Table("appointment").Create(&appt).Error)
	apptID := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", apptID) })

	// Khách + đơn + dịch vụ ở chi nhánh B; phiếu ở chi nhánh A (tn.BranchID).
	require.NoError(t, testDB.Exec("UPDATE customer SET branch_id = ? WHERE id = ?", branchB, tn.CustomerID).Error)
	require.NoError(t, testDB.Exec("UPDATE order_plan SET branch_id = ? WHERE id = ?", branchB, planID).Error)
	require.NoError(t, testDB.Exec("UPDATE order_service_used SET branch_id = ? WHERE id = ?", branchB, suID).Error)
	payID := seedPaymentRow(t, tn, planID, suID, 300_000, define.PaymentForOrder)
	require.NoError(t, testDB.Exec("UPDATE payment SET appointment_id = ? WHERE id = ?", apptID, payID).Error)

	code, body := callPaymentDetail(t, tn.BranchID, fmt.Sprintf("/%d/detail", payID))
	require.Equal(t, 200, code, "đối chứng: phiếu chi nhánh A đọc được ở chi nhánh A")
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "body=%v", body)
	for _, k := range []string{"customer", "orderPlan", "serviceUsed", "appointment"} {
		rel, ok := data[k].(map[string]any)
		require.True(t, ok && rel["id"] != nil, "%s chi nhánh khác phải được attach, data=%v", k, data)
	}
}

// Tạo phiếu có FK tới record cha (đơn/dịch vụ/khách/phiếu cọc) ở chi nhánh khác chi nhánh active:
// tra cứu FK vẫn thấy record + lấy branch THẬT của record cha (không lấy branch session/payload).
func TestPaymentFKLookup_ChiNhanhKhacVanThay(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	planID, suID := seedPaymentOrderRefs(t, tn)
	require.NoError(t, testDB.Exec("UPDATE order_plan SET branch_id = ? WHERE id = ?", branchB, planID).Error)
	require.NoError(t, testDB.Exec("UPDATE order_service_used SET branch_id = ? WHERE id = ?", branchB, suID).Error)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)
	c.Set("branchActiveId", tn.BranchID)
	c.Set("branchIds", []uint{tn.BranchID})
	h := New(Deps{DB: testDB})

	for _, tc := range []struct {
		table string
		id    uint
	}{{"order_plan", planID}, {"order_service_used", suID}} {
		branchID, ok := h.fkBranch(c, tc.table, int(tc.id))
		require.True(t, ok, "%s chi nhánh khác phải tìm thấy", tc.table)
		require.Equal(t, int(branchB), branchID, "%s: branch lấy từ record cha", tc.table)
	}
	_, ok := h.fkBranch(c, "order_plan", 999999999)
	require.False(t, ok, "record không tồn tại ⇒ không thấy")

	require.True(t, h.serviceUsedIDsInTenant(c, []int{int(suID)}), "dịch vụ chi nhánh khác vẫn hợp lệ")
	require.False(t, h.serviceUsedIDsInTenant(c, []int{int(suID), 999999999}), "lẫn id không tồn tại ⇒ chặn")
}

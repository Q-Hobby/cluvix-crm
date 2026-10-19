//go:build integration

package handler

// lens-money M-14 (2026-10-18): khoá dịch vụ (LockServiceUsed) phải tính lại CẢ debtsPlan của đơn (bản cũ chỉ
// tính khách) và kỳ trả góp của dịch vụ đã khoá không còn hiện là nợ ở InstallmentOfCustomer (dropdown thu
// tiền theo kỳ). Mở khoá ⇒ mọi thứ quay lại.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func TestLockServiceUsed_RecalcOrderDebtAndInstallments(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	planID := seedPlanS37(t, tn, tn.BranchID)
	now := time.Now().Unix()

	// Dịch vụ trả góp 9tr (3 kỳ × 3tr), đã thu 4tr ⇒ nợ 5tr ở khách lẫn đơn.
	osu := map[string]any{"code": fmt.Sprintf("SV%d", time.Now().UnixNano()), "customer_id": tn.CustomerID,
		"order_plan_id": planID, "service_id": 1, "branch_id": tn.BranchID, "quantity": 1,
		"totalEnd": 9_000_000, "payment": 4_000_000, "payment_status": define.ServicePaymentStatusPartly,
		"isLock": define.ServiceUsedNotLock, "created_at": now, "updated_at": now}
	require.NoError(t, testDB.Table("order_service_used").Create(&osu).Error)
	osuID := testhelpers.LastInsertID(testDB)
	require.NoError(t, testDB.Exec("INSERT INTO payment (code, customer_id, branch_id, method, amount, payment_for, parent_id, serviceUsed_id, payment_time, payment_of_date, created_at, updated_at) VALUES (?, ?, ?, 1, 4000000, ?, ?, ?, ?, ?, ?, ?)",
		fmt.Sprintf("TT%d", time.Now().UnixNano()), tn.CustomerID, tn.BranchID, define.PaymentForOrder, planID, osuID, now, now, now, now).Error)
	for i, paid := range []int64{3_000_000, 1_000_000, 0} {
		st := define.InstallmentStatusPaid
		if paid == 0 {
			st = define.InstallmentStatusUnpaid
		} else if paid < 3_000_000 {
			st = define.InstallmentStatusPartial
		}
		require.NoError(t, testDB.Exec("INSERT INTO order_installment (order_service_used_id, period_number, amount, paid_amount, status, due_date) VALUES (?, ?, 3000000, ?, ?, CURDATE())",
			osuID, i, paid, st).Error)
	}
	require.NoError(t, testDB.Exec("UPDATE order_plan SET paymentPlan = 4000000, debtsPlan = 5000000 WHERE id = ?", planID).Error)
	require.NoError(t, testDB.Exec("UPDATE customer SET totalRevenue = 9000000, totalPayment = 4000000, totalDebt = 5000000 WHERE id = ?", tn.CustomerID).Error)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM order_installment WHERE order_service_used_id = ?", osuID)
		testDB.Exec("DELETE FROM payment WHERE serviceUsed_id = ?", osuID)
		testDB.Exec("DELETE FROM order_service_used WHERE id = ?", osuID)
	})

	h := &Handler{DB: testDB}
	ctx := func(method, url string, body any) (*gin.Context, *httptest.ResponseRecorder) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		c.Request = httptest.NewRequest(method, url, &buf)
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("branchActiveId", tn.BranchID)
		c.Set("branchIds", []uint{tn.BranchID})
		c.Set("userID", uint(1))
		return c, w
	}
	toggle := func(curLock int) {
		t.Helper()
		var ver int64
		require.NoError(t, testDB.Raw("SELECT updated_at FROM order_service_used WHERE id = ?", osuID).Scan(&ver).Error)
		c, w := ctx("POST", fmt.Sprintf("/crm/order-service-used/%d/lock-service-used", osuID), map[string]any{"isLock": curLock, "updated_at": ver})
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(osuID), 10)}}
		h.LockServiceUsed(c)
		require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	}
	dec := func(sql string, args ...any) decimal.Decimal {
		t.Helper()
		var d decimal.Decimal
		require.NoError(t, testDB.Raw(sql, args...).Scan(&d).Error)
		return d
	}
	eq := func(want int64, got decimal.Decimal, what string) {
		t.Helper()
		require.True(t, got.Equal(decimal.NewFromInt(want)), "%s: muốn %d, có %s", what, want, got)
	}
	openInstallments := func() int {
		t.Helper()
		c, w := ctx("GET", fmt.Sprintf("/crm/order-service-used/installment-of-customer?customer_id=%d", tn.CustomerID), nil)
		h.InstallmentOfCustomer(c)
		require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
		var items []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
		return len(items)
	}
	require.Equal(t, 2, openInstallments(), "tiền đề: 2 kỳ còn mở (1 phần + chưa thu)")

	// Khoá: totalEnd = 4tr (đã thu), phần còn lại 5tr thành giảm trừ.
	toggle(define.ServiceUsedNotLock)
	eq(4_000_000, dec("SELECT totalEnd FROM order_service_used WHERE id = ?", osuID), "khoá: osu.totalEnd")
	eq(4_000_000, dec("SELECT paymentPlan FROM order_plan WHERE id = ?", planID), "khoá: order.paymentPlan")
	eq(0, dec("SELECT debtsPlan FROM order_plan WHERE id = ?", planID), "khoá: order.debtsPlan (M-14)")
	eq(0, dec("SELECT totalDebt FROM customer WHERE id = ?", tn.CustomerID), "khoá: customer.totalDebt")
	require.Equal(t, 0, openInstallments(), "khoá: kỳ góp của dịch vụ đã khoá không còn là nợ")

	// Mở khoá: quay lại 9tr / nợ 5tr / 2 kỳ mở.
	toggle(define.ServiceUsedIsLock)
	eq(9_000_000, dec("SELECT totalEnd FROM order_service_used WHERE id = ?", osuID), "mở: osu.totalEnd")
	eq(5_000_000, dec("SELECT debtsPlan FROM order_plan WHERE id = ?", planID), "mở: order.debtsPlan")
	eq(5_000_000, dec("SELECT totalDebt FROM customer WHERE id = ?", tn.CustomerID), "mở: customer.totalDebt")
	require.Equal(t, 2, openInstallments(), "mở: 2 kỳ mở trở lại")
}

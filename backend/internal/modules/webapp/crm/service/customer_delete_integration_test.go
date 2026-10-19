//go:build integration

package service

// DeleteCustomerWithPayments: xoá khách KÈM mọi phiếu thu/chi (gắn thẳng khách, theo dòng dịch vụ,
// theo đơn) qua deletePayment. Log: chỉ còn 1 dòng log xoá khách. Fixture logFx: log_delete_integration_test.go.
// (crm-standalone story-09: bỏ bút toán đảo sổ kép + guard kỳ đã khoá cùng module kế toán ⇒ bỏ assert
// event đảo và test "kỳ đã khoá huỷ cả lần xoá".)

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func TestDeleteCustomerWithPayments_DeletesPaymentsAndCleansLogs(t *testing.T) {
	f := newLogFx(t)
	cid := f.tn.CustomerID
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM log_customer WHERE changed_by = ?", f.userID)
	})

	deposit, err := testhelpers.CreatePayment(testDB, f.tn, decimal.NewFromInt(500000), define.PaymentForDeposit)
	require.NoError(t, err)
	planID, osuIDs := f.seedPlan(t, 1)
	byService := f.seed(t, "payment", map[string]any{"branch_id": f.tn.BranchID, "method": 1, "amount": 100000,
		"serviceUsed_id": osuIDs[0], "payment_for": define.PaymentForOrder})
	byPlan := f.seed(t, "payment", map[string]any{"branch_id": f.tn.BranchID, "method": 1, "amount": 0, "parent_id": planID})
	ids := []uint{deposit, byService, byPlan}
	f.oldLog(t, "log_payment", "payment_id", deposit)
	withFKChecks(t)

	require.NoError(t, DeleteCustomerWithPayments(testDB, cid, int(f.userID)))

	require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM customer WHERE id = ?", cid))
	require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM payment WHERE id IN ?", ids), "mọi phiếu của khách phải bị xoá")
	// Story-40 (D15): nhận diện log của test theo changed_by (user tạo riêng cho test).
	require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM log_payment WHERE changed_by = ?", f.userID),
		"xoá khách ⇒ không còn log thanh toán nào (cả log cũ lẫn dòng log xoá phiếu)")
	require.EqualValues(t, 1, countWhere(t, "SELECT COUNT(*) FROM log_customer WHERE changed_by = ? AND action_type = ? AND customer_id IS NULL",
		f.userID, history.ActionDelete), "chỉ còn 1 dòng log xoá khách")
}

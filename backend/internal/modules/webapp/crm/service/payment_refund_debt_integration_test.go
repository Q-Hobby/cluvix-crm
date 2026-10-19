//go:build integration

package service

// story-56 (crm-standalone, D20/D20a): hoàn tiền dịch vụ giảm CẢ doanh thu lẫn đã thu ⇒ khi tạo/sửa/xoá
// phiếu hoàn phải tính lại ngay customer.totalRevenue = ΣtotalEnd − ΣrefundServiceUsed,
// customer.totalDebt = totalRevenue − totalPayment, order_plan.debtsPlan = (ΣtotalEnd − Σrefund) − paymentPlan.

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/errs"
)

type refundDebtFx struct {
	f      *logFx
	svc    *PaymentService
	planID uint
	osuID  int
	cust   int
	now    int64
}

// newRefundDebtFx: 1 đơn, 1 dòng dịch vụ totalEnd = total, khách totalRevenue = total (như sau lưu đơn).
func newRefundDebtFx(t *testing.T, total int64) *refundDebtFx {
	t.Helper()
	f := newLogFx(t)
	planID, osuIDs := f.seedPlan(t, 1)
	osuID := int(osuIDs[0])
	require.NoError(t, testDB.Exec("UPDATE order_service_used SET totalEnd = ?, payment = 0, payment_status = ? WHERE id = ?",
		total, define.ServicePaymentStatusStart, osuID).Error)
	require.NoError(t, testDB.Exec("UPDATE customer SET totalRevenue = ?, totalDebt = ?, totalPayment = 0 WHERE id = ?",
		total, total, f.tn.CustomerID).Error)

	// codegen payment cần config_code khớp chi nhánh (FreshTenant không seed sẵn).
	ts := time.Now().Unix()
	cc56 := testhelpers.WithTenantCols(f.tn, map[string]any{
		"name":         "Payment (s56 tmp)",
		"parent_table": "payment", "infix_code": "TT", "serial_length": 4, "include_branch_code": 0,
		"monthly_reset": 1, "created_at": ts, "updated_at": ts,
	})
	require.NoError(t, testDB.Table("config_code").Create(&cc56).Error)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id IN (SELECT id FROM config_code WHERE branch_id = ?)", f.tn.BranchID)
		testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", f.tn.BranchID)
	})
	return &refundDebtFx{f: f, svc: NewPaymentService(testDB), planID: planID, osuID: osuID,
		cust: int(f.tn.CustomerID), now: time.Now().Unix()}
}

func (x *refundDebtFx) pay(t *testing.T, amount int64) {
	t.Helper()
	tpl := &crmPaymentDao.Payment{CustomerId: &x.cust, BranchId: int(x.f.tn.BranchID), Method: 1,
		Amount: decimal.NewFromInt(amount), PaymentTime: &x.now, PaymentOfDate: &x.now}
	_, err := x.svc.CreatePaymentCustomerFromPayment(tpl, []int{x.osuID}, int(x.f.userID))
	require.NoError(t, err)
}

func (x *refundDebtFx) refund(t *testing.T, amount int64) uint {
	t.Helper()
	osu := x.osuID
	p, err := x.svc.CreateRefundOrderPlanFromPayment(&crmPaymentDao.Payment{CustomerId: &x.cust, ServiceUsedId: &osu,
		BranchId: int(x.f.tn.BranchID), Method: 1, Amount: decimal.NewFromInt(amount),
		PaymentTime: &x.now, PaymentOfDate: &x.now}, int(x.f.userID))
	require.NoError(t, err)
	require.NotZero(t, p.ID)
	return p.ID
}

// check: refund = osu.refundServiceUsed; plan/debts = order_plan; rev/pay/debt = customer.
func (x *refundDebtFx) check(t *testing.T, step string, refund, plan, debts, rev, pay, debt int64) {
	t.Helper()
	var osuRefund, pp, dp, cr, cp, cd decimal.Decimal
	require.NoError(t, testDB.Raw("SELECT COALESCE(refundServiceUsed,0) FROM order_service_used WHERE id = ?", x.osuID).Row().Scan(&osuRefund))
	require.NoError(t, testDB.Raw("SELECT COALESCE(paymentPlan,0), COALESCE(debtsPlan,0) FROM order_plan WHERE id = ?", x.planID).Row().Scan(&pp, &dp))
	require.NoError(t, testDB.Raw("SELECT COALESCE(totalRevenue,0), COALESCE(totalPayment,0), COALESCE(totalDebt,0) FROM customer WHERE id = ?", x.cust).Row().Scan(&cr, &cp, &cd))
	eq := func(want int64, got decimal.Decimal, what string) {
		t.Helper()
		require.True(t, got.Equal(decimal.NewFromInt(want)), "%s %s: muốn %d, có %s", step, what, want, got)
	}
	eq(refund, osuRefund, "osu.refundServiceUsed")
	eq(plan, pp, "order.paymentPlan")
	eq(debts, dp, "order.debtsPlan")
	eq(rev, cr, "customer.totalRevenue")
	eq(pay, cp, "customer.totalPayment")
	eq(debt, cd, "customer.totalDebt")
}

// Trả đủ 1tr ⇒ hoàn 200k ⇒ sửa hoàn 200k→300k ⇒ xoá hoàn (về trạng thái trước hoàn).
func TestPaymentMoney_RefundRecomputeDebt_FullyPaid(t *testing.T) {
	x := newRefundDebtFx(t, 1_000_000)

	x.pay(t, 1_000_000)
	x.check(t, "trả đủ", 0, 1_000_000, 0, 1_000_000, 1_000_000, 0)

	refundID := x.refund(t, 200_000)
	x.check(t, "hoàn 200k", 200_000, 800_000, 0, 800_000, 800_000, 0)

	// Sửa đơn sau hoàn (recalcDebtsPlan) phải giữ cùng công thức ⇒ debtsPlan vẫn 0.
	NewOrderPlanService(testDB).recalcDebtsPlan(testDB, x.planID)
	x.check(t, "recalcDebtsPlan sau hoàn", 200_000, 800_000, 0, 800_000, 800_000, 0)

	_, err := x.svc.UpdatePaymentCustomer(refundID, map[string]interface{}{"amount": 300_000}, int(x.f.userID))
	require.NoError(t, err)
	x.check(t, "sửa hoàn 200k→300k", 300_000, 700_000, 0, 700_000, 700_000, 0)

	require.NoError(t, x.svc.DeleteRefundOrderPlan(testDB, refundID, int(x.f.userID)))
	x.check(t, "xoá hoàn", 0, 1_000_000, 0, 1_000_000, 1_000_000, 0)
}

// Trả 500k ⇒ hoàn 200k ⇒ nợ 500k (doanh thu 800k − đã thu 300k); sửa/xoá hoàn tính lại.
func TestPaymentMoney_RefundRecomputeDebt_PartlyPaid(t *testing.T) {
	x := newRefundDebtFx(t, 1_000_000)

	x.pay(t, 500_000)
	x.check(t, "trả 500k", 0, 500_000, 500_000, 1_000_000, 500_000, 500_000)

	refundID := x.refund(t, 200_000)
	x.check(t, "hoàn 200k", 200_000, 300_000, 500_000, 800_000, 300_000, 500_000)

	_, err := x.svc.UpdatePaymentCustomer(refundID, map[string]interface{}{"amount": 300_000}, int(x.f.userID))
	require.NoError(t, err)
	x.check(t, "sửa hoàn 200k→300k", 300_000, 200_000, 500_000, 700_000, 200_000, 500_000)

	require.NoError(t, x.svc.DeleteRefundOrderPlan(testDB, refundID, int(x.f.userID)))
	x.check(t, "xoá hoàn", 0, 500_000, 500_000, 1_000_000, 500_000, 500_000)
}

// R-3 (re-review 2026-10-18): sửa GIẢM phiếu thanh toán xuống dưới số đã hoàn ⇒ lỗi refund_exceeds_paid,
// không ghi gì (đã thu ròng không âm). Đúng bằng số đã hoàn (biên) vẫn được.
func TestPaymentMoney_UpdateShrinkBelowRefund_Blocked(t *testing.T) {
	x := newRefundDebtFx(t, 5_000_000)

	x.pay(t, 1_000_000)
	var payID uint
	require.NoError(t, testDB.Raw("SELECT id FROM payment WHERE serviceUsed_id = ? AND payment_for = ? ORDER BY id LIMIT 1",
		x.osuID, define.PaymentForOrder).Row().Scan(&payID))
	require.NotZero(t, payID)

	x.refund(t, 500_000)
	x.check(t, "thu 1tr, hoàn 0,5tr", 500_000, 500_000, 4_000_000, 4_500_000, 500_000, 4_000_000)

	_, err := x.svc.UpdatePaymentCustomer(payID, map[string]interface{}{"amount": 200_000}, int(x.f.userID))
	require.ErrorIs(t, err, errs.ErrRefundExceedsPaid, "sửa thu 1tr→0,2tr dưới số đã hoàn 0,5tr phải bị chặn")
	x.check(t, "sau lần sửa bị chặn", 500_000, 500_000, 4_000_000, 4_500_000, 500_000, 4_000_000)
	var amt decimal.Decimal
	require.NoError(t, testDB.Raw("SELECT amount FROM payment WHERE id = ?", payID).Row().Scan(&amt))
	require.True(t, amt.Equal(decimal.NewFromInt(1_000_000)), "phiếu không được đổi, có %s", amt)

	// Biên: giảm về đúng số đã hoàn ⇒ đã thu ròng = 0, hợp lệ.
	_, err = x.svc.UpdatePaymentCustomer(payID, map[string]interface{}{"amount": 500_000}, int(x.f.userID))
	require.NoError(t, err)
	var cp decimal.Decimal
	require.NoError(t, testDB.Raw("SELECT COALESCE(totalPayment,0) FROM customer WHERE id = ?", x.cust).Row().Scan(&cp))
	require.True(t, cp.IsZero(), "totalPayment sau khi giảm về đúng số đã hoàn phải = 0, có %s", cp)
}

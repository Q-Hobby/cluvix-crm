//go:build integration

package service

// lens-money M-13 (user chốt 2026-10-18 "trừ cả doanh số và hoa hồng"): phiếu hoàn dịch vụ ghi dòng ÂM vào
// order_revenue_telesale + order_commission_telesale theo tỷ lệ phần hoàn, chia đều (người cuối nhận phần dư);
// xoá phiếu hoàn ⇒ gỡ các dòng âm. Chạy trên pool prod-like (10 kết nối, FK bật).

import (
	"testing"

	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

// telesaleRefundFx — dòng dịch vụ 1.000.001, 3 tư vấn viên, hoa hồng snapshot 10% — đã thu đủ.
type telesaleRefundFx struct {
	*moneyFx
	osuID int
	users []int
}

func newTelesaleRefundFx(t *testing.T) *telesaleRefundFx {
	t.Helper()
	x := newMoneyFx(t)
	t.Cleanup(func() { testDB.Exec("DELETE FROM order_commission_telesale WHERE customer_id = ?", x.cust) })
	svcRow := map[string]any{"code": "M13T" + itoa(int(x.now%100000)), "name": "M-13 refund telesale"}
	require.NoError(t, testDB.Table("service").Create(&svcRow).Error)
	svcID := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM service WHERE id = ?", svcID) })

	users := []int{
		int(testhelpers.CreateUser(t, testDB, "m13a", x.tn)),
		int(testhelpers.CreateUser(t, testDB, "m13b", x.tn)),
		int(testhelpers.CreateUser(t, testDB, "m13c", x.tn)),
	}
	planID, ids := x.services(t, 1, 1_000_001)
	require.NoError(t, testDB.Exec("UPDATE order_plan SET customer_id = ? WHERE id = ?", x.cust, planID).Error)
	list := "[" + itoa(users[0]) + "," + itoa(users[1]) + "," + itoa(users[2]) + "]"
	require.NoError(t, testDB.Exec("UPDATE order_service_used SET service_id = ?, listRevenues = ?, commission_telesale_snap = 10, commission_telesale_type_snap = ? WHERE id = ?",
		svcID, list, define.DiscountTypePercent, ids[0]).Error)
	x.pay(t, ids[0], 1_000_001)
	return &telesaleRefundFx{moneyFx: x, osuID: ids[0], users: users}
}

func (f *telesaleRefundFx) refund(t *testing.T, amount int64) error {
	t.Helper()
	p := f.tpl(amount)
	osu := f.osuID
	p.ServiceUsedId = &osu
	_, err := f.svc.CreateRefundOrderPlanFromPayment(p, int(f.userID))
	return err
}

// net — tổng ròng doanh số / hoa hồng (cả dòng) và theo từng tư vấn viên.
func (f *telesaleRefundFx) requireNet(t *testing.T, step string, revenue, commission int64, perUserRevenue []int64) {
	t.Helper()
	requireDec(t, revenue, f.dec(t, "SELECT COALESCE(SUM(revenue),0) FROM order_revenue_telesale WHERE order_service_used_id = ?", f.osuID), step+": Σ doanh số")
	requireDec(t, commission, f.dec(t, "SELECT COALESCE(SUM(commissionTelesale),0) FROM order_commission_telesale WHERE order_service_used_id = ?", f.osuID), step+": Σ hoa hồng")
	for i, u := range f.users {
		requireDec(t, perUserRevenue[i], f.dec(t, "SELECT COALESCE(SUM(revenue),0) FROM order_revenue_telesale WHERE order_service_used_id = ? AND revenue_id = ?", f.osuID, u), step+": doanh số TVV")
	}
	// Đối chiếu: Σ doanh số = đã thu RÒNG của dòng dịch vụ (thanh toán − hoàn).
	requireDec(t, revenue, f.dec(t, "SELECT COALESCE(SUM(amount),0) FROM payment WHERE serviceUsed_id = ? AND payment_for IN ?", f.osuID, netPaidPaymentFors), step+": Σ doanh số = đã thu ròng")
}

func TestPaymentMoney_RefundReversesTelesaleRevenueAndCommission(t *testing.T) {
	f := newTelesaleRefundFx(t)
	// Thu 1.000.001: doanh số 333.333/333.333/333.335; hoa hồng 10% = 100.000 ⇒ 33.333/33.333/33.334.
	f.requireNet(t, "thu đủ", 1_000_001, 100_000, []int64{333_333, 333_333, 333_335})

	// Hoàn 300.001: doanh số −100.000/−100.000/−100.001; hoa hồng −30.000 ⇒ −10.000 × 3.
	require.NoError(t, f.refund(t, 300_001))
	f.requireNet(t, "hoàn 1 phần", 700_000, 70_000, []int64{233_333, 233_333, 233_334})
	var refundID uint
	require.NoError(t, testDB.Raw("SELECT id FROM payment WHERE serviceUsed_id = ? AND payment_for = ?", f.osuID, define.PaymentForRefundService).Scan(&refundID).Error)
	requireDec(t, 3, f.dec(t, "SELECT COUNT(*) FROM order_revenue_telesale WHERE payment_id = ? AND revenue < 0", refundID), "3 dòng doanh số âm")
	requireDec(t, 3, f.dec(t, "SELECT COUNT(*) FROM order_commission_telesale WHERE payment_id = ? AND commissionTelesale < 0", refundID), "3 dòng hoa hồng âm")

	// Xoá phiếu hoàn ⇒ quay về như trước khi hoàn.
	require.NoError(t, f.svc.DeleteRefundOrderPlan(f.pool, refundID, int(f.userID)))
	f.requireNet(t, "xoá phiếu hoàn", 1_000_001, 100_000, []int64{333_333, 333_333, 333_335})
	requireDec(t, 0, f.dec(t, "SELECT COUNT(*) FROM order_revenue_telesale WHERE payment_id = ?", refundID), "không còn dòng doanh số của phiếu hoàn")
	requireDec(t, 0, f.dec(t, "SELECT COUNT(*) FROM order_commission_telesale WHERE payment_id = ?", refundID), "không còn dòng hoa hồng của phiếu hoàn")
}

// Đồng thời: 5 phiếu hoàn 100.000 cùng lúc ⇒ mỗi phiếu thành công có đúng 3 dòng âm, tổng ròng khớp đã thu ròng.
func TestPaymentConcurrency_RefundTelesaleNet(t *testing.T) {
	f := newTelesaleRefundFx(t)
	res := race(5, func(int) error { return f.refund(t, 100_000) })
	requireOnlyExpected(t, res)
	ok := int64(countOK(res))
	require.Positive(t, ok)
	f.requireNet(t, "5 hoàn song song", 1_000_001-ok*100_000, 100_000-ok*10_000,
		[]int64{333_333 - ok*33_333, 333_333 - ok*33_333, 333_335 - ok*33_334})
	requireDec(t, ok*3, f.dec(t, "SELECT COUNT(*) FROM order_revenue_telesale r JOIN payment p ON p.id = r.payment_id WHERE r.order_service_used_id = ? AND p.payment_for = ?", f.osuID, define.PaymentForRefundService), "3 dòng âm / phiếu hoàn")
}

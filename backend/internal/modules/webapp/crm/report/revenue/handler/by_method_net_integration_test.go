//go:build integration

package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/shopspring/decimal"

	"erpCluvix/pkg/define"
)

// lens-money M-12: báo cáo Hình thức thanh toán phải giữ phiếu ÂM (hoàn DV / hoàn cọc lưu số âm theo
// quy ước production) để tổng ròng đúng. Kịch bản E2E của lens: nhận cọc 5tr + thanh toán 6tr − hoàn DV
// 0.5tr − hoàn cọc 1tr = 9.5tr tiền thật; chuyển cọc 4tr là bút toán nội bộ (FE không cộng vào Tổng).
// Bản cũ (`HAVING SUM > 0`) bỏ 2 dòng âm ⇒ màn hình báo 15tr.
func TestPaymentByMethod_NetTotalKeepsNegatives(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seedRevenue(t)
	mustExec(t, "DELETE FROM payment")
	mustExec(t, "DELETE FROM payment_method WHERE id IN (3, 4)")
	mustExec(t, "INSERT INTO payment_method (id, name) VALUES (3, 'Chuyển cọc (M12)'), (4, 'Tiền mặt (M12)')")
	t.Cleanup(func() { testDB.Exec("DELETE FROM payment_method WHERE id IN (3, 4)") })

	ins := func(id, method, paymentFor int, amount string) {
		t.Helper()
		mustExec(t, `INSERT INTO payment (id, code, customer_id, branch_id, method, amount, payment_for, payment_time, created_at, updated_at)
			VALUES (?, ?, 901, ?, ?, ?, ?, ?, ?, ?)`, id, fmt.Sprintf("PT%d", id), revB1, method, amount, paymentFor, revT, revT, revT)
	}
	ins(9501, 4, define.PaymentForDeposit, "5000000")
	ins(9502, 4, define.PaymentForOrder, "6000000")
	ins(9503, 3, define.PaymentForMoveDeposit, "4000000")
	ins(9504, 4, define.PaymentForRefundService, "-500000")
	ins(9505, 4, define.PaymentForRefundDeposit, "-1000000")

	var rows []struct {
		MethodID   int         `json:"method_id"`
		PaymentFor int         `json:"payment_for"`
		Amount     json.Number `json:"amount"`
	}
	decodeJSON(t, call(t, (*Handler).ReportPaymentByMethod, fmt.Sprintf("from=%d&to=%d", revFrom, revTo), revB1), &rows)

	got := map[int]decimal.Decimal{}
	net := decimal.Zero // tổng tiền thật = mọi dòng trừ chuyển cọc (cùng định nghĩa cột Tổng của FE)
	for _, r := range rows {
		a := decimal.RequireFromString(r.Amount.String())
		got[r.PaymentFor] = got[r.PaymentFor].Add(a)
		if r.PaymentFor != define.PaymentForMoveDeposit {
			net = net.Add(a)
		}
	}
	want := map[int]string{
		define.PaymentForDeposit: "5000000", define.PaymentForOrder: "6000000", define.PaymentForMoveDeposit: "4000000",
		define.PaymentForRefundService: "-500000", define.PaymentForRefundDeposit: "-1000000",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, muốn đủ %d loại phiếu (kể cả dòng âm)", got, len(want))
	}
	for k, v := range want {
		if !got[k].Equal(decimal.RequireFromString(v)) {
			t.Errorf("payment_for %d = %s, muốn %s", k, got[k], v)
		}
	}
	if !net.Equal(decimal.NewFromInt(9_500_000)) {
		t.Errorf("tổng ròng = %s, muốn 9500000", net)
	}
}

//go:build integration

package service

// Lưới TIỀN hoa hồng + doanh số TELESALE (crm-standalone story-14, D7): gỡ hoa hồng direct-sale /
// bác sĩ / phụ tá / KTV và liệu trình KHÔNG được đổi 1 đồng nào của order_commission_telesale /
// order_revenue_telesale. Số expected dưới đây được chốt bằng cách chạy test này trên code TRƯỚC khi
// xoá (baseline 7db8952 + story-13) rồi giữ nguyên sau khi xoá — đổi số ở đây = đổi tiền, phải có
// quyết định của user.
// Đổi số duy nhất từ đó: lens-money M-18 (user duyệt 2026-10-18) — doanh số order_revenue_telesale
// chia theo splitEvenLastRemainder như hoa hồng ⇒ Σ doanh số = đúng số tiền phiếu (lệch tối đa vài đồng/phiếu).
//
// Phủ: snapshot VND (DiscountTypeNormal, prorate theo totalEnd), fallback live `service.commission_
// telesale_new` kiểu % khi OSU chưa có snapshot, phần dư dồn người cuối (splitEvenLastRemainder), sửa
// phiếu (UpdatePaymentCustomer), resyncRevenueForOSU khi đổi listRevenues.

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

type telesaleRow struct {
	PaymentID uint
	RevenueID int
	Amount    decimal.Decimal
	Value     decimal.Decimal
}

func readTelesaleRows(t *testing.T, table, valueCol string, osuID int) []telesaleRow {
	t.Helper()
	rows, err := testDB.Raw("SELECT payment_id, revenue_id, amount, "+valueCol+" FROM "+table+
		" WHERE order_service_used_id = ? ORDER BY payment_id, id", osuID).Rows()
	require.NoError(t, err)
	defer rows.Close()
	var out []telesaleRow
	for rows.Next() {
		var r telesaleRow
		require.NoError(t, rows.Scan(&r.PaymentID, &r.RevenueID, &r.Amount, &r.Value))
		out = append(out, r)
	}
	return out
}

func requireTelesaleRows(t *testing.T, what string, got []telesaleRow, want [][3]int64) {
	t.Helper()
	require.Len(t, got, len(want), what)
	for i, w := range want {
		require.Equal(t, int(w[0]), got[i].RevenueID, "%s[%d] revenue_id", what, i)
		require.True(t, got[i].Amount.Equal(decimal.NewFromInt(w[1])), "%s[%d] amount: muốn %d, có %s", what, i, w[1], got[i].Amount)
		require.True(t, got[i].Value.Equal(decimal.NewFromInt(w[2])), "%s[%d] value: muốn %d, có %s", what, i, w[2], got[i].Value)
	}
}

func TestPaymentMoney_TelesaleCommissionRevenueParity(t *testing.T) {
	f := newLogFx(t)

	ts := time.Now().Unix()
	cc14 := testhelpers.WithTenantCols(f.tn, map[string]any{
		"name":         "Payment (s14 tmp)",
		"parent_table": "payment", "infix_code": "TT", "serial_length": 4, "include_branch_code": 0,
		"monthly_reset": 1, "created_at": ts, "updated_at": ts,
	})
	require.NoError(t, testDB.Table("config_code").Create(&cc14).Error)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id IN (SELECT id FROM config_code WHERE branch_id = ?)", f.tn.BranchID)
		testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", f.tn.BranchID)
		testDB.Exec("DELETE FROM order_commission_telesale WHERE customer_id = ?", f.tn.CustomerID)
		testDB.Exec("DELETE FROM order_revenue_telesale WHERE customer_id = ?", f.tn.CustomerID)
	})

	u1 := int(testhelpers.CreateUser(t, testDB, "tele1", f.tn))
	u2 := int(testhelpers.CreateUser(t, testDB, "tele2", f.tn))
	u3 := int(testhelpers.CreateUser(t, testDB, "tele3", f.tn))

	// Dịch vụ riêng cho nhánh fallback live (không snapshot): rate cũ 50_000 VND khác rate mới 7.5% —
	// đơn loại New phải lấy commission_telesale_new.
	svcRow := map[string]any{
		"code": "S14T" + itoa(int(ts%100000)), "name": "Telesale parity (s14)",
		"commission_telesale": 50_000, "commission_telesale_new": 7.5, "commission_telesale_type": define.DiscountTypePercent,
	}
	require.NoError(t, testDB.Table("service").Create(&svcRow).Error)
	svcID := testhelpers.LastInsertID(testDB)
	require.NotZero(t, svcID)
	t.Cleanup(func() { testDB.Exec("DELETE FROM service WHERE id = ?", svcID) })

	planID := f.seed(t, "order_plan", map[string]any{"branch_id": f.tn.BranchID, "orderPlanType": define.OrderPlanTypeNew})
	// OSU1: snapshot VND 100_001, 3 telesale, totalEnd 1_000_000.
	osu1 := int(f.seed(t, "order_service_used", map[string]any{
		"order_plan_id": planID, "service_id": svcID, "branch_id": f.tn.BranchID, "quantity": 1,
		"totalEnd": 1_000_000, "payment": 0, "payment_status": define.ServicePaymentStatusStart,
		"listRevenues":                  []byte(`[` + itoa(u1) + `,` + itoa(u2) + `,` + itoa(u3) + `]`),
		"commission_telesale_snap":      100_001,
		"commission_telesale_type_snap": define.DiscountTypeNormal,
	}))
	// OSU2: KHÔNG snapshot ⇒ fallback live service.commission_telesale_new (7.5%), 2 telesale.
	osu2 := int(f.seed(t, "order_service_used", map[string]any{
		"order_plan_id": planID, "service_id": svcID, "branch_id": f.tn.BranchID, "quantity": 1,
		"totalEnd": 500_000, "payment": 0, "payment_status": define.ServicePaymentStatusStart,
		"listRevenues": []byte(`[` + itoa(u1) + `,` + itoa(u2) + `]`),
	}))
	require.NoError(t, testDB.Exec("UPDATE customer SET totalRevenue = ? WHERE id = ?", 1_500_000, f.tn.CustomerID).Error)

	svc := NewPaymentService(testDB)
	cust := int(f.tn.CustomerID)
	now := time.Now().Unix()
	pay := func(osuID int, amount int64) {
		t.Helper()
		tpl := &crmPaymentDao.Payment{CustomerId: &cust, BranchId: int(f.tn.BranchID), Method: 1,
			Amount: decimal.NewFromInt(amount), PaymentTime: &now, PaymentOfDate: &now}
		_, err := svc.CreatePaymentCustomerFromPayment(tpl, []int{osuID}, int(f.userID))
		require.NoError(t, err)
	}

	pay(osu1, 333_333)
	pay(osu1, 666_667)
	pay(osu2, 333_333)

	// OSU1 VND: 333_333 → %=33.33 → 33_330 → 11_110×3; 666_667 → %=66.67 → 66_671 → 22_223,22_223,22_225.
	requireTelesaleRows(t, "osu1 commission", readTelesaleRows(t, "order_commission_telesale", "commissionTelesale", osu1), [][3]int64{
		{int64(u1), 333_333, 11_110}, {int64(u2), 333_333, 11_110}, {int64(u3), 333_333, 11_110},
		{int64(u1), 666_667, 22_223}, {int64(u2), 666_667, 22_223}, {int64(u3), 666_667, 22_225},
	})
	requireTelesaleRows(t, "osu1 revenue", readTelesaleRows(t, "order_revenue_telesale", "revenue", osu1), [][3]int64{
		{int64(u1), 333_333, 111_111}, {int64(u2), 333_333, 111_111}, {int64(u3), 333_333, 111_111},
		{int64(u1), 666_667, 222_222}, {int64(u2), 666_667, 222_222}, {int64(u3), 666_667, 222_223},
	})
	// OSU2 %: 333_333 × 7.5% = 24_999.975 → 25_000 → 12_500×2; doanh số chia đều, người cuối nhận phần dư
	// (lens-money M-18): 166_666 + 166_667 = 333_333 (bản cũ Round(0) từng người ⇒ 166_667×2 = 333_334).
	requireTelesaleRows(t, "osu2 commission", readTelesaleRows(t, "order_commission_telesale", "commissionTelesale", osu2), [][3]int64{
		{int64(u1), 333_333, 12_500}, {int64(u2), 333_333, 12_500},
	})
	requireTelesaleRows(t, "osu2 revenue", readTelesaleRows(t, "order_revenue_telesale", "revenue", osu2), [][3]int64{
		{int64(u1), 333_333, 166_666}, {int64(u2), 333_333, 166_667},
	})

	// Sửa phiếu OSU2 333_333 → 200_001: 15_000.075 → 15_000 → 7_500×2; doanh số 100_000 + 100_001 (M-18).
	var p2 uint
	require.NoError(t, testDB.Raw("SELECT id FROM payment WHERE serviceUsed_id = ? ORDER BY id DESC LIMIT 1", osu2).Scan(&p2).Error)
	require.NotZero(t, p2)
	_, err := svc.UpdatePaymentCustomer(p2, map[string]interface{}{"amount": 200_001}, int(f.userID))
	require.NoError(t, err)
	requireTelesaleRows(t, "osu2 commission sau sửa", readTelesaleRows(t, "order_commission_telesale", "commissionTelesale", osu2), [][3]int64{
		{int64(u1), 200_001, 7_500}, {int64(u2), 200_001, 7_500},
	})
	requireTelesaleRows(t, "osu2 revenue sau sửa", readTelesaleRows(t, "order_revenue_telesale", "revenue", osu2), [][3]int64{
		{int64(u1), 200_001, 100_000}, {int64(u2), 200_001, 100_001},
	})

	// Đổi listRevenues OSU1 → [u3] rồi resync: doanh số dồn hết cho u3 theo từng phiếu; hoa hồng giữ nguyên.
	require.NoError(t, testDB.Exec("UPDATE order_service_used SET listRevenues = ? WHERE id = ?", `[`+itoa(u3)+`]`, osu1).Error)
	// Story-37/62: resync chỉ nhận OSU/đơn/khách.
	resyncRevenueForOSU(testDB, osu1, int(planID), cust)
	requireTelesaleRows(t, "osu1 revenue sau resync", readTelesaleRows(t, "order_revenue_telesale", "revenue", osu1), [][3]int64{
		{int64(u3), 333_333, 333_333}, {int64(u3), 666_667, 666_667},
	})
	require.Len(t, readTelesaleRows(t, "order_commission_telesale", "commissionTelesale", osu1), 6, "resync không đụng hoa hồng")
}

func itoa(n int) string { return decimal.NewFromInt(int64(n)).String() }

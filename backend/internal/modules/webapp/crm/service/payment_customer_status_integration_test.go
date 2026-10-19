//go:build integration

package service

// crm-standalone story-14 (D1): quy tắc nâng khách lên status "Khách hàng" trong updateCustomer.
// Quy tắc cũ = "có dòng dịch vụ thanh toán đủ (totalEnd = payment) VÀ hoàn thành điều trị (theo khoa)";
// liệu trình + khoa đã bỏ ⇒ giữ NGUYÊN vế thanh toán đủ, bỏ vế hoàn thành điều trị.
//   - trả thiếu            ⇒ status không đổi;
//   - trả đủ totalEnd      ⇒ status = Khách hàng;
//   - accepted != Ok       ⇒ không nâng (điều kiện cũ giữ nguyên);
//   - status thuộc skip    ⇒ không đổi (Khiếu nại… — điều kiện cũ giữ nguyên).

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func TestPaymentMoney_CustomerStatusPromotedWhenServiceFullyPaid(t *testing.T) {
	cases := []struct {
		name       string
		accepted   int
		status     int
		pays       []int64
		wantStatus int
	}{
		{"trả thiếu ⇒ giữ status", define.CustomerAcceptedOk, define.CustomerStatusContact, []int64{400_000}, define.CustomerStatusContact},
		{"trả đủ ⇒ Khách hàng", define.CustomerAcceptedOk, define.CustomerStatusContact, []int64{400_000, 600_000}, define.CustomerStatusCustomer},
		{"chưa accepted ⇒ không nâng", 0, define.CustomerStatusContact, []int64{1_000_000}, define.CustomerStatusContact},
		{"status Khiếu nại ⇒ không đổi", define.CustomerAcceptedOk, define.CustomerStatusComplaint, []int64{1_000_000}, define.CustomerStatusComplaint},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLogFx(t)
			_, osuIDs := f.seedPlan(t, 1)
			osuID := int(osuIDs[0])
			// Quy tắc INNER JOIN service (giữ nguyên từ bản cũ) ⇒ dòng phải trỏ tới dịch vụ có thật.
			svcRow := map[string]any{
				"code": "S14S" + itoa(osuID), "name": "Status rule (s14)",
			}
			require.NoError(t, testDB.Table("service").Create(&svcRow).Error)
			svcID := testhelpers.LastInsertID(testDB)
			t.Cleanup(func() { testDB.Exec("DELETE FROM service WHERE id = ?", svcID) })
			require.NoError(t, testDB.Exec("UPDATE order_service_used SET service_id = ?, totalEnd = ?, payment = 0, payment_status = ? WHERE id = ?",
				svcID, 1_000_000, define.ServicePaymentStatusStart, osuID).Error)
			require.NoError(t, testDB.Exec("UPDATE customer SET totalRevenue = ?, accepted = ?, status = ? WHERE id = ?",
				1_000_000, tc.accepted, tc.status, f.tn.CustomerID).Error)

			ts := time.Now().Unix()
			ccRow := testhelpers.WithTenantCols(f.tn, map[string]any{
				"name":         "Payment (s14 status)",
				"parent_table": "payment", "infix_code": "TT", "serial_length": 4, "include_branch_code": 0,
				"monthly_reset": 1, "created_at": ts, "updated_at": ts,
			})
			require.NoError(t, testDB.Table("config_code").Create(&ccRow).Error)
			t.Cleanup(func() {
				testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id IN (SELECT id FROM config_code WHERE branch_id = ?)", f.tn.BranchID)
				testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", f.tn.BranchID)
			})

			svc := NewPaymentService(testDB)
			cust := int(f.tn.CustomerID)
			now := time.Now().Unix()
			for _, amt := range tc.pays {
				tpl := &crmPaymentDao.Payment{CustomerId: &cust, BranchId: int(f.tn.BranchID), Method: 1,
					Amount: decimal.NewFromInt(amt), PaymentTime: &now, PaymentOfDate: &now}
				_, err := svc.CreatePaymentCustomerFromPayment(tpl, []int{osuID}, int(f.userID))
				require.NoError(t, err)
			}

			var got int
			require.NoError(t, testDB.Raw("SELECT COALESCE(status,0) FROM customer WHERE id = ?", f.tn.CustomerID).Scan(&got).Error)
			require.Equal(t, tc.wantStatus, got)
		})
	}
}

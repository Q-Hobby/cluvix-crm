//go:build integration

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/tz"
)

// Tạo đơn hàng cho khách CHƯA có mã ⇒ sinh mã khách theo chi nhánh CỦA KHÁCH; chi nhánh của đơn chỉ
// dùng khi khách chưa có chi nhánh (và khi đó được gán luôn cho khách). Quy tắc:
// skill/skills/working-on-crm/reference/customer.md §Mã khách hàng.
func TestOrderCreate_CustomerCodeFollowsCustomerBranch(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	t.Setenv(define.CustomerCodeOptionEnv, "") // option mặc định: theo config_code

	// Service đọc biến toàn cục logger.L (nil trong test ⇒ panic ở nhánh log) và db.DB — trỏ tạm vào
	// logger test + DB test, trả lại khi xong.
	logger.Init(logger.Config{Env: "test", AppCode: "crm_service_test"})
	origDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = origDB })

	cases := []struct {
		name              string
		customerHasBranch bool // true: khách đã thuộc chi nhánh A, đơn tạo ở chi nhánh B
	}{
		{"khách chưa có chi nhánh ⇒ theo chi nhánh đơn", false},
		{"khách đã có chi nhánh ⇒ theo chi nhánh khách, không theo đơn", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tenant := testhelpers.FreshTenant(t, testDB)
			branchA := tenant.BranchID
			branchB := testhelpers.CreateBranch(t, testDB)

			for _, b := range []uint{branchA, branchB} {
				require.NoError(t, testDB.Exec(`INSERT INTO config_code
					(branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
					VALUES (?, 'Khách hàng', 'customer', 'KH', 1, 1, 2, 0, 0),
					       (?, 'Đơn hàng', 'order_plan', 'DH', 1, 1, 2, 0, 0)`, b, b).Error)
			}
			t.Cleanup(func() {
				testDB.Exec("DELETE FROM order_plan WHERE customer_id = ?", tenant.CustomerID)
				testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id IN (SELECT id FROM config_code WHERE branch_id IN (?, ?))", branchA, branchB)
				testDB.Exec("DELETE FROM config_code WHERE branch_id IN (?, ?)", branchA, branchB)
			})

			wantBranch := branchB
			if c.customerHasBranch {
				wantBranch = branchA
				require.NoError(t, testDB.Exec("UPDATE customer SET branch_id = ? WHERE id = ?", branchA, tenant.CustomerID).Error)
			}

			req := &OrderPlanRequest{}
			req.CustomerId = int(tenant.CustomerID)
			req.BranchId = new(int(branchB))
			req.CreatedTime = new(time.Now().Unix())
			require.NoError(t, NewOrderPlanService(testDB).Create(req, 1))

			var cust struct {
				BranchID *uint  `gorm:"column:branch_id"`
				Code     string `gorm:"column:code"`
			}
			require.NoError(t, testDB.Table("customer").Select("branch_id, COALESCE(code, '') AS code").
				Where("id = ?", tenant.CustomerID).Scan(&cust).Error)

			var branchCode string
			require.NoError(t, testDB.Table("branch").Select("code").Where("id = ?", wantBranch).Scan(&branchCode).Error)
			ym := tz.Now(testDB).Format("0601") // story-40: tz theo thương hiệu (business_profile)

			require.NotNil(t, cust.BranchID)
			require.Equal(t, wantBranch, *cust.BranchID, "chi nhánh khách")
			require.Equal(t, strings.ToUpper(branchCode+".KH."+ym+".01"), cust.Code, "mã khách phải mang mã chi nhánh của khách")
		})
	}
}

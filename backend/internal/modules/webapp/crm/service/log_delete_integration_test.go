//go:build integration

package service

// Xoá bản ghi CRM PHẢI để lại đúng 1 dòng log action_type=delete (history.LogDeleteSnapshot):
// fk = NULL sau khi xoá (FK ON DELETE SET NULL), snapshot id/customer_id.
// Thứ tự bắt buộc ở mọi chỗ xoá: dọn log cũ → ghi dòng xoá → xoá bản ghi — đảo 2 bước đầu thì chính
// dòng xoá bị dọn mất mà build/test cũ vẫn xanh. Skill: debugging-backend-queries/reference/log-history.md.
//
// Suite tắt foreign_key_checks cho cả phiên ⇒ SET NULL KHÔNG chạy. Mọi test bật lại checks SAU khi
// seed (withFKChecks) — không bật thì "fk NULL sau khi xoá" xanh giả hoặc đỏ vì lý do sai.

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"erpCluvix/db"
	apptSvc "erpCluvix/internal/modules/webapp/crm/appointment/service"
	orderPlanDao "erpCluvix/internal/modules/webapp/crm/order_plan/dao"
	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/logger"
)

type logFx struct {
	tn     *testhelpers.TestTenant
	userID uint
}

// newLogFx: tenant + user thật (changed_by là FK RESTRICT → user khi checks bật) + trỏ db.DB/logger.L
// toàn cục vào DB/logger test như order_customer_code_integration_test.go.
func newLogFx(t *testing.T) *logFx {
	t.Helper()
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	logger.Init(logger.Config{Env: "test", AppCode: "crm_service_test"})
	origDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = origDB })
	tn := testhelpers.FreshTenant(t, testDB)
	f := &logFx{tn: tn, userID: testhelpers.CreateUser(t, testDB, "logdel", tn)}
	t.Cleanup(func() {
		for _, tbl := range []string{"log_appointment", "log_payment", "log_order_service_used"} {
			// Log của test nhận diện theo changed_by (user riêng) hoặc snapshot customer_id của dòng xoá.
			testDB.Exec("DELETE FROM "+tbl+" WHERE changed_by = ? OR JSON_VALUE(changed_fields, '$.customer_id.from' RETURNING UNSIGNED) = ?", f.userID, tn.CustomerID)
		}
		for _, tbl := range []string{"payment", "order_service_used", "appointment", "order_plan"} {
			testDB.Exec("DELETE FROM "+tbl+" WHERE customer_id = ?", tn.CustomerID)
		}
	})
	return f
}

func (f *logFx) seed(t *testing.T, table string, row map[string]any) uint {
	t.Helper()
	if row == nil {
		row = map[string]any{}
	}
	if _, ok := row["customer_id"]; !ok {
		row["customer_id"] = f.tn.CustomerID
	}
	if _, ok := row["code"]; !ok {
		row["code"] = fmt.Sprintf("T%d", time.Now().UnixNano())
	}
	require.NoError(t, testDB.Table(table).Create(&row).Error)
	return testhelpers.LastInsertID(testDB)
}

// oldLog chèn 1 dòng log update "cũ" trỏ tới bản ghi — để chứng minh bước dọn log cũ có chạy.
func (f *logFx) oldLog(t *testing.T, table, fkCol string, id uint) {
	t.Helper()
	require.NoError(t, testDB.Exec("INSERT INTO "+table+" ("+fkCol+", action_type, changed_fields, changed_by, changed_at) VALUES (?, ?, ?, ?, ?)",
		id, history.ActionUpdate, `{"note": {"from": "a", "to": "b"}}`, f.userID, time.Now().Unix()).Error)
}

func withFKChecks(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec("SET SESSION foreign_key_checks = 1").Error)
	t.Cleanup(func() { testDB.Exec("SET SESSION foreign_key_checks = 0") })
}

// requireDeleteLog: đúng 1 dòng xoá (fk NULL, snapshot id + customer_id);
// keepOld=false ⇒ không còn dòng log nào trỏ về id (log cũ đã dọn / đã SET NULL).
func (f *logFx) requireDeleteLog(t *testing.T, table, fkCol string, id uint) {
	t.Helper()
	var r struct {
		Cnt, FkNull int
		SnapCust    *uint
	}
	require.NoError(t, testDB.Raw("SELECT COUNT(*) cnt, COALESCE(SUM("+fkCol+" IS NULL), 0) fk_null, "+
		"MAX(JSON_VALUE(changed_fields, '$.customer_id.from' RETURNING UNSIGNED)) snap_cust FROM "+table+
		" WHERE action_type = ? AND JSON_VALUE(changed_fields, '$.id.from' RETURNING UNSIGNED) = ?", history.ActionDelete, id).Scan(&r).Error)
	require.Equal(t, 1, r.Cnt, "%s#%d: phải có đúng 1 dòng log xoá", table, id)
	require.Equal(t, 1, r.FkNull, "%s#%d: dòng xoá phải có fk NULL sau khi xoá (FK SET NULL)", table, id)
	require.NotNil(t, r.SnapCust, "%s#%d: thiếu snapshot customer_id", table, id)
	require.Equal(t, f.tn.CustomerID, *r.SnapCust)
}

func countWhere(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Raw(sql, args...).Scan(&n).Error)
	return n
}

func TestLogDelete_Appointment(t *testing.T) {
	f := newLogFx(t)
	id := f.seed(t, "appointment", map[string]any{"branch_id": f.tn.BranchID, "status": define.AppointmentStatusNotCheckin})
	f.oldLog(t, "log_appointment", "appointment_id", id)
	f.oldLog(t, "log_appointment", "appointment_id", id)
	withFKChecks(t)

	require.NoError(t, apptSvc.NewAppointmentService(testDB).Delete(id, int(f.userID), testDB))

	require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM appointment WHERE id = ?", id))
	f.requireDeleteLog(t, "log_appointment", "appointment_id", id)
	require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM log_appointment WHERE changed_by = ? AND action_type = ?", f.userID, history.ActionUpdate),
		"log cũ của lịch hẹn phải được dọn")
}

func TestLogDelete_Payment_KeepsOldLogs(t *testing.T) {
	f := newLogFx(t)
	payID, err := testhelpers.CreatePayment(testDB, f.tn, decimal.NewFromInt(100000), define.PaymentForDeposit)
	require.NoError(t, err)
	f.oldLog(t, "log_payment", "payment_id", payID)
	withFKChecks(t)

	var p crmPaymentDao.Payment
	require.NoError(t, testDB.First(&p, payID).Error)
	require.NoError(t, (&PaymentService{DB: testDB}).deletePayment(&p, int(f.userID)))

	require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM payment WHERE id = ?", payID))
	f.requireDeleteLog(t, "log_payment", "payment_id", payID)
	// Thanh toán GIỮ log cũ (khác 3 bảng còn lại) — FK SET NULL gỡ payment_id thay vì chặn xoá.
	require.EqualValues(t, 1, countWhere(t, "SELECT COUNT(*) FROM log_payment WHERE changed_by = ? AND action_type = ? AND payment_id IS NULL",
		f.userID, history.ActionUpdate))
}

// seedPlan: đơn + n dòng dịch vụ (chưa điều trị, chưa thanh toán ⇒ xoá được).
func (f *logFx) seedPlan(t *testing.T, n int) (uint, []uint) {
	t.Helper()
	planID := f.seed(t, "order_plan", map[string]any{"branch_id": f.tn.BranchID})
	osuIDs := make([]uint, 0, n)
	for i := 0; i < n; i++ {
		osuIDs = append(osuIDs, f.seed(t, "order_service_used", map[string]any{
			"order_plan_id": planID, "service_id": 1, "branch_id": f.tn.BranchID, "quantity": 1,
		}))
	}
	return planID, osuIDs
}

func TestLogDelete_OrderPlan_ServiceLinesAndParentPayment(t *testing.T) {
	f := newLogFx(t)
	planID, osuIDs := f.seedPlan(t, 2)
	for _, id := range osuIDs {
		f.oldLog(t, "log_order_service_used", "order_service_used_id", id)
	}
	payID := f.seed(t, "payment", map[string]any{"branch_id": f.tn.BranchID, "method": 1, "amount": 0, "parent_id": planID})
	withFKChecks(t)

	require.NoError(t, NewOrderPlanService(testDB).Delete(planID, f.userID, testDB))

	for _, id := range osuIDs {
		f.requireDeleteLog(t, "log_order_service_used", "order_service_used_id", id)
		require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM log_order_service_used WHERE order_service_used_id = ?", id))
	}
	f.requireDeleteLog(t, "log_payment", "payment_id", payID)
}

func TestLogDelete_HandleServices_RemovedLines(t *testing.T) {
	f := newLogFx(t)
	planID, osuIDs := f.seedPlan(t, 2)
	f.oldLog(t, "log_order_service_used", "order_service_used_id", osuIDs[0])
	withFKChecks(t)

	var order orderPlanDao.OrderPlan
	require.NoError(t, testDB.First(&order, planID).Error)
	svc := NewOrderPlanService(testDB)
	// Payload sửa đơn không còn dòng nào ⇒ cả 2 dòng cũ thành "orphan" và bị xoá.
	require.NoError(t, testDB.Transaction(func(tx *gorm.DB) error {
		return svc.handleServices(tx, &order, []map[string]interface{}{}, true, f.userID)
	}))

	for _, id := range osuIDs {
		require.Zero(t, countWhere(t, "SELECT COUNT(*) FROM order_service_used WHERE id = ?", id))
		f.requireDeleteLog(t, "log_order_service_used", "order_service_used_id", id)
	}
}

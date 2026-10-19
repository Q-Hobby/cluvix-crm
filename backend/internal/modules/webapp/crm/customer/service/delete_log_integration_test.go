//go:build integration

package service

// Xoá khách hàng ⇒ CHỈ còn 1 dòng log_customer action_type=delete (user chốt 2026-09-22): log cũ của
// khách và dòng log XOÁ cũ của entity con (fk đã NULL, chỉ nhận ra qua snapshot customer_id) đều bị
// dọn — dòng của khách KHÁC phải còn nguyên. Phần payment (xoá qua deletePayment + bút toán đảo) nằm ở
// crm/service/customer_delete_integration_test.go — package này không import được crm/service.
// Suite tắt foreign_key_checks ⇒ bật lại sau khi seed, không thì FK SET NULL không chạy.

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

func TestDeleteWithCleanup_LogRetention(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	logger.Init(logger.Config{Env: "test", AppCode: "crm_customer_service_test"})
	tn := testhelpers.FreshTenant(t, testDB)
	other := testhelpers.FreshTenant(t, testDB)
	userID := testhelpers.CreateUser(t, testDB, "custdel", tn)
	// Log của khách KHÁC ghi bởi user khác — phân biệt 2 nhóm log bằng changed_by.
	otherUser := testhelpers.CreateUser(t, testDB, "custdel2", other)
	cid := tn.CustomerID
	now := time.Now().Unix()
	t.Cleanup(func() {
		for _, tbl := range []string{"log_customer", "log_appointment", "log_payment"} {
			testDB.Exec("DELETE FROM "+tbl+" WHERE changed_by IN (?, ?)", userID, otherUser)
		}
	})

	insertLog := func(table, fkCol string, fk any, by uint, action int, fields string) {
		t.Helper()
		require.NoError(t, testDB.Exec("INSERT INTO "+table+" ("+fkCol+", action_type, changed_fields, changed_by, changed_at) VALUES (?, ?, ?, ?, ?)",
			fk, action, fields, by, now).Error)
	}
	snapCust := func(c uint) string {
		return fmt.Sprintf(`{"id": {"from": 999, "to": null}, "customer_id": {"from": %d, "to": null}}`, c)
	}

	// Log cũ của chính khách.
	insertLog("log_customer", "customer_id", cid, userID, history.ActionUpdate, `{"phone": {"from": "1", "to": "2"}}`)
	// Dòng log XOÁ cũ của entity con đã xoá trước đó (fk NULL) — của khách này và của khách khác.
	insertLog("log_appointment", "appointment_id", nil, userID, history.ActionDelete, snapCust(cid))
	insertLog("log_payment", "payment_id", nil, userID, history.ActionDelete, snapCust(cid))
	insertLog("log_appointment", "appointment_id", nil, otherUser, history.ActionDelete, snapCust(other.CustomerID))

	require.NoError(t, testDB.Exec("SET SESSION foreign_key_checks = 1").Error)
	t.Cleanup(func() { testDB.Exec("SET SESSION foreign_key_checks = 0") })

	require.NoError(t, NewCustomerService(testDB).DeleteWithCleanup(cid, int(userID)))

	count := func(sql string, args ...any) int64 {
		t.Helper()
		var n int64
		require.NoError(t, testDB.Raw(sql, args...).Scan(&n).Error)
		return n
	}
	require.Zero(t, count("SELECT COUNT(*) FROM customer WHERE id = ?", cid))

	var del struct {
		Cnt, FkNull int
		Fullname    *string
	}
	require.NoError(t, testDB.Raw(`SELECT COUNT(*) cnt, COALESCE(SUM(customer_id IS NULL), 0) fk_null,
		MAX(JSON_UNQUOTE(JSON_EXTRACT(changed_fields, '$.fullname.from'))) fullname
		FROM log_customer WHERE changed_by = ? AND action_type = ?`, userID, history.ActionDelete).Scan(&del).Error)
	// ↑ Story-40 (D15): dòng log xoá mới nhận diện theo changed_by (user riêng của test).
	require.Equal(t, 1, del.Cnt, "phải còn đúng 1 dòng log xoá khách")
	require.Equal(t, 1, del.FkNull, "dòng xoá khách phải có customer_id NULL (FK SET NULL)")
	require.NotNil(t, del.Fullname)
	require.Equal(t, "Test Customer", *del.Fullname, "snapshot tên khách")
	require.Zero(t, count("SELECT COUNT(*) FROM log_customer WHERE changed_by = ? AND action_type = ?", userID, history.ActionUpdate), "log cũ của khách phải được dọn")

	require.Zero(t, count("SELECT COUNT(*) FROM log_payment WHERE changed_by = ?", userID), "dòng log xoá cũ của thanh toán thuộc khách phải được dọn")
	require.Zero(t, count("SELECT COUNT(*) FROM log_appointment WHERE changed_by = ?", userID), "dòng log xoá cũ của lịch hẹn thuộc khách phải được dọn")
	require.EqualValues(t, 1, count("SELECT COUNT(*) FROM log_appointment WHERE changed_by = ?", otherUser), "dòng log của khách KHÁC phải còn")
}

// DeleteWithCleanup KHÔNG xoá payment (xoá thẳng bằng SQL bỏ qua bút toán đảo). Khách còn phiếu ⇒ FK
// payment.customer_id (RESTRICT) chặn ⇒ lỗi, transaction rollback: khách, phiếu, log đều còn nguyên.
func TestDeleteWithCleanup_RefusesWhenPaymentsRemain(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	logger.Init(logger.Config{Env: "test", AppCode: "crm_customer_service_test"})
	tn := testhelpers.FreshTenant(t, testDB)
	userID := testhelpers.CreateUser(t, testDB, "custdel", tn)
	require.NoError(t, testDB.Exec("INSERT INTO payment (code, branch_id, method, amount, customer_id) VALUES (?, ?, 1, 0, ?)",
		fmt.Sprintf("PT%d", time.Now().UnixNano()), tn.BranchID, tn.CustomerID).Error)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM log_customer WHERE changed_by = ?", userID)
	})

	require.NoError(t, testDB.Exec("SET SESSION foreign_key_checks = 1").Error)
	t.Cleanup(func() { testDB.Exec("SET SESSION foreign_key_checks = 0") })

	err := NewCustomerService(testDB).DeleteWithCleanup(tn.CustomerID, int(userID))
	require.Error(t, err)
	require.Contains(t, err.Error(), "1451", "phải là lỗi FK, không phải xoá âm thầm")

	var n int64
	require.NoError(t, testDB.Raw("SELECT COUNT(*) FROM customer WHERE id = ?", tn.CustomerID).Scan(&n).Error)
	require.EqualValues(t, 1, n, "rollback: khách phải còn")
	require.NoError(t, testDB.Raw("SELECT COUNT(*) FROM payment WHERE customer_id = ?", tn.CustomerID).Scan(&n).Error)
	require.EqualValues(t, 1, n, "phiếu phải còn")
	require.NoError(t, testDB.Raw("SELECT COUNT(*) FROM log_customer WHERE changed_by = ?", userID).Scan(&n).Error)
	require.Zero(t, n, "rollback: không được còn dòng log xoá khách")
}

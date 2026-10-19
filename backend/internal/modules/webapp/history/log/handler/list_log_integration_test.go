//go:build integration

package handler

// Query đọc màn lịch sử: dòng xoá lấy id/customer_id/code từ snapshot, và `customer_id` trả về là c.id
// (NULL khi khách không tồn tại) chứ không phải khoá JOIN — trả khoá JOIN thì FE dựng link tới khách
// không tồn tại. `log_*` không có branch_id ⇒ không lọc theo phạm vi; JOIN khách không lọc thêm.

import (
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/helpers"
)

func toInt(v interface{}) int64 { return helpers.ToInt64(v) }

func str(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	}
	return ""
}

func TestListLog_ScopeAndDeleteSnapshot(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB not available (set TEST_MYSQL_DSN)")
	}
	a := testhelpers.FreshTenant(t, testDB)
	b := testhelpers.FreshTenant(t, testDB)
	now := time.Now().Unix()
	var apptIDs, apptLogIDs, custLogIDs []uint
	t.Cleanup(func() {
		if len(apptLogIDs) > 0 {
			testDB.Exec("DELETE FROM log_appointment WHERE id IN ?", apptLogIDs)
		}
		if len(custLogIDs) > 0 {
			testDB.Exec("DELETE FROM log_customer WHERE id IN ?", custLogIDs)
		}
		if len(apptIDs) > 0 {
			testDB.Exec("DELETE FROM appointment WHERE id IN ?", apptIDs)
		}
	})
	seedAppt := func(branch uint, customer uint) uint {
		t.Helper()
		require.NoError(t, testDB.Exec("INSERT INTO appointment (code, branch_id, customer_id) VALUES (?, ?, ?)",
			fmt.Sprintf("LH%d", time.Now().UnixNano()), branch, customer).Error)
		id := testhelpers.LastInsertID(testDB)
		apptIDs = append(apptIDs, id)
		return id
	}
	insertLog := func(table, fkCol string, fk any, action int, fields string) uint {
		t.Helper()
		require.NoError(t, testDB.Exec("INSERT INTO "+table+" ("+fkCol+", action_type, changed_fields, changed_at) VALUES (?, ?, ?, ?)",
			fk, action, fields, now).Error)
		id := testhelpers.LastInsertID(testDB)
		if table == "log_customer" {
			custLogIDs = append(custLogIDs, id)
		} else {
			apptLogIDs = append(apptLogIDs, id)
		}
		return id
	}

	apptA := seedAppt(a.BranchID, a.CustomerID)
	apptCross := seedAppt(a.BranchID, b.CustomerID) // lịch hẹn chi nhánh A trỏ khách của tenant B
	updA := insertLog("log_appointment", "appointment_id", apptA, history.ActionUpdate, `{"note": {"from": "x", "to": "y"}}`)
	delA := insertLog("log_appointment", "appointment_id", nil, history.ActionDelete,
		fmt.Sprintf(`{"id": {"from": 555001, "to": null}, "code": {"from": "LH-DEL-555001", "to": null}, "customer_id": {"from": %d, "to": null}}`, a.CustomerID))
	delGone := insertLog("log_appointment", "appointment_id", nil, history.ActionDelete,
		`{"id": {"from": 555002, "to": null}, "customer_id": {"from": 987654321, "to": null}}`)
	cross := insertLog("log_appointment", "appointment_id", apptCross, history.ActionUpdate, `{"note": {"from": "x", "to": "y"}}`)
	delB := insertLog("log_appointment", "appointment_id", nil, history.ActionDelete,
		fmt.Sprintf(`{"id": {"from": 555003, "to": null}, "customer_id": {"from": %d, "to": null}}`, b.CustomerID))

	h := New(Deps{DB: testDB})
	ctxA := func() *gin.Context { return testhelpers.TenantCtx(a.BranchID, 1) }

	var rows []map[string]interface{}
	require.NoError(t, h.logAppointmentQuery(ctxA(), "").Find(&rows).Error)
	byID := map[uint]map[string]interface{}{}
	for _, r := range rows {
		byID[uint(toInt(r["id"]))] = r
	}
	// `log_appointment` không có branch_id ⇒ mọi dòng log đều hiện. Kiểm theo id đã chèn (không đếm cả
	// bảng — log của test khác cùng package có thể còn).
	for _, id := range []uint{updA, delA, delGone, cross, delB} {
		require.Contains(t, byID, id, "log id=%d phải có trong danh sách", id)
	}

	require.EqualValues(t, apptA, toInt(byID[updA]["appointment_id"]))
	require.EqualValues(t, a.CustomerID, toInt(byID[updA]["customer_id"]))

	require.EqualValues(t, 555001, toInt(byID[delA]["appointment_id"]), "dòng xoá: id lấy từ snapshot")
	require.EqualValues(t, a.CustomerID, toInt(byID[delA]["customer_id"]), "dòng xoá: khách lấy từ snapshot customer_id")
	require.Equal(t, "Test Customer", str(byID[delA]["customer_fullname"]))

	require.Nil(t, byID[delGone]["customer_id"], "khách không tồn tại ⇒ customer_id NULL, không trả khoá JOIN")
	require.EqualValues(t, b.CustomerID, toInt(byID[cross]["customer_id"]), "khách thật của lịch hẹn hiện đúng id")

	var hit []map[string]interface{}
	require.NoError(t, h.logAppointmentQuery(ctxA(), "LH-DEL-555001").Find(&hit).Error)
	require.Len(t, hit, 1, "tìm được dòng xoá theo code trong snapshot")

	// log_customer: dòng xoá khách (customer_id NULL) lấy tên từ snapshot, tìm được theo SĐT snapshot.
	insertLog("log_customer", "customer_id", nil, history.ActionDelete,
		`{"fullname": {"from": "Khách Đã Xoá", "to": null}, "phone": {"from": "0909555001", "to": null}}`)
	custB := insertLog("log_customer", "customer_id", nil, history.ActionDelete,
		`{"fullname": {"from": "Khách Khác", "to": null}}`)
	var cust []map[string]interface{}
	require.NoError(t, h.logCustomerQuery(ctxA(), "0909555001", true).Find(&cust).Error)
	require.Len(t, cust, 1)
	require.Equal(t, "Khách Đã Xoá", str(cust[0]["customer_fullname"]))
	var custAll []map[string]interface{}
	require.NoError(t, h.logCustomerQuery(ctxA(), "", true).Find(&custAll).Error)
	// log_customer không có branch_id ⇒ không còn vế scope nào.
	custIDs := map[uint]bool{}
	for _, r := range custAll {
		custIDs[uint(toInt(r["id"]))] = true
	}
	require.True(t, custIDs[custB], "dòng log_customer không bị lọc")
}

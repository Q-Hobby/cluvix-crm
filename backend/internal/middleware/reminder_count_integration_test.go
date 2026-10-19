//go:build integration

// Story-40 (E6/D15) — chốt hành vi `getReminderCount` (header `Cluvix-Reminder`) trước/sau khi bỏ
// tham số company: đếm nhắc việc CỦA ĐÚNG người gọi (`relatedUser` chứa userID), trong NGÀY HÔM NAY
// theo múi giờ thương hiệu, trạng thái "đã lên kế hoạch". Người khác / ngày khác / trạng thái khác
// không được tính.
package middleware

import (
	"fmt"
	"testing"
	"time"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/define"

	"github.com/stretchr/testify/require"
)

func TestGetReminderCount_ChiDemCuaDungNguoiTrongHomNay(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	me := testhelpers.CreateUser(t, testDB, "s40rem", tn)
	other := testhelpers.CreateUser(t, testDB, "s40rem2", tn)

	loc := brand.Location(testDB)
	now := time.Now().In(loc)
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).Unix()
	tomorrow := noon + 86400

	seed := func(related uint, start int64, status int) {
		t.Helper()
		row := map[string]any{
			"code":        fmt.Sprintf("S40R%d", time.Now().UnixNano()),
			"note":        "s40",
			"relatedUser": fmt.Sprintf("[%d]", related),
			"start_time":  start,
			"status":      status,
			"created_at":  time.Now().Unix(),
			"updated_at":  time.Now().Unix(),
		}
		require.NoError(t, testDB.Table("reminder").Create(&row).Error)
		id := testhelpers.LastInsertID(testDB)
		t.Cleanup(func() { testDB.Exec("DELETE FROM reminder WHERE id = ?", id) })
	}
	seed(me, noon, define.ReminderStatusPlaned)     // tính
	seed(me, noon, define.ReminderStatusPlaned)     // tính
	seed(me, tomorrow, define.ReminderStatusPlaned) // ngày mai — không tính
	seed(me, noon, define.ReminderStatusPlaned+1)   // trạng thái khác — không tính
	seed(other, noon, define.ReminderStatusPlaned)  // người khác — không tính

	// Story-40: chỉ đổi chữ ký (bỏ tham số company) — expected giữ nguyên so với lần chạy trên thân cũ.
	require.Equal(t, 2, getReminderCount(testDB, nil, me))
	require.Equal(t, 1, getReminderCount(testDB, nil, other))
}

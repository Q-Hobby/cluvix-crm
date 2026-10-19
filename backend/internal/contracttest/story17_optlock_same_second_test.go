//go:build integration

// crm-standalone story-17 (D6) — khoá lạc quan theo `updated_at` (Unix GIÂY) phải bắt được xung đột
// giữa 2 request gửi CÙNG giá trị updated_at TRONG CÙNG 1 GIÂY. Lỗ cũ của thiết kế "SET updated_at =
// now": lần ghi 1 đặt updated_at = now = đúng giá trị client 2 đang giữ ⇒ request 2 vẫn khớp ⇒ ghi đè
// mất dữ liệu. Quy tắc GREATEST(now, updated_at + 1) (pkg/optlock) làm updated_at tăng nghiêm ngặt.
//
// Mỗi test ép kịch bản cùng-giây: chờ sang đầu giây mới, đặt updated_at = giây hiện tại bằng SQL thô
// (không qua callback), chạy 2 lần sửa liên tiếp với CÙNG giá trị đó, và chạy lại nếu lỡ vắt qua ranh
// giới giây. Mutation-check: thay GREATEST bằng `now` trong optlock.BumpExpr ⇒ cả 3 test ĐỎ.
package contracttest

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	appointmentDao "erpCluvix/internal/modules/webapp/crm/appointment/dao"
	appointmentService "erpCluvix/internal/modules/webapp/crm/appointment/service"
	reminderHandler "erpCluvix/internal/modules/webapp/crm/reminder/handler"
	crmService "erpCluvix/internal/modules/webapp/crm/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/errs"
)

// sameSecond chạy fn trong cùng 1 giây Unix (chờ sang đầu giây mới trước). fn nhận `sec` = giây đang
// chạy; trả false nếu vắt qua ranh giới giây (khi đó chạy lại, tối đa 3 lần).
func sameSecond(t *testing.T, fn func(sec int64)) {
	t.Helper()
	for attempt := 0; attempt < 3; attempt++ {
		now := time.Now()
		time.Sleep(time.Until(time.Unix(now.Unix()+1, 0)) + 5*time.Millisecond)
		sec := time.Now().Unix()
		fn(sec)
		if time.Now().Unix() == sec {
			return
		}
		t.Logf("vắt qua ranh giới giây (lần %d) — chạy lại", attempt+1)
	}
	t.Fatal("không chạy được kịch bản trong cùng 1 giây sau 3 lần")
}

func setUpdatedAtRaw(t *testing.T, table string, id uint, sec int64) {
	t.Helper()
	if err := testDB.Exec("UPDATE `"+table+"` SET updated_at = ? WHERE id = ?", sec, id).Error; err != nil {
		t.Fatalf("set updated_at %s: %v", table, err)
	}
}

func readUpdatedAt(t *testing.T, table string, id uint) int64 {
	t.Helper()
	var ts int64
	if err := testDB.Raw("SELECT updated_at FROM `"+table+"` WHERE id = ?", id).Scan(&ts).Error; err != nil {
		t.Fatalf("read updated_at %s: %v", table, err)
	}
	return ts
}

func TestS17Optlock_Appointment_SameSecondConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tn, map[string]any{
		"code": fmt.Sprintf("LH%d", time.Now().UnixNano()), "customer_id": tn.CustomerID,
		"status":           define.AppointmentStatusNotCheckin,
		"appointment_date": now + 86400, "created_at": now, "updated_at": now,
	})
	if err := testDB.Table("appointment").Create(&row).Error; err != nil {
		t.Fatalf("seed appointment: %v", err)
	}
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM appointment WHERE id = ?", id) })

	svc := appointmentService.NewAppointmentService(testDB)
	edit := func(ts int64, content string) (*appointmentDao.Appointment, error) {
		a := &appointmentDao.Appointment{Content: content}
		a.ID = id
		a.UpdatedAt = ts
		return a, svc.Update(a, 1, testDB)
	}

	sameSecond(t, func(sec int64) {
		setUpdatedAtRaw(t, "appointment", id, sec)
		first, err := edit(sec, "sửa lần 1")
		if err != nil {
			t.Fatalf("request 1 phải thành công: %v", err)
		}
		if got := readUpdatedAt(t, "appointment", id); got <= sec || first.UpdatedAt != got {
			t.Fatalf("updated_at phải tăng nghiêm ngặt (> %d) và response = DB: db=%d resp=%d", sec, got, first.UpdatedAt)
		}
		if _, err := edit(sec, "sửa lần 2 (stale)"); !errors.Is(err, errs.ErrVersionConflict) {
			t.Fatalf("request 2 cùng giây, cùng updated_at cũ phải bị từ chối ErrVersionConflict, got %v", err)
		}
	})
	var content string
	testDB.Raw("SELECT content FROM appointment WHERE id = ?", id).Scan(&content)
	if content != "sửa lần 1" {
		t.Fatalf("nội dung bị ghi đè bởi request stale: %q", content)
	}
}

func TestS17Optlock_Payment_SameSecondConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	payID, err := testhelpers.CreatePayment(testDB, tn, decimal.NewFromInt(100_000), define.PaymentForDeposit)
	if err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	svc := crmService.NewPaymentService(testDB)

	sameSecond(t, func(sec int64) {
		setUpdatedAtRaw(t, "payment", payID, sec)
		first, err := svc.UpdatePaymentCustomer(payID, map[string]interface{}{"updated_at": sec, "note": "sửa lần 1"}, 1)
		if err != nil {
			t.Fatalf("request 1 phải thành công: %v", err)
		}
		if got := readUpdatedAt(t, "payment", payID); got <= sec || first.UpdatedAt != got {
			t.Fatalf("updated_at phải tăng nghiêm ngặt (> %d) và response = DB: db=%d resp=%d", sec, got, first.UpdatedAt)
		}
		_, err = svc.UpdatePaymentCustomer(payID, map[string]interface{}{"updated_at": sec, "note": "sửa lần 2 (stale)"}, 1)
		if !errors.Is(err, errs.ErrVersionConflict) {
			t.Fatalf("request 2 cùng giây, cùng updated_at cũ phải bị từ chối ErrVersionConflict, got %v", err)
		}
	})
	var note string
	testDB.Raw("SELECT note FROM payment WHERE id = ?", payID).Scan(&note)
	if note != "sửa lần 1" {
		t.Fatalf("ghi chú bị ghi đè bởi request stale: %q", note)
	}
}

func TestS17Optlock_Reminder_SameSecondConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	const uid = uint(1)
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tn, map[string]any{
		"code": fmt.Sprintf("NV%d", time.Now().UnixNano()), "note": "gốc", "status": 1,
		"customer_id": tn.CustomerID, "relatedUser": "[]",
		"start_time": now, "created_by": uid, "created_at": now, "updated_at": now,
	})
	delete(row, "branch_id") // reminder KHÔNG có cột branch_id
	if err := testDB.Table("reminder").Create(&row).Error; err != nil {
		t.Fatalf("seed reminder: %v", err)
	}
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM reminder WHERE id = ?", id) })

	h := reminderHandler.New(reminderHandler.Deps{DB: testDB})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/_s17/reminder/:id", func(c *gin.Context) {
		c.Set("branchIds", []uint{tn.BranchID})
		c.Set("branchActiveId", tn.BranchID)
		c.Set("userID", uid)
		h.UpdateReminder(c)
	})
	put := func(ts int64, note string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		body := fmt.Sprintf(`{"updated_at": %d, "note": %q}`, ts, note)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/_s17/reminder/%d", id), bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	sameSecond(t, func(sec int64) {
		setUpdatedAtRaw(t, "reminder", id, sec)
		if w := put(sec, "sửa lần 1"); w.Code != http.StatusOK {
			t.Fatalf("request 1 phải 200, got %d — %s", w.Code, w.Body.String())
		}
		if got := readUpdatedAt(t, "reminder", id); got <= sec {
			t.Fatalf("updated_at phải tăng nghiêm ngặt: %d → %d", sec, got)
		}
		if w := put(sec, "sửa lần 2 (stale)"); w.Code != http.StatusConflict {
			t.Fatalf("request 2 cùng giây, cùng updated_at cũ phải 409, got %d — %s", w.Code, w.Body.String())
		}
	})
	var note string
	testDB.Raw("SELECT note FROM reminder WHERE id = ?", id).Scan(&note)
	if note != "sửa lần 1" {
		t.Fatalf("ghi chú bị ghi đè bởi request stale: %q", note)
	}
}

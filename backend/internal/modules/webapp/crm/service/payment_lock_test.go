package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"

	"erpCluvix/pkg/errs"
)

// R-10: lock-wait timeout 1205 (kể cả bọc %w) ⇒ payment_busy_retry thay vì lỗi MySQL thô; lỗi khác giữ nguyên.
func TestMapLockWaitTimeout(t *testing.T) {
	lw := fmt.Errorf("save: %w", &mysql.MySQLError{Number: mysqlLockWaitTimeout, Message: "Lock wait timeout exceeded"})
	if got := mapLockWaitTimeout(lw); !errors.Is(got, errs.ErrPaymentBusyRetry) {
		t.Fatalf("1205 ⇒ %v, muốn ErrPaymentBusyRetry", got)
	}
	other := &mysql.MySQLError{Number: 1062, Message: "dup"}
	if got := mapLockWaitTimeout(other); got != other {
		t.Fatalf("lỗi khác bị đổi: %v", got)
	}
	if mapLockWaitTimeout(nil) != nil {
		t.Fatal("nil phải giữ nil")
	}
}

package testhelpers

import (
	"errors"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ErrInjectedWrite — lỗi giả do FailingWritesDB cấy vào mọi lệnh ghi.
var ErrInjectedWrite = errors.New("testhelpers: injected write failure")

// FailingWritesDB trả 1 *gorm.DB MỚI dùng CHUNG connection pool với db nhưng có bộ callback RIÊNG:
// mọi Create/Update/Delete lỗi ErrInjectedWrite TRƯỚC khi chạm MySQL, còn đọc (Query/Raw) chạy bình
// thường. Dùng để chứng minh nhánh "ghi DB lỗi" của handler (dọn file vừa upload, không xoá file cũ…)
// mà không phải DROP bảng/tạo trigger trên DB dùng chung của suite.
//
// Callback gắn vào gorm.DB mới ⇒ KHÔNG ảnh hưởng db gốc hay test khác.
func FailingWritesDB(t *testing.T, db *gorm.DB) *gorm.DB {
	t.Helper()
	return BeforeWriteHookDB(t, db, func(tx *gorm.DB) { _ = tx.AddError(ErrInjectedWrite) })
}

// BeforeWriteHookDB như FailingWritesDB nhưng chạy hook tuỳ ý ngay TRƯỚC mỗi Create/Update/Delete
// (vd mô phỏng request khác vừa ghi chen vào để khoá lạc quan trả 409). Bản sao KHÔNG mang các
// callback đã đăng ký trên db gốc (optlock bump, …).
func BeforeWriteHookDB(t *testing.T, db *gorm.DB, hook func(tx *gorm.DB)) *gorm.DB {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("BeforeWriteHookDB: %v", err)
	}
	fdb, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("BeforeWriteHookDB: %v", err)
	}
	cb := fdb.Callback()
	for _, reg := range []error{
		cb.Create().Before("gorm:create").Register("testhelpers:hook_create", hook),
		cb.Update().Before("gorm:update").Register("testhelpers:hook_update", hook),
		cb.Delete().Before("gorm:delete").Register("testhelpers:hook_delete", hook),
	} {
		if reg != nil {
			t.Fatalf("BeforeWriteHookDB: %v", reg)
		}
	}
	return fdb
}

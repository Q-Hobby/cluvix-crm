package export

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Neo BoundedRows (/check-scale 2026-09-14): báo cáo nhóm theo ngày/chi nhánh bỏ câu COUNT bọc nguyên
// query (chạy query nặng HAI lần mỗi export). Kết quả rỗng vẫn phải trả đúng lỗi cũ và KHÔNG để lại file.

type brEmptyConnector struct{}

func (brEmptyConnector) Connect(context.Context) (driver.Conn, error) { return brEmptyConn{}, nil }
func (brEmptyConnector) Driver() driver.Driver                        { return nil }

type brEmptyConn struct{}

func (brEmptyConn) Prepare(string) (driver.Stmt, error) { return brEmptyStmt{}, nil }
func (brEmptyConn) Close() error                        { return nil }
func (brEmptyConn) Begin() (driver.Tx, error) {
	return nil, errors.New("test: không hỗ trợ transaction")
}

type brEmptyStmt struct{}

func (brEmptyStmt) Close() error                               { return nil }
func (brEmptyStmt) NumInput() int                              { return -1 }
func (brEmptyStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (brEmptyStmt) Query([]driver.Value) (driver.Rows, error)  { return brEmptyRows{}, nil }

type brEmptyRows struct{}

func (brEmptyRows) Columns() []string         { return []string{"x"} }
func (brEmptyRows) Close() error              { return nil }
func (brEmptyRows) Next([]driver.Value) error { return io.EOF }

type brCapture struct{ stmts []string }

func (l *brCapture) LogMode(logger.LogLevel) logger.Interface      { return l }
func (l *brCapture) Info(context.Context, string, ...interface{})  {}
func (l *brCapture) Warn(context.Context, string, ...interface{})  {}
func (l *brCapture) Error(context.Context, string, ...interface{}) {}
func (l *brCapture) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	s, _ := fc()
	l.stmts = append(l.stmts, s)
}

func TestExport_BoundedRows_BoQuaCountVaRongVanBaoLoi(t *testing.T) {
	for _, bounded := range []bool{true, false} {
		capt := &brCapture{}
		db, err := gorm.Open(mysql.New(mysql.Config{
			Conn:                      sql.OpenDB(brEmptyConnector{}),
			SkipInitializeWithVersion: true,
		}), &gorm.Config{DisableAutomaticPing: true, Logger: capt})
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir() + "/"
		_, err = Export(db.Raw("SELECT 1 AS x"), Config{
			FileName: "bounded", Headers: []string{"x"}, ExportDir: dir, BoundedRows: bounded,
		}, func(r map[string]interface{}) []interface{} { return []interface{}{r["x"]} })
		if err == nil || err.Error() != "Không có dữ liệu để export" {
			t.Fatalf("bounded=%v: kết quả rỗng phải trả lỗi cũ, got %v", bounded, err)
		}
		hasCount := false
		for _, s := range capt.stmts {
			if strings.Contains(s, "SELECT COUNT(*) FROM") {
				hasCount = true
			}
		}
		if hasCount == bounded {
			t.Errorf("bounded=%v: có câu COUNT = %v (các câu: %v)", bounded, hasCount, capt.stmts)
		}
		if len(capt.stmts) == 0 {
			t.Fatal("không bắt được câu SQL nào — phép kiểm vô hiệu")
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Errorf("bounded=%v: kết quả rỗng không được để lại file, còn %d", bounded, len(entries))
		}
	}
}

// CountRows thay câu COUNT bọc query (/check-scale 2026-09-17): có hàm ⇒ KHÔNG chạy COUNT, dùng đúng số hàm trả
// (0 ⇒ lỗi "không có dữ liệu", không file); hàm lỗi ⇒ export lỗi.
func TestExport_CountRows_ThayCauCount(t *testing.T) {
	for _, tc := range []struct {
		name    string
		count   func() (int64, error)
		wantErr string
	}{
		{"0 dong", func() (int64, error) { return 0, nil }, "Không có dữ liệu để export"},
		{"ham loi", func() (int64, error) { return 0, errors.New("boom") }, "count error: boom"},
	} {
		capt := &brCapture{}
		db, err := gorm.Open(mysql.New(mysql.Config{
			Conn:                      sql.OpenDB(brEmptyConnector{}),
			SkipInitializeWithVersion: true,
		}), &gorm.Config{DisableAutomaticPing: true, Logger: capt})
		if err != nil {
			t.Fatal(err)
		}
		called := false
		dir := t.TempDir() + "/"
		_, err = Export(db.Raw("SELECT 1 AS x"), Config{
			FileName: "counted", Headers: []string{"x"}, ExportDir: dir,
			CountRows: func() (int64, error) { called = true; return tc.count() },
		}, func(r map[string]interface{}) []interface{} { return []interface{}{r["x"]} })
		if !called {
			t.Fatalf("%s: CountRows không được gọi", tc.name)
		}
		if err == nil || err.Error() != tc.wantErr {
			t.Fatalf("%s: lỗi mong %q, got %v", tc.name, tc.wantErr, err)
		}
		for _, s := range capt.stmts {
			if strings.Contains(s, "SELECT COUNT(*) FROM") {
				t.Fatalf("%s: có CountRows thì không được chạy COUNT bọc query: %v", tc.name, capt.stmts)
			}
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%s: không được để lại file, còn %d", tc.name, len(entries))
		}
	}
}

// ExportMultiSheet cũng tôn trọng CountRows (export Báo cáo chi nhánh, /check-scale 2026-09-17).
func TestExportMultiSheet_CountRows_ThayCauCount(t *testing.T) {
	capt := &brCapture{}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(brEmptyConnector{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true, Logger: capt})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	_, err = ExportMultiSheet(db.Raw("SELECT 1 AS x"), MultiSheetConfig{Config: Config{
		FileName: "multi", Headers: []string{"x"}, ExportDir: t.TempDir() + "/",
		CountRows: func() (int64, error) { called = true; return 0, nil },
	}}, func(map[string]interface{}) string { return "a" },
		func(r map[string]interface{}) []interface{} { return []interface{}{r["x"]} })
	if !called || err == nil || err.Error() != "Không có dữ liệu để export" {
		t.Fatalf("CountRows phải được gọi và 0 dòng ⇒ lỗi cũ; called=%v err=%v", called, err)
	}
	for _, s := range capt.stmts {
		if strings.Contains(s, "SELECT COUNT(*) FROM") {
			t.Fatalf("có CountRows thì không được COUNT bọc query: %v", capt.stmts)
		}
	}
}

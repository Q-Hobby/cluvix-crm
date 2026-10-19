package export

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appLogger "erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Neo /check-scale 2026-09-14: export Doanh thu theo ngày dùng CHUNG cổng FORCE INDEX với list
// (`PaymentByDateIndexHints`). Không hint, 3 nhánh tra theo `code` đọc toàn bảng. Chạy CHÍNH
// ExportPaymentByDate trên driver giả (0 dòng), bắt SQL qua logger.

type rbdEmptyConnector struct{}

func (rbdEmptyConnector) Connect(context.Context) (driver.Conn, error) { return rbdEmptyConn{}, nil }
func (rbdEmptyConnector) Driver() driver.Driver                        { return nil }

type rbdEmptyConn struct{}

func (rbdEmptyConn) Prepare(string) (driver.Stmt, error) { return rbdEmptyStmt{}, nil }
func (rbdEmptyConn) Close() error                        { return nil }
func (rbdEmptyConn) Begin() (driver.Tx, error) {
	return nil, errors.New("test: không hỗ trợ transaction")
}

type rbdEmptyStmt struct{}

func (rbdEmptyStmt) Close() error                               { return nil }
func (rbdEmptyStmt) NumInput() int                              { return -1 }
func (rbdEmptyStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(0), nil }
func (rbdEmptyStmt) Query([]driver.Value) (driver.Rows, error)  { return rbdEmptyRows{}, nil }

type rbdEmptyRows struct{}

func (rbdEmptyRows) Columns() []string         { return nil }
func (rbdEmptyRows) Close() error              { return nil }
func (rbdEmptyRows) Next([]driver.Value) error { return io.EOF }

type rbdCapture struct{ stmts []string }

func (l *rbdCapture) LogMode(logger.LogLevel) logger.Interface      { return l }
func (l *rbdCapture) Info(context.Context, string, ...interface{})  {}
func (l *rbdCapture) Warn(context.Context, string, ...interface{})  {}
func (l *rbdCapture) Error(context.Context, string, ...interface{}) {}
func (l *rbdCapture) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	s, _ := fc()
	l.stmts = append(l.stmts, strings.Join(strings.Fields(s), " "))
}

func TestExportPaymentByDate_HintCoCong(t *testing.T) {
	t.Chdir(t.TempDir())
	appLogger.Init(appLogger.Config{Env: "test", AppCode: "export_by_date_test"})
	const year = "_ngay_thang[gte]=1757869200&_ngay_thang[lte]=1789405199"
	for _, tc := range []struct {
		name, raw     string
		want, notWant []string
	}{
		{"nhiều chi nhánh, có ngày ⇒ hint cả 3", year, []string{
			"FROM appointment FORCE INDEX (idx_appointment_date_scope) INNER JOIN",
			"FROM order_plan FORCE INDEX (idx_order_plan_created_time_branch) LEFT JOIN",
			"FROM payment FORCE INDEX (idx_payment_time_cover) WHERE",
		}, nil},
		{"không lọc ngày ⇒ payment KHÔNG hint", "", []string{
			"FROM order_plan FORCE INDEX (idx_order_plan_created_time_branch)",
		}, []string{"FROM payment FORCE INDEX"}},
		{"đúng 1 chi nhánh ⇒ không hint", year + "&filter[branch_id][in][0]=3", []string{
			"FROM appointment INNER JOIN", "FROM payment WHERE",
		}, []string{"FORCE INDEX"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capt := &rbdCapture{}
			db, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sql.OpenDB(rbdEmptyConnector{}),
				SkipInitializeWithVersion: true,
			}), &gorm.Config{DisableAutomaticPing: true, Logger: capt})
			if err != nil {
				t.Fatal(err)
			}
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/?"+tc.raw, nil)
			ExportPaymentByDate(c, db)
			var s string
			for _, st := range capt.stmts {
				if strings.Contains(st, "UNION ALL") {
					s = st
				}
			}
			if s == "" {
				t.Fatalf("không bắt được baseSQL — phép kiểm vô hiệu (bắt được: %v)", capt.stmts)
			}
			for _, w := range tc.want {
				if !strings.Contains(s, w) {
					t.Errorf("thiếu %q trong: %s", w, s)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(s, w) {
					t.Errorf("không được có %q trong: %s", w, s)
				}
			}
		})
	}
}

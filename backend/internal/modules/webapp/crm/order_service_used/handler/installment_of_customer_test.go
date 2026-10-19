package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appLogger "erpCluvix/pkg/logger"
	"erpCluvix/pkg/tz"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Neo GET /crm/order-service-used/installment-of-customer (picker kỳ trả góp khi thu tiền, 2026-09-14):
//   - quá hạn = kỳ còn mở `status IN (1,3)` có `due_date < hôm nay` theo timezone thương hiệu — trước
//     đó nhãn "⚠ Quá hạn" gắn cho status 3 (thực ra là "trả một phần");
//   - `customer_id` phải parse ra số TRƯỚC khi vào query (MySQL ép '5abc' → 5 ⇒ đọc trúng khách 5).
// GORM DryRun — không đụng database nào.

type nopConnIoc struct{}

func (nopConnIoc) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dry run: test này không được phép mở kết nối")
}
func (nopConnIoc) Driver() driver.Driver { return nil }

type sqlCaptureIoc struct{ stmts []string }

func (l *sqlCaptureIoc) LogMode(logger.LogLevel) logger.Interface      { return l }
func (l *sqlCaptureIoc) Info(context.Context, string, ...interface{})  {}
func (l *sqlCaptureIoc) Warn(context.Context, string, ...interface{})  {}
func (l *sqlCaptureIoc) Error(context.Context, string, ...interface{}) {}
func (l *sqlCaptureIoc) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	if s, _ := fc(); s != "" {
		l.stmts = append(l.stmts, s)
	}
}

func runInstallmentOfCustomer(t *testing.T, customerID string) (*httptest.ResponseRecorder, *sqlCaptureIoc, *gin.Context, *gorm.DB) {
	t.Helper()
	// DryRun ⇒ Scan trả lỗi ⇒ handler ghi ServerError ⇒ cần `logger.L` khác nil.
	appLogger.Init(appLogger.Config{Env: "test", AppCode: "installment_of_customer_test"})
	capt := &sqlCaptureIoc{}
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(nopConnIoc{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: capt})
	if err != nil {
		t.Fatalf("mở DryRun DB: %v", err)
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/crm/order-service-used/installment-of-customer?customer_id="+customerID, nil)
	// Story-35/62: tz.LocFromContext đọc múi giờ qua pkg/brand (1 câu SELECT bảng business_profile;
	// DryRun ⇒ không có dòng ⇒ DefaultTZ).
	New(Deps{DB: db}).InstallmentOfCustomer(c)
	return w, capt, c, db
}

func TestInstallmentOfCustomer_QuaHanTheoNgay(t *testing.T) {
	_, capt, c, db := runInstallmentOfCustomer(t, "5")
	if len(capt.stmts) == 0 {
		t.Fatal("không bắt được câu SQL nào — phép kiểm vô hiệu")
	}
	// Story-35: câu đầu tiên có thể là câu đọc hồ sơ thương hiệu của pkg/brand (tz.LocFromContext)
	// ⇒ lấy câu query chính theo nội dung, không theo vị trí.
	s := ""
	for _, st := range capt.stmts {
		if strings.Contains(st, "is_overdue") {
			s = strings.Join(strings.Fields(st), " ")
			break
		}
	}
	if s == "" {
		t.Fatalf("không thấy câu query kỳ trả góp (chứa is_overdue) trong %d câu: %v", len(capt.stmts), capt.stmts)
	}
	today := tz.LocalDate(tz.LocFromContext(c, db), 0)
	for _, want := range []string{
		"(oi.due_date < '" + today + "') as is_overdue",
		"otp.customer_id = 5",
		"oi.status IN (1,3)",
		"is_overdue DESC",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("SQL thiếu %q.\nSQL: %s", want, s)
		}
	}
	if strings.Contains(s, "1, 3, 4") || strings.Contains(s, "FIELD(oi.status") {
		t.Fatalf("SQL còn định nghĩa cũ (status 4 / FIELD theo status).\nSQL: %s", s)
	}
}

func TestInstallmentOfCustomer_CustomerIDKhongPhaiSoBiChan(t *testing.T) {
	for _, bad := range []string{"5abc", "", "-1", "0"} {
		w, capt, _, _ := runInstallmentOfCustomer(t, bad)
		if len(capt.stmts) != 0 {
			t.Fatalf("customer_id=%q phải bị chặn TRƯỚC query, nhưng đã chạy: %v", bad, capt.stmts)
		}
		if w.Code == http.StatusOK || w.Code >= http.StatusInternalServerError {
			t.Fatalf("customer_id=%q phải trả lỗi validate, nhận HTTP %d", bad, w.Code)
		}
	}
}

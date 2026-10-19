package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// nopConnector — connector không bao giờ mở kết nối thật. `sql.OpenDB` là LƯỜI (không dial lúc gọi),
// còn GORM ở DryRun thì dựng SQL rồi dừng, nên Connect không bao giờ được gọi.
// Nhờ vậy test chạy được ở mọi máy/CI mà KHÔNG đụng tới bất kỳ database nào.
type nopConnector struct{}

func (nopConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dry run: test này không được phép mở kết nối")
}
func (nopConnector) Driver() driver.Driver { return nil }

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(nopConnector{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("mở DryRun DB: %v", err)
	}
	return db
}

// Guard IDOR của ImportUsers dựng tập id-trong-scope bằng Pluck. GORM chỉ tự đặt SELECT cho Pluck khi
// `len(Statement.Selects) != 1` — mà BuildUserQuery gọi `.Select("<chuỗi 36 cột>")` nên Selects có
// đúng 1 phần tử ⇒ Pluck GIỮ NGUYÊN 36 cột, scan vào []uint là lỗi "destination not a pointer" và
// trả về toàn số 0.
//
// Hậu quả thật (đã xảy ra): tập scope rỗng ⇒ MỌI dòng cập nhật trong file import báo
// "Lỗi dòng 3: Không tìm thấy nhân viên", trong khi nguyên nhân là query hỏng.
// Test khoá: câu SQL sinh ra phải chỉ chọn ĐÚNG một cột id.
func TestScopeIDQuerySelectsOnlyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)

	h := New(Deps{DB: dryRunDB(t)})

	var ids []uint
	stmt := h.BuildUserQuery(c).
		Where("u.id IN ?", []uint{7, 8}).
		Select("u.id").
		Pluck("u.id", &ids).Statement
	sql := stmt.SQL.String()

	selectPart := sql
	if i := strings.Index(strings.ToUpper(sql), " FROM "); i > 0 {
		selectPart = sql[:i]
	}
	if strings.Contains(selectPart, ",") {
		t.Fatalf("SELECT phải chỉ có 1 cột, đang có nhiều cột:\n%s", selectPart)
	}
	if !strings.Contains(selectPart, "u.id") {
		t.Fatalf("SELECT phải là u.id, đang là:\n%s", selectPart)
	}
	// GROUP BY vẫn phải còn: nó là thứ chặn user nhiều vai trò nhân đôi dòng.
	if !strings.Contains(strings.ToUpper(sql), "GROUP BY") {
		t.Errorf("thiếu GROUP BY trong:\n%s", sql)
	}
}

// Chứng minh chiều ngược lại: KHÔNG có `.Select("u.id")` thì Pluck để nguyên SELECT nhiều cột.
// Nếu ngày nào đó GORM đổi hành vi này, test sẽ đỏ và ta biết có thể bỏ workaround đi.
func TestPluckDoesNotOverrideMultiColumnSelect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)

	h := New(Deps{DB: dryRunDB(t)})

	var ids []uint
	sql := h.BuildUserQuery(c).Where("u.id IN ?", []uint{7}).Pluck("u.id", &ids).Statement.SQL.String()
	upper := strings.ToUpper(sql)
	selectPart := sql
	if i := strings.Index(upper, " FROM "); i > 0 {
		selectPart = sql[:i]
	}
	if !strings.Contains(selectPart, ",") {
		t.Fatalf("kỳ vọng Pluck GIỮ NGUYÊN select nhiều cột (hành vi GORM hiện tại), nhưng chỉ thấy:\n%s", selectPart)
	}
}

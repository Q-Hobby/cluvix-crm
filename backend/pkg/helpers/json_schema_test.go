package helpers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"erpCluvix/pkg/logger"

	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ── Fake driver: mọi kết nối đều fail ─────────────────────────────────────────
// Mục đích: mô phỏng "InitDecimalColumnMap thất bại" (DSN sai / không đọc được
// INFORMATION_SCHEMA) mà KHÔNG chạm tới bất kỳ MySQL thật nào.

var errNoConn = errors.New("simulated: cannot reach INFORMATION_SCHEMA")

type failingConnector struct{}

func (failingConnector) Connect(context.Context) (driver.Conn, error) { return nil, errNoConn }
func (failingConnector) Driver() driver.Driver                        { return failingDriver{} }

type failingDriver struct{}

func (failingDriver) Open(string) (driver.Conn, error) { return nil, errNoConn }

// failingGorm trả về *gorm.DB mà mọi query đều lỗi (không mở kết nối thật).
func failingGorm(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB := sql.OpenDB(failingConnector{})
	g, err := gorm.Open(
		mysqldriver.New(mysqldriver.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true, // không chạy SELECT VERSION() lúc Open
		}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("open fake gorm: %v", err)
	}
	return g
}

// ── Fake driver: query THÀNH CÔNG nhưng trả 0 dòng ───────────────────────────
// Mô phỏng ca nguy hiểm nhất: DSN không kèm tên DB (DATABASE() NULL) hoặc user MySQL
// thiếu quyền đọc INFORMATION_SCHEMA — MySQL KHÔNG trả lỗi, chỉ trả 0 dòng.

type emptyConnector struct{}

func (emptyConnector) Connect(context.Context) (driver.Conn, error) { return emptyConn{}, nil }
func (emptyConnector) Driver() driver.Driver                        { return emptyDriver{} }

type emptyDriver struct{}

func (emptyDriver) Open(string) (driver.Conn, error) { return emptyConn{}, nil }

type emptyConn struct{}

func (emptyConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (emptyConn) Close() error              { return nil }
func (emptyConn) Begin() (driver.Tx, error) { return nil, errors.New("tx not supported") }
func (emptyConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &emptyRows{}, nil
}

type emptyRows struct{}

func (*emptyRows) Columns() []string         { return []string{"TABLE_NAME", "COLUMN_NAME"} }
func (*emptyRows) Close() error              { return nil }
func (*emptyRows) Next([]driver.Value) error { return io.EOF }

func emptyGorm(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB := sql.OpenDB(emptyConnector{})
	g, err := gorm.Open(
		mysqldriver.New(mysqldriver.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("open empty gorm: %v", err)
	}
	return g
}

// AC1 + AC2: "query OK nhưng 0 dòng" phải bị coi là THẤT BẠI.
// Không có case này thì map là "rỗng khác nil" ⇒ ParseDecimalFieldsForTables qua guard nil rồi
// return ở len(cols)==0 ⇒ toàn hệ thống trả string, /health báo ready=true, không log gì.
func TestInitDecimalColumnMap_ZeroRowsIsFailure(t *testing.T) {
	InvalidateSchemaCache()
	mem := logger.NewMemoryLogger()
	schemaLogger = mem
	t.Cleanup(func() {
		schemaLogger = nil
		InvalidateSchemaCache()
	})

	InitDecimalColumnMap(emptyGorm(t))

	st := DecimalColumnMapStatusSnapshot()
	if st.Ready {
		t.Fatalf("0 dòng PHẢI là Ready=false (nếu không: suy biến im lặng), got %+v", st)
	}
	if !strings.Contains(st.Error, "0 cột decimal") {
		t.Fatalf("status Error phải nêu nguyên nhân 0 cột, got %q", st.Error)
	}

	var errEntries []logger.Entry
	for _, e := range mem.Entries() {
		if e.Level == "error" {
			errEntries = append(errEntries, e)
		}
	}
	if len(errEntries) != 1 {
		t.Fatalf("expected exactly 1 error log, got %d (%+v)", len(errEntries), mem.Entries())
	}
	if !strings.Contains(errEntries[0].Msg, "InitDecimalColumnMap") {
		t.Fatalf("error log must name the function, got %q", errEntries[0].Msg)
	}
	t.Logf("observed ERROR log: %s", errEntries[0].Msg)

	// AC5: parser vẫn return sớm, không panic, không đổi giá trị.
	rows := []map[string]interface{}{{"total": []uint8("123.45")}}
	ParseDecimalFieldsForTables(rows, "order")
	if _, stillRaw := rows[0]["total"].([]uint8); !stillRaw {
		t.Fatalf("expected untouched raw value, got %T", rows[0]["total"])
	}
}

// AC1 + AC3: init thất bại → log ERROR nêu rõ nguyên nhân.
// AC2 + AC3: trạng thái đọc được từ ngoài qua DecimalColumnMapStatusSnapshot (health-check).
// AC4: không panic, không os.Exit — hàm return bình thường.
// AC5: ParseDecimalFieldsForTables vẫn return sớm, không panic, không đổi row.
func TestInitDecimalColumnMap_FailureIsObservable(t *testing.T) {
	InvalidateSchemaCache()
	mem := logger.NewMemoryLogger()
	schemaLogger = mem
	t.Cleanup(func() {
		schemaLogger = nil
		InvalidateSchemaCache()
	})

	// AC4: gọi bình thường, không panic
	InitDecimalColumnMap(failingGorm(t))

	// AC2: trạng thái phơi ra ngoài
	st := DecimalColumnMapStatusSnapshot()
	if st.Ready {
		t.Fatalf("expected Ready=false after failed init, got %+v", st)
	}
	if !strings.Contains(st.Error, "simulated") {
		t.Fatalf("status Error should carry root cause, got %q", st.Error)
	}
	if st.Tables != 0 || st.Columns != 0 {
		t.Fatalf("expected zero counts, got %+v", st)
	}

	// AC1: có đúng 1 log mức error, nêu nguyên nhân
	var errEntries []logger.Entry
	for _, e := range mem.Entries() {
		if e.Level == "error" {
			errEntries = append(errEntries, e)
		}
	}
	if len(errEntries) != 1 {
		t.Fatalf("expected exactly 1 error log, got %d (%+v)", len(errEntries), mem.Entries())
	}
	msg := errEntries[0].Msg
	if !strings.Contains(msg, "InitDecimalColumnMap") || !strings.Contains(msg, "simulated") {
		t.Fatalf("error log must name the function and the cause, got %q", msg)
	}
	t.Logf("observed ERROR log: %s", msg)
	t.Logf("observed health status: %+v", st)

	// AC2 (phần route thật): assert /health phơi field `decimal_column_map` nằm ở
	// `internal/router/health_test.go` — nó gọi ĐÚNG `healthHandler` qua `gin.Engine` +
	// `httptest`. Trước đây chỗ này tự `json.Marshal(gin.H{...})` nên xoá field trong router
	// vẫn xanh (verification debt, QA pass 1 [P2]); pkg/helpers không import ngược
	// internal/router được (import cycle) nên lưới phải đặt bên đó.

	// AC5: parser giữ nguyên hành vi return sớm — không panic, không đổi giá trị
	rows := []map[string]interface{}{{"total": []uint8("123.45")}}
	ParseDecimalFieldsForTables(rows, "order")
	if _, stillRaw := rows[0]["total"].([]uint8); !stillRaw {
		t.Fatalf("expected untouched raw value when map is unavailable, got %T", rows[0]["total"])
	}
}

// Status mặc định (chưa init) cũng phải đọc được là "chưa sẵn sàng".
func TestDecimalColumnMapStatus_NotInitialized(t *testing.T) {
	InvalidateSchemaCache()
	t.Cleanup(func() { InvalidateSchemaCache() })

	st := DecimalColumnMapStatusSnapshot()
	if st.Ready || st.Error == "" {
		t.Fatalf("expected not-ready status with reason, got %+v", st)
	}
}

// story-26 AC2/AC4 — InitJSONColumnMap trước đây nuốt lỗi INFORMATION_SCHEMA im lặng (`return`
// không log/không phơi trạng thái), y hệt bản decimal trước khi story-02 vá. Mirror kỹ thuật test
// của story-02 (mock lỗi qua driver.Connector giả, không chạm DB thật).
func TestInitJSONColumnMap_ZeroRowsIsFailure(t *testing.T) {
	InvalidateSchemaCache()
	mem := logger.NewMemoryLogger()
	schemaLogger = mem
	t.Cleanup(func() {
		schemaLogger = nil
		InvalidateSchemaCache()
	})

	InitJSONColumnMap(emptyGorm(t))

	st := JSONColumnMapStatusSnapshot()
	if st.Ready {
		t.Fatalf("0 dòng PHẢI là Ready=false (nếu không: suy biến im lặng), got %+v", st)
	}
	if !strings.Contains(st.Error, "0 cột json") {
		t.Fatalf("status Error phải nêu nguyên nhân 0 cột, got %q", st.Error)
	}

	var errEntries []logger.Entry
	for _, e := range mem.Entries() {
		if e.Level == "error" {
			errEntries = append(errEntries, e)
		}
	}
	if len(errEntries) != 1 {
		t.Fatalf("expected exactly 1 error log, got %d (%+v)", len(errEntries), mem.Entries())
	}
	if !strings.Contains(errEntries[0].Msg, "InitJSONColumnMap") {
		t.Fatalf("error log must name the function, got %q", errEntries[0].Msg)
	}

	// AC parser vẫn return sớm, không panic, không đổi giá trị.
	rows := []map[string]interface{}{{"listRevenues": "[1,2]"}}
	ParseJSONFieldsForTables(rows, "order_service_used")
	if _, stillRaw := rows[0]["listRevenues"].(string); !stillRaw {
		t.Fatalf("expected untouched raw value, got %T", rows[0]["listRevenues"])
	}
}

func TestInitJSONColumnMap_FailureIsObservable(t *testing.T) {
	InvalidateSchemaCache()
	mem := logger.NewMemoryLogger()
	schemaLogger = mem
	t.Cleanup(func() {
		schemaLogger = nil
		InvalidateSchemaCache()
	})

	InitJSONColumnMap(failingGorm(t)) // không panic

	st := JSONColumnMapStatusSnapshot()
	if st.Ready {
		t.Fatalf("expected Ready=false after failed init, got %+v", st)
	}
	if !strings.Contains(st.Error, "simulated") {
		t.Fatalf("status Error should carry root cause, got %q", st.Error)
	}
	if st.Tables != 0 || st.Columns != 0 {
		t.Fatalf("expected zero counts, got %+v", st)
	}

	var errEntries []logger.Entry
	for _, e := range mem.Entries() {
		if e.Level == "error" {
			errEntries = append(errEntries, e)
		}
	}
	if len(errEntries) != 1 {
		t.Fatalf("expected exactly 1 error log, got %d (%+v)", len(errEntries), mem.Entries())
	}
	msg := errEntries[0].Msg
	if !strings.Contains(msg, "InitJSONColumnMap") || !strings.Contains(msg, "simulated") {
		t.Fatalf("error log must name the function and the cause, got %q", msg)
	}
}

// Status mặc định (chưa init) cũng phải đọc được là "chưa sẵn sàng".
func TestJSONColumnMapStatus_NotInitialized(t *testing.T) {
	InvalidateSchemaCache()
	t.Cleanup(func() { InvalidateSchemaCache() })

	st := JSONColumnMapStatusSnapshot()
	if st.Ready || st.Error == "" {
		t.Fatalf("expected not-ready status with reason, got %+v", st)
	}
}

// Package testhelpers provides shared test infrastructure for the Cluvix ERP backend.
//
// # Build tags
//
// Tests that depend on a real MySQL database should be guarded with:
//
//	//go:build integration
//
// Without `-tags=integration` they will not compile, so `go test ./...`
// runs only fast pure-function tests. To run the full suite:
//
//	TEST_MYSQL_DSN='root:testpass@(localhost:3306)/cluvix_test?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true' \
//	  go test -tags=integration ./...
//
// # Suite usage
//
// A test package that needs DB access creates a Suite in TestMain:
//
//	//go:build integration
//
//	var testDB *gorm.DB
//
//	func TestMain(m *testing.M) {
//	    s, err := testhelpers.New(testhelpers.MigrationsDir())
//	    if err != nil {
//	        log.Printf("testhelpers: %v — skipping integration tests", err)
//	        os.Exit(m.Run())
//	    }
//	    defer s.Close()
//	    testDB = s.DB
//	    os.Exit(m.Run())
//	}
//
// # Schema mode
//
// Story-62 (schema mới `backend/migrations`, 1 bảng/1 file, không còn baseline): SchemaBaseline và
// SchemaAll cùng áp MỌI migration trừ seed (`*_seed_*`) — DB trắng không dữ liệu. SchemaFull áp cả
// seed (hồ sơ doanh nghiệp, chi nhánh CN01, admin, config_code, user_option, địa chỉ).
package testhelpers

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"erpCluvix/pkg/optlock"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SchemaMode controls how much of the migration history is applied to the test DB.
type SchemaMode int

const (
	// SchemaBaseline — mặc định; từ story-62 giống SchemaAll (schema mới không có file baseline).
	SchemaBaseline SchemaMode = iota
	// SchemaAll applies every *.up.sql file in order, EXCEPT seed files (`*_seed_*`).
	SchemaAll
	// SchemaFull applies all migrations including seed files.
	SchemaFull
)

// Suite holds a MySQL test connection with the requested schema applied.
type Suite struct {
	DB    *gorm.DB
	sqlDB *sql.DB
}

// New connects to the MySQL database named by the TEST_MYSQL_DSN env var,
// resets it (drops every existing table), applies the requested schema, and returns a Suite.
//
// The DSN MUST point to a disposable test database — all its tables are dropped —
// and MUST include multiStatements=true, e.g.:
//
//	root:testpass@(localhost:3306)/cluvix_test?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true
//
// Returns error if TEST_MYSQL_DSN is unset or unreachable — callers should skip
// integration tests gracefully.
func New(migrationsDir string, mode ...SchemaMode) (*Suite, error) {
	m := SchemaBaseline
	if len(mode) > 0 {
		m = mode[0]
	}

	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("%w (need a disposable MySQL database)", ErrNoDSN)
	}
	// applySchema chạy CẢ FILE .up.sql như MỘT multi-statement Exec, nên DSN buộc phải có
	// multiStatements=true. Thiếu tham số này thì baseline lỗi `Error 1064` ngay dòng `/*!40101 ... */`,
	// New() trả error, và TestMain nào chỉ `log` rồi `m.Run()` sẽ **im lặng SKIP toàn bộ** trong khi
	// output tổng vẫn là `ok` — xanh giả. Tự thêm thay vì bắt mọi caller nhớ (đo được 2026-08-28).
	dsn = ensureMultiStatements(dsn)

	// GUARD (bài học 2026-07-18): New() gọi dropAllTables = DROP mọi bảng. Nếu TEST_MYSQL_DSN vô tình
	// trỏ vào DB thật thì wipe sạch data của 1 khách (mô hình DB-per-customer). Chặn trước khi connect.
	// Xem skill/skills/writing-mysql-migrations/SKILL.md mục ⛔.
	if err := assertDisposableDB(dbNameFromDSN(dsn)); err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, fmt.Errorf("connect to mysql: %w", err)
	}
	// Cùng callback khoá lạc quan updated_at như production (db.Connect).
	if err := optlock.Register(db); err != nil {
		return nil, fmt.Errorf("register optlock callback: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	// Single connection so the SESSION-level settings below apply to every query in the suite.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	// Session-level (not GLOBAL — the server may be a shared local MySQL, not a throwaway
	// container) so fixtures can create records without the full object graph.
	if err := db.Exec("SET SESSION foreign_key_checks = 0").Error; err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("disable foreign_key_checks: %w", err)
	}

	if err := dropAllTables(db); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("reset test database: %w", err)
	}

	if err := applySchema(db, migrationsDir, m); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Suite{DB: db, sqlDB: sqlDB}, nil
}

// Close releases the DB connection. Always defer this after a successful New().
func (s *Suite) Close() {
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
	}
}

// dbNameFromDSN trích tên database từ go-sql-driver DSN
// (dạng `user:pass@tcp(host:port)/dbname?params`): đoạn sau dấu `/` cuối, cắt tại `?`.
func dbNameFromDSN(dsn string) string {
	slash := strings.LastIndex(dsn, "/")
	if slash < 0 {
		return ""
	}
	rest := dsn[slash+1:]
	if q := strings.IndexByte(rest, '?'); q >= 0 {
		rest = rest[:q]
	}
	return rest
}

// assertDisposableDB từ chối mọi DB không phải "dùng-một-lần" TRƯỚC khi kết nối.
//
// Vì sao FAIL-CLOSED bằng quy ước tên, không chỉ so với DB_NAME (sửa 2026-09-02):
// bản cũ chỉ có `if realDB := os.Getenv("DB_NAME"); realDB != "" && ...`. Nhưng KHÔNG một
// *_test.go nào trong repo nạp .env (đo: 0 file dùng godotenv, đối chứng 461 file có "testing"),
// nên trong process test biến DB_NAME luôn rỗng → điều kiện short-circuit → guard đó CHƯA TỪNG
// bắn một lần nào, dù comment ghi "Fatal ngay trước khi connect". Lưới phụ thuộc vào một biến
// môi trường mà không ai set là lưới trang trí.
//
// Hai tầng, tầng 1 không cần biết DB thật tên gì nên vẫn đúng cả khi đọc .env thất bại:
//  1. Tên DB PHẢI kết thúc bằng _test hoặc _tmp (quy ước DB dùng-một-lần trong CLAUDE.md).
//  2. VÀ không được trùng DB thật — bắt trường hợp DB dev lỡ đặt tên hợp quy ước.
func assertDisposableDB(name string) error {
	if name == "" {
		return fmt.Errorf(
			"testhelpers: TEST_MYSQL_DSN has no database name — refusing to run, this harness DROPS ALL TABLES")
	}
	if !strings.HasSuffix(name, "_test") && !strings.HasSuffix(name, "_tmp") {
		return fmt.Errorf(
			"testhelpers refuses to run against %q: this harness DROPS ALL TABLES, so the database must be "+
				"disposable and named with a _test or _tmp suffix (create it first, drop it after)", name)
	}
	if real := realDBName(); real != "" && name == real {
		return fmt.Errorf(
			"testhelpers refuses to run against the real database %q: TEST_MYSQL_DSN points at DB_NAME "+
				"and this harness DROPS ALL TABLES. Use a disposable *_test/*_tmp database", real)
	}
	return nil
}

// realDBName trả tên DB thật của môi trường dev. Ưu tiên biến môi trường; rỗng thì ĐỌC THẲNG
// backend/.env, vì process test không nạp .env (xem assertDisposableDB). Không đọc được → "" và
// tầng 1 của assertDisposableDB vẫn giữ lưới.
func realDBName() string {
	if v := os.Getenv("DB_NAME"); v != "" {
		return v
	}
	f, err := os.Open(filepath.Join(backendRoot(), ".env"))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "DB_NAME") {
			continue
		}
		_, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		// Bỏ comment cuối dòng + nháy bao quanh: `DB_NAME="x" # ghi chú`.
		if i := strings.IndexByte(val, '#'); i >= 0 {
			val = val[:i]
		}
		return strings.Trim(strings.TrimSpace(val), `"'`)
	}
	return ""
}

// dropAllTables drops every table in the current database so each run starts from
// a clean slate (the database persists between runs, unlike the old container setup).
func dropAllTables(db *gorm.DB) error {
	var tables []string
	if err := db.Raw(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'",
	).Scan(&tables).Error; err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	for _, t := range tables {
		if err := db.Exec("DROP TABLE IF EXISTS `" + t + "`").Error; err != nil {
			return fmt.Errorf("drop table %s: %w", t, err)
		}
	}
	return nil
}

// MigrationsDir returns the absolute path to the migrations directory used by
// integration tests.
//
// Luôn là `backend/migrations` (schema mới). Story-62 bỏ env TEST_MIGRATIONS_DIR (story-04 thêm để
// chạy tạm trên `migrations_backup` trong lúc schema mới chưa có) — schema cũ không còn khớp code.
func MigrationsDir() string {
	return filepath.Join(backendRoot(), "migrations")
}

// backendRoot trả thư mục backend/ suy từ vị trí SOURCE của file này, nên không phụ thuộc
// working dir của package test đang chạy.
func backendRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// file = .../backend/internal/testhelpers/suite.go
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// applySchema dispatches to the correct migration runner based on mode.
func applySchema(db *gorm.DB, migrationsDir string, mode SchemaMode) error {
	switch mode {
	case SchemaBaseline, SchemaAll:
		return applyMigrations(db, migrationsDir, false)
	case SchemaFull:
		return applyMigrations(db, migrationsDir, true)
	default:
		return fmt.Errorf("unknown SchemaMode: %d", mode)
	}
}

// applyMigrations executes every *.up.sql in migrationsDir in lexical order,
// optionally skipping seed files (`*_seed_*`, vd `20260928094711_seed_business_profile.up.sql`).
func applyMigrations(db *gorm.DB, migrationsDir string, includeSeed bool) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		if !includeSeed && strings.Contains(name, "_seed_") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)

	for _, f := range files {
		if err := execFile(db, filepath.Join(migrationsDir, f)); err != nil {
			return fmt.Errorf("apply %s: %w", f, err)
		}
	}
	return nil
}

// execFile reads a SQL file and runs it as a single multi-statement Exec.
func execFile(db *gorm.DB, path string) error {
	sql, err := os.ReadFile(path) //nolint:gosec // test helper; path comes from internal fixtures, not user input
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return db.Exec(string(sql)).Error
}

// ErrNoDSN — TEST_MYSQL_DSN chưa được set. Tách riêng để TestMain phân biệt hai tình huống KHÁC HẲN
// nhau: "máy này chưa cấu hình DB test" (skip là đúng) với "đã cấu hình mà dựng schema hỏng" (phải
// FAIL). Gộp chúng lại chính là cách một suite integration hỏng đi qua CI với dấu `ok`.
var ErrNoDSN = errors.New("TEST_MYSQL_DSN not set")

// ensureMultiStatements thêm multiStatements=true vào DSN nếu thiếu. Giữ nguyên nếu caller đã khai
// (kể cả khai =false — đó là lựa chọn tường minh của họ, không ghi đè).
func ensureMultiStatements(dsn string) string {
	if strings.Contains(dsn, "multiStatements=") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&multiStatements=true"
	}
	return dsn + "?multiStatements=true"
}

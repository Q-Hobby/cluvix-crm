package testhelpers

import (
	"os"
	"strings"
	"testing"
)

// TestAssertDisposableDB khoá lưới chặn "test chạm DB thật" (sự cố 2026-07-18: dropAllTables
// = DROP mọi bảng, DB-per-customer nên bằng mất sạch data của 1 khách).
//
// Bản guard cũ chỉ so với os.Getenv("DB_NAME") và KHÔNG có test nào — nên việc nó là no-op
// (không process test nào nạp .env → DB_NAME rỗng → short-circuit) sống sót nhiều tháng mà
// suite vẫn xanh. Test này tồn tại để lần hỏng sau không im lặng như vậy nữa.
func TestAssertDisposableDB(t *testing.T) {
	// Cô lập khỏi môi trường thật: DB_NAME rỗng = đúng tình huống process test bình thường.
	t.Setenv("DB_NAME", "")

	cases := []struct {
		name    string
		db      string
		wantErr bool
	}{
		{"suffix _test", "cluvix_test", false},
		{"suffix _tmp", "cluvix_qa10_tmp", false},
		{"tên DB thật", "cluvix_erp", true},
		// Tiền tố không phải hậu tố: quy ước là *_test/*_tmp, "test_x" KHÔNG đạt.
		{"tiền tố test_", "test_cluvix_erp", true},
		// Siêu chuỗi ngược: dễ lọt nếu ai đó đổi sang Contains thay vì HasSuffix.
		{"_test ở giữa", "cluvix_test_backup", true},
		{"rỗng (DSN không có tên DB)", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertDisposableDB(tc.db)
			if tc.wantErr && err == nil {
				t.Fatalf("assertDisposableDB(%q) = nil, muốn lỗi — lưới này chặn việc DROP nhầm DB thật", tc.db)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("assertDisposableDB(%q) = %v, muốn nil — chặn oan DB dùng-một-lần sẽ khiến người ta đi tìm đường lách", tc.db, err)
			}
		})
	}
}

// TestAssertDisposableDB_RealDBNameFromEnvFile chứng minh tầng 2 (so với DB thật) thật sự
// đọc được .env — tức phần đã hỏng trước 2026-09-02. Skip nếu máy không có backend/.env.
func TestAssertDisposableDB_RealDBNameFromEnvFile(t *testing.T) {
	t.Setenv("DB_NAME", "")
	real := realDBName()
	if real == "" {
		t.Skip("không đọc được DB_NAME từ backend/.env — bỏ qua (tầng 1 vẫn giữ lưới)")
	}
	if strings.HasSuffix(real, "_test") || strings.HasSuffix(real, "_tmp") {
		// DB dev lỡ đặt tên hợp quy ước: đây chính là ca mà tầng 2 sinh ra để bắt.
		if err := assertDisposableDB(real); err == nil {
			t.Fatalf("DB thật %q hợp quy ước tên nhưng vẫn được chấp nhận — tầng 2 hỏng", real)
		}
	}
}

// TestRealDBName_EnvWins: biến môi trường phải thắng file, để CI đặt DB_NAME khác được.
func TestRealDBName_EnvWins(t *testing.T) {
	t.Setenv("DB_NAME", "sentinel_db_from_env")
	if got := realDBName(); got != "sentinel_db_from_env" {
		t.Fatalf("realDBName() = %q, muốn giá trị từ env", got)
	}
}

// TestBackendRootHasEnvOrMigrations: backendRoot() phải trỏ đúng thư mục backend/.
func TestBackendRootHasEnvOrMigrations(t *testing.T) {
	root := backendRoot()
	if _, err := os.Stat(root + "/migrations"); err != nil {
		t.Fatalf("backendRoot() = %q nhưng không có migrations/ ở đó: %v", root, err)
	}
}

// TestMigrationsDir — story-04 AC5: MigrationsDir() đọc TEST_MIGRATIONS_DIR nếu có, mặc định
// giữ nguyên backend/migrations. Lý do: suốt E2/E3 (trước story-26) migrations/ đang RỖNG (schema
// mới viết lại từ story-24), integration test cần trỏ TEST_MIGRATIONS_DIR sang migrations_backup/
// (schema ERP đầy đủ ⊇ CRM) mới dựng được DB test.
func TestMigrationsDir(t *testing.T) {
	// Story-62: env TEST_MIGRATIONS_DIR bị BỎ — schema cũ (`migrations_backup`) không còn khớp code.
	// Kể cả khi env còn sót trong shell, MigrationsDir() PHẢI luôn là `backend/migrations`.
	want := backendRoot() + "/migrations"
	for _, env := range []string{"", backendRoot() + "/migrations_backup"} {
		t.Setenv("TEST_MIGRATIONS_DIR", env)
		if got := MigrationsDir(); got != want {
			t.Fatalf("TEST_MIGRATIONS_DIR=%q ⇒ MigrationsDir() = %q, muốn %q", env, got, want)
		}
	}
}

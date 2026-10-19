//go:build integration

package codegen

import (
	"errors"
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

// testDB set bởi TestMain khi có TEST_MYSQL_DSN; nil → mỗi test t.Skip.
// SchemaAll: cột include_branch_code/monthly_reset/serial_length của config_code chỉ có từ
// migration 20260714181841 trở đi, KHÔNG có ở baseline.
var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "codegen_test"})

	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		if errors.Is(err, testhelpers.ErrNoDSN) {
			log.Printf("testhelpers: %v — skipping integration tests", err)
			os.Exit(m.Run())
		}
		log.Fatalf("testhelpers: đã có TEST_MYSQL_DSN nhưng dựng schema hỏng: %v", err)
	}
	defer s.Close()
	testDB = s.DB
	// Suite ghim SetMaxOpenConns(1); bộ đếm mã khách cần kết nối THỨ HAI ngoài tx của caller
	// (nextCustomerSerial) — để 1 thì nó hết hạn chờ rồi lùi về đếm trong tx, test [2] không chứng minh
	// được gì. Các test ở đây không dựa vào biến session nên nới pool an toàn.
	if sqlDB, err := testDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(4)
	}
	os.Exit(m.Run())
}

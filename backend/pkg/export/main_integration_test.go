//go:build integration

package export

import (
	"errors"
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

// TestMain PHẢI phân biệt hai tình huống khác hẳn nhau:
//   - ErrNoDSN  → chưa set TEST_MYSQL_DSN, bỏ qua suite (dev máy local không có MySQL rảnh).
//   - lỗi khác  → hạ tầng test hỏng thật, phải Fatal.
//
// Chỉ `log` rồi `m.Run()` cho CẢ HAI là cách suite integration hỏng đi qua CI với dấu `ok` —
// xem skill/skills/testing-backend/SKILL.md §"Build tag — tách unit vs integration".
func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir())
	if err != nil {
		if errors.Is(err, testhelpers.ErrNoDSN) {
			log.Printf("pkg/export: %v — bỏ qua integration tests", err)
			os.Exit(m.Run())
		}
		log.Fatalf("pkg/export: dựng suite test thất bại: %v", err)
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

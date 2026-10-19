//go:build integration

package service

import (
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

// testDB is set by TestMain when the integration DB is available; nil otherwise.
// Integration tests guard with:  if testDB == nil { t.Skip("integration DB not available (set TEST_MYSQL_DSN)") }
var testDB *gorm.DB

func TestMain(m *testing.M) {
	// SchemaAll: chạy đủ migration sau baseline — nhiều test trong package dựa vào cột/bảng
	// thêm sau baseline (story-10 crm-standalone: test hoa hồng CTV đã xoá cùng module loyalty).
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

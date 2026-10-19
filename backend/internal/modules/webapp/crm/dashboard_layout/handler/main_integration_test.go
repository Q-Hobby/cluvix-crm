//go:build integration

package handler_test

import (
	"errors"
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if errors.Is(err, testhelpers.ErrNoDSN) {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	if err != nil {
		log.Fatalf("testhelpers: %v", err)
	}
	testDB = s.DB
	code := m.Run()
	s.Close()
	os.Exit(code)
}

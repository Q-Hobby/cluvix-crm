//go:build integration

package handler

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
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

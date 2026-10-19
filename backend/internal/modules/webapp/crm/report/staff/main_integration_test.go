//go:build integration

package staff

import (
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/helpers"
)

// testDB được TestMain set khi có DB integration; nil nếu không có.
var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	db.DB = testDB
	helpers.InitJSONColumnMap(testDB)
	helpers.InitDecimalColumnMap(testDB)
	os.Exit(m.Run())
}

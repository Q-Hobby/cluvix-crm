//go:build integration

package onboarding

import (
	"errors"
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "onboarding_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		if errors.Is(err, testhelpers.ErrNoDSN) {
			log.Printf("testhelpers: %v — skipping integration tests", err)
			os.Exit(m.Run())
		}
		// DSN có mà dựng DB hỏng ⇒ KHÔNG được im lặng skip (xanh giả).
		log.Fatalf("testhelpers: %v", err)
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

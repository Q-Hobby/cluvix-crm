//go:build integration

package seed

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

// TestMain — story-60: đọc DB DÙNG-MỘT-LẦN qua `testhelpers.New` (env `TEST_MYSQL_DSN`),
// TUYỆT ĐỐI không đọc DB_HOST/DB_USER/DB_PASS/DB_NAME (trỏ thẳng `.env`/DB dev thật). Thiếu
// TEST_MYSQL_DSN ⇒ skip toàn bộ test trong package, KHÔNG fallback DB thật.
func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

// TestSeedPreflight_Integration — READ-ONLY: dự đoán boot-seed sẽ thêm/prune gì trên DB test
// (đã nạp seed đầy đủ qua migrations_backup), KHÔNG mutate. Chạy:
//
//	TEST_MYSQL_DSN='root:testpass@(localhost:3306)/cluvix_s60_tmp?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true' \
//	TEST_MIGRATIONS_DIR=.../migrations_backup \
//	go test -tags integration -run TestSeedPreflight_Integration ./internal/modules/webapp/auth/rbac/seed/ -v
func TestSeedPreflight_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_MYSQL_DSN chưa set — skip integration test")
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(PermissionsJSON, &items); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	desired := make(map[string]bool, len(items))
	for _, it := range items {
		desired[it.Name] = true
	}

	var existing []string
	if err := testDB.Table("rbac_auth_item").Where("type = ?", 2).Pluck("name", &existing).Error; err != nil {
		t.Fatalf("scan existing: %v", err)
	}
	exSet := make(map[string]bool, len(existing))
	for _, n := range existing {
		exSet[n] = true
	}

	var pruned, added []string
	for _, n := range existing {
		if !desired[n] {
			pruned = append(pruned, n)
		}
	}
	for n := range desired {
		if !exSet[n] {
			added = append(added, n)
		}
	}

	t.Logf("PRE-FLIGHT boot-seed: existing(type=2)=%d, artifact=%d → +%d added, ~%d updated, -%d pruned",
		len(existing), len(items), len(added), len(items)-len(added), len(pruned))
	if len(added) > 0 {
		t.Logf("SẼ THÊM (%d): %v", len(added), added)
	}
	if len(pruned) > 0 {
		t.Logf("SẼ PRUNE (%d): %v", len(pruned), pruned)
	}
	maxPrune := int(math.Ceil(float64(len(existing)) * 0.30))
	if len(pruned) > maxPrune {
		t.Logf("⚠️  prune %d > guard %d (30%%) → seeder sẽ ABORT prune (an toàn, KHÔNG xoá)", len(pruned), maxPrune)
	} else {
		t.Logf("✅ prune %d <= guard %d → boot-seed an toàn", len(pruned), maxPrune)
	}
}

//go:build integration

// Integration test — seed `config_systems` (migration `*_seed_config_systems_row`, story-43 D2) là điều
// kiện để `BumpDexieTable` có tác dụng: bảng rỗng ⇒ UPDATE không WHERE là no-op ⇒ header
// `X-Dexie-Versions` rỗng mãi. Test dùng SchemaFull (có seed), KHÔNG tự chèn dòng như các test cũ.
package dexie

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaFull)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

func countConfigSystems(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testDB.Table("config_systems").Count(&n).Error; err != nil {
		t.Fatalf("đếm config_systems: %v", err)
	}
	return n
}

// Seed đúng 1 dòng và chạy LẠI file seed không nhân bản (bảng không có khoá seed ⇒ phải WHERE NOT EXISTS).
func TestSeedConfigSystems_MotDong_Idempotent(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	if n := countConfigSystems(t); n != 1 {
		t.Fatalf("sau migrate: config_systems = %d dòng, muốn 1", n)
	}
	files, _ := filepath.Glob(filepath.Join(testhelpers.MigrationsDir(), "*_seed_config_systems_row.up.sql"))
	if len(files) != 1 {
		t.Fatalf("muốn đúng 1 file *_seed_config_systems_row.up.sql, có %d", len(files))
	}
	sql, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("đọc %s: %v", files[0], err)
	}
	if err := testDB.Exec(string(sql)).Error; err != nil {
		t.Fatalf("chạy lại seed: %v", err)
	}
	if n := countConfigSystems(t); n != 1 {
		t.Fatalf("chạy lại seed: config_systems = %d dòng, muốn vẫn 1", n)
	}
}

// Trên DB vừa migrate (không chèn tay), bump phải ghi được ⇒ versions + header có giá trị.
func TestBumpDexieTable_TrenSeed_HeaderCoGiaTri(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	BumpDexieTable(testDB, "business_profile")
	v := GetDexieVersions(testDB)
	if v["business_profile"] <= 0 {
		t.Fatalf("sau BumpDexieTable: versions = %v, muốn business_profile > 0", v)
	}
	if h := SerializeDexieVersionsHeader(v); h == "" {
		t.Fatalf("X-Dexie-Versions rỗng sau bump: versions = %v", v)
	}
}

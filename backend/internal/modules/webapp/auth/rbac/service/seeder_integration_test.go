//go:build integration

package service

import (
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	rbacSeed "erpCluvix/internal/modules/webapp/auth/rbac/seed"
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

// TestSeedPermissions_Integration — e2e seed/prune/guard trên DB test (BMAD perm-codegen-seeder
// story-07, dồn về `internal/testhelpers` ở story-60). SAFE: dùng marker `__seedtest_*` + defer
// cleanup; items = snapshot quyền seed sẵn qua migrations_backup (giữ nguyên description → upsert
// no-op, chỉ bump updated_at). KHÔNG xoá quyền thật (guard tỉ lệ + marker cô lập). Chạy:
//
//	TEST_MYSQL_DSN='root:testpass@(localhost:3306)/cluvix_s60_tmp?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true' \
//	TEST_MIGRATIONS_DIR=.../migrations_backup \
//	go test -tags integration -run TestSeedPermissions_Integration ./internal/modules/webapp/auth/rbac/service/ -v
func TestSeedPermissions_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_MYSQL_DSN chưa set — skip integration test")
	}
	db := testDB
	svc := &RbacService{DB: db} // Cache/TokenSvc nil — nil-safe (các invalidate đều guard nil)

	var rows []struct {
		Name        string
		Description string
	}
	// Story-62: schema mới KHÔNG seed catalog quyền bằng SQL (seed_admin chỉ có role `admin`) — app
	// nạp từ artifact embed lúc boot (bootstrap.seedPermissions). Dựng catalog đúng đường đó, không
	// thì test SKIP âm thầm trên mọi DB test.
	var boot []SyncPermInput
	if err := json.Unmarshal(rbacSeed.PermissionsJSON, &boot); err != nil {
		t.Fatalf("parse artifact quyền: %v", err)
	}
	if _, err := svc.SeedPermissions(boot, false); err != nil {
		t.Fatalf("dựng catalog từ artifact: %v", err)
	}
	if err := db.Table("rbac_auth_item").Where("type = ?", 2).Find(&rows).Error; err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(rows) < 5 {
		t.Skipf("cần catalog >= 5 quyền để test guard tỉ lệ, hiện %d", len(rows))
	}
	items := make([]SyncPermInput, len(rows))
	for i, r := range rows {
		items[i] = SyncPermInput{Name: r.Name, Description: r.Description}
	}

	const marker = "__seedtest_prune_marker"
	now := time.Now().Unix()
	db.Exec("DELETE FROM rbac_auth_item WHERE name = ?", marker)
	if err := db.Exec("INSERT INTO rbac_auth_item (name,type,description,created_at,updated_at) VALUES (?,2,?,?,?)",
		marker, "seedtest", now, now).Error; err != nil {
		t.Fatalf("inject marker: %v", err)
	}
	defer db.Exec("DELETE FROM rbac_auth_item WHERE name = ?", marker)

	// AC4: seed prune=true (marker KHÔNG trong items) → prune marker, giữ nguyên quyền thật.
	res, err := svc.SeedPermissions(items, true)
	if err != nil {
		t.Fatalf("AC4 seed: %v", err)
	}
	var cnt int64
	db.Table("rbac_auth_item").Where("name = ?", marker).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("AC4: marker phải bị prune, còn %d", cnt)
	}
	var realCnt int64
	db.Table("rbac_auth_item").Where("type = ?", 2).Count(&realCnt)
	if int(realCnt) != len(rows) {
		t.Fatalf("AC4: số quyền thật đổi %d → %d (không được)", len(rows), realCnt)
	}
	found := false
	for _, p := range res.Pruned {
		if p == marker {
			found = true
		}
	}
	if !found {
		t.Fatalf("AC4: res.Pruned phải chứa marker, got %v", res.Pruned)
	}

	// AC3: idempotent — seed lại → không prune gì.
	res2, err := svc.SeedPermissions(items, true)
	if err != nil {
		t.Fatalf("AC3 seed lần 2: %v", err)
	}
	if len(res2.Pruned) != 0 {
		t.Fatalf("AC3: idempotent — lần 2 không được prune, got %v", res2.Pruned)
	}

	// AC5: artifact rỗng → error, catalog KHÔNG đổi.
	var before int64
	db.Table("rbac_auth_item").Where("type = ?", 2).Count(&before)
	if _, err := svc.SeedPermissions(nil, true); err == nil {
		t.Fatal("AC5: artifact rỗng phải trả lỗi ABORT")
	}
	var after int64
	db.Table("rbac_auth_item").Where("type = ?", 2).Count(&after)
	if before != after {
		t.Fatalf("AC5: artifact rỗng không được đổi catalog %d → %d", before, after)
	}

	// AC (dry-run mặc định SEED_PRUNE=false): re-inject marker → seed prune=false →
	// KHÔNG xoá, marker vẫn còn, res.WouldPrune chứa marker, res.Pruned rỗng.
	db.Exec("DELETE FROM rbac_auth_item WHERE name = ?", marker)
	db.Exec("INSERT INTO rbac_auth_item (name,type,description,created_at,updated_at) VALUES (?,2,?,?,?)",
		marker, "seedtest", now, now)
	res3, err := svc.SeedPermissions(items, false)
	if err != nil {
		t.Fatalf("dry-run seed: %v", err)
	}
	if len(res3.Pruned) != 0 {
		t.Fatalf("dry-run: KHÔNG được xoá gì, got Pruned=%v", res3.Pruned)
	}
	var markerCnt int64
	db.Table("rbac_auth_item").Where("name = ?", marker).Count(&markerCnt)
	if markerCnt != 1 {
		t.Fatalf("dry-run: marker phải còn nguyên, count=%d", markerCnt)
	}
	foundWould := false
	for _, p := range res3.WouldPrune {
		if p == marker {
			foundWould = true
		}
	}
	if !foundWould {
		t.Fatalf("dry-run: res.WouldPrune phải chứa marker, got %v", res3.WouldPrune)
	}
}

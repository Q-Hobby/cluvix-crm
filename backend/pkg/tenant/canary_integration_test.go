//go:build integration

// Phase 0.2 canary — guardrail trước khi refactor kiến trúc.
// Mục đích:
//  1. Lock-in logic ScopedDBNullBranch trên 1 synthetic table (`branch_behavior_integration_test.go`).
//  2. Phát hiện drift giữa branchTables (scope.go) và schema thật (`backend/migrations`):
//     - White-list có entry mà table không có cột branch_id → orphan.
//     - Table có cột branch_id mà không có trong white-list → potential leak.
//
// Chạy: `go test -tags=integration ./pkg/tenant/...`

package tenant

import (
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

// scopeExempt — tables có cột branch_id nhưng KHÔNG cần scope (vd log lưu snapshot chi nhánh để biết
// record gốc thuộc đâu, không phải cổng truy cập). Mọi entry phải có lý do. Story-62: schema mới
// (`backend/migrations`) chỉ còn bảng keep-set ⇒ bỏ mọi entry của bảng đã không còn tồn tại.
var scopeExempt = map[string]string{
	"app_event_log":   "log chẩn đoán pkg/applog, chỉ đọc bằng SQL vận hành, không có endpoint list theo phạm vi; branch_id là snapshot ngữ cảnh (NULL được)",
	"attachment":      "branch_id là snapshot chi nhánh lúc upload, KHÔNG phải quyền sở hữu: file gắn ref cha (customer/appointment) vốn xem cross-branch; ScopedDBNullBranch(\"attachment\") chưa từng lọc branch (không nằm branchTables) — đăng ký sẽ ĐỔI hành vi (ẩn file khách cross-branch)",
	"print_templates": "branch_id là THAM SỐ mẫu in (0 = mẫu chung, không phải NULL), handler lọc tường minh (GetDefault branch→0, ApplyFilter branch_id); đăng ký branchTables sẽ ẩn mẫu chung branch_id=0",
}

func TestMain(m *testing.M) {
	// SchemaFull: TestCrossBranchOption_OnOff cần dòng `user_option` 18/23 — schema mới đặt chúng ở
	// seed (`*_seed_lookup`), không còn nằm trong migration schema như `migrations_backup`.
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaFull)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB

	if err := setupCanaryTable(testDB); err != nil {
		log.Fatalf("canary table setup: %v", err)
	}
	// Register canary table in branch list so ScopedDB* sees it.
	branchTables["canary_scope"] = true

	os.Exit(m.Run())
}

func setupCanaryTable(db *gorm.DB) error {
	if err := db.Exec(`DROP TABLE IF EXISTS canary_scope`).Error; err != nil {
		return err
	}
	return db.Exec(`
		CREATE TABLE canary_scope (
			id INT AUTO_INCREMENT PRIMARY KEY,
			label VARCHAR(64) NOT NULL,
			branch_id INT NULL
		)
	`).Error
}

// up returns a *uint pointer (nil for 0 → SQL NULL).
func up(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

func ctxWith(branchActive uint, branchIds []uint) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
	}
	if len(branchIds) > 0 {
		c.Set("branchIds", branchIds)
	}
	return c
}

// TestWhitelist_NoDrift — đảm bảo tenantTables/branchTables khớp schema thật.
// Phát hiện 2 loại lỗi:
//  1. Entry trong white-list nhưng table không có cột → orphan, scope no-op âm thầm.
//  2. Table có cột branch_id nhưng vắng white-list → potential tenant leak
//     (cho phép qua scopeExempt với justification).
func TestWhitelist_NoDrift(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}

	checkDrift(t, "branch_id", branchTables)
}

func checkDrift(t *testing.T, column string, whitelist map[string]bool) {
	t.Helper()

	// 1. Tables trong DB có cột này.
	var dbTables []string
	if err := testDB.Table("INFORMATION_SCHEMA.COLUMNS").
		Where("TABLE_SCHEMA = DATABASE() AND COLUMN_NAME = ?", column).
		Pluck("TABLE_NAME", &dbTables).Error; err != nil {
		t.Fatalf("query INFORMATION_SCHEMA: %v", err)
	}
	dbSet := make(map[string]bool, len(dbTables))
	for _, tbl := range dbTables {
		dbSet[tbl] = true
	}

	// 2. Orphan check: white-list → DB.
	for tbl := range whitelist {
		if tbl == "canary_scope" {
			continue
		}
		if !dbSet[tbl] {
			t.Errorf("[%s] %q in whitelist but table has no %s column (orphan — scope is no-op)",
				column, tbl, column)
		}
	}

	// 3. Leak check: DB → white-list (+ exempt).
	for _, tbl := range dbTables {
		if tbl == "canary_scope" {
			continue
		}
		if whitelist[tbl] {
			continue
		}
		if reason, ok := scopeExempt[tbl]; ok {
			t.Logf("[%s] %q exempt from scope — reason: %s", column, tbl, reason)
			continue
		}
		t.Errorf("[%s] table %q has %s column but missing from whitelist — potential tenant leak. "+
			"Add to %sTables in pkg/tenant/scope.go, or to scopeExempt with justification.",
			column, tbl, column, scopePrefix(column))
	}
}

func scopePrefix(column string) string {
	if column == "branch_id" {
		return "branch"
	}
	return "tenant"
}

func stringSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

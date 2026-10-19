//go:build integration

// Plan 09 Phase 2 — integration test cover IDOR + happy path cho BaseRepo[T].
// Pattern follow pkg/tenant/canary_integration_test.go: synthetic table được
// register vào tenant whitelist trong TestMain, sau đó chạy CRUD qua BaseRepo
// và assert hành vi (in-scope vs out-of-scope).
//
// Chạy: `go test -tags=integration ./pkg/repository/...`

package repository

import (
	"errors"
	"log"
	"net/http/httptest"
	"os"
	"testing"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var testDB *gorm.DB

// CanaryRow — fake model cho synthetic table. KHÔNG dùng GORM model tag —
// repo dùng .Table(name) trực tiếp nên struct chỉ cần column tags.
type CanaryRow struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	Label     string `gorm:"column:label"`
	BranchID  *uint  `gorm:"column:branch_id"`
	UpdatedAt int64  `gorm:"column:updated_at;autoUpdateTime:false;autoCreateTime:false"`
}

// TableName — bắt buộc: BaseRepo.Create() gọi r.DB.Create(m) dùng GORM naming convention mặc
// định (pluralize) khi model KHÔNG implement TableName(), nên sẽ insert nhầm bảng "canary_rows"
// (không tồn tại) thay vì "canary_repo". Test đỏ có sẵn trước story-60 (TestCreate_NoAutoFill*)
// — bug ở test fixture, KHÔNG phải ở BaseRepo (entity thật đều có TableName(), xem
// crm/customer/dao/entity.go).
func (CanaryRow) TableName() string { return "canary_repo" }

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB

	if err := setupCanaryTable(testDB); err != nil {
		log.Fatalf("canary table setup: %v", err)
	}
	// Register vào tenant scope maps để ScopedDBNullBranch nhận diện.
	tenant.RegisterTestTable("canary_repo", true)

	os.Exit(m.Run())
}

func setupCanaryTable(db *gorm.DB) error {
	if err := db.Exec(`DROP TABLE IF EXISTS canary_repo`).Error; err != nil {
		return err
	}
	return db.Exec(`
		CREATE TABLE canary_repo (
			id INT AUTO_INCREMENT PRIMARY KEY,
			label VARCHAR(64) NOT NULL,
			branch_id INT NULL,
			updated_at BIGINT NOT NULL DEFAULT 0
		)
	`).Error
}

func up(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// ctxWith — ctx chỉ có phạm vi chi nhánh (session không còn thương hiệu).
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

// seedRows — repopulate canary_repo với 5 row chuẩn:
//
//	A: branch=10 (in-scope user-1)
//	B: branch=11 (in-scope user-1 nếu list branch)
//	C: branch=20 (NEVER in-scope user-1)
//	D, E: branch=NULL (chưa gán chi nhánh — luôn accessible)
func seedRows(t *testing.T) map[string]uint {
	t.Helper()
	testDB.Exec("DELETE FROM canary_repo")
	seeds := []CanaryRow{
		{Label: "A", BranchID: up(10)},
		{Label: "B", BranchID: up(11)},
		{Label: "C", BranchID: up(20)},
		{Label: "D", BranchID: nil},
		{Label: "E", BranchID: nil},
	}
	ids := map[string]uint{}
	for _, s := range seeds {
		if err := testDB.Table("canary_repo").Create(&s).Error; err != nil {
			t.Fatalf("seed %s: %v", s.Label, err)
		}
		ids[s.Label] = s.ID
	}
	return ids
}

// ───────────────────────── FindByID ─────────────────────────

func TestFindByID_InScope_OK(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil) // branch=10

	got, err := repo.FindByID(c, ids["A"], nil)
	if err != nil {
		t.Fatalf("FindByID(A) want nil err, got %v", err)
	}
	if got == nil || got.Label != "A" {
		t.Errorf("got=%+v want label=A", got)
	}
}

// CRITICAL: out-of-scope record phải trả ErrNotFound, KHÔNG leak existence.
func TestFindByID_OutOfScope_ReturnsNotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")

	// User ở branch=10; record C thuộc branch=20.
	c := ctxWith(10, nil)
	got, err := repo.FindByID(c, ids["C"], nil)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("FindByID(C) cross-tenant want ErrNotFound, got err=%v got=%+v", err, got)
	}
	if got != nil {
		t.Errorf("got=%+v want nil (no leak)", got)
	}
}

// branch_id IS NULL records (chưa gán chi nhánh) phải accessible.
func TestFindByID_NullBranch_Accessible(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")

	c := ctxWith(10, nil) // active branch=10, nhưng D có branch=NULL
	got, err := repo.FindByID(c, ids["D"], nil)
	if err != nil {
		t.Fatalf("FindByID(D) NULL branch want nil err, got %v", err)
	}
	if got == nil || got.Label != "D" {
		t.Errorf("got=%+v want D", got)
	}
}

func TestFindByID_SkipTenant_BypassesScope(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	// SkipTenant=true → load được cross-tenant. Documented escape hatch.
	got, err := repo.FindByID(c, ids["C"], &Options{SkipTenant: true})
	if err != nil {
		t.Fatalf("SkipTenant FindByID(C) want nil err, got %v", err)
	}
	if got == nil || got.Label != "C" {
		t.Errorf("got=%+v want C", got)
	}
}

// ───────────────────────── List + Count ─────────────────────────

func TestList_ScopedToTenant(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil) // branch=10 → A + D + E (branch NULL luôn lấy, xem CLAUDE.md B.1)

	var items []CanaryRow
	if err := repo.List(c, &items); err != nil {
		t.Fatalf("List: %v", err)
	}
	labels := map[string]bool{}
	for _, it := range items {
		labels[it.Label] = true
	}
	if !labels["A"] || !labels["D"] || !labels["E"] {
		t.Errorf("missing A, D hoặc E (branch NULL luôn lấy), got=%v", labels)
	}
	if labels["B"] || labels["C"] {
		t.Errorf("leaked out-of-scope (khác branch): got=%v", labels)
	}
}

// TestCount_ScopedToTenant — story-60: expected=3 (A+D+E), không phải 2 — trước story-60 test này
// đỏ có sẵn (assert cũ quên cộng E, record branch_id NULL LUÔN được lấy kể cả khi active, xem
// CLAUDE.md B.1). Không phải hành vi mới do story-60 gây ra, chỉ sửa lại expectation cho đúng.
func TestCount_ScopedToTenant(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	n, err := repo.Count(c)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Errorf("Count want=3 (A+D+E) got=%d", n)
	}
}

// ───────────────────────── UpdateByID ─────────────────────────

func TestUpdateByID_InScope_OK(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	err := repo.UpdateByID(c, ids["A"], map[string]any{"label": "A-updated"}, nil)
	if err != nil {
		t.Fatalf("UpdateByID(A): %v", err)
	}

	var row CanaryRow
	testDB.Table("canary_repo").Where("id = ?", ids["A"]).Take(&row)
	if row.Label != "A-updated" {
		t.Errorf("label not updated, got=%s", row.Label)
	}
}

// CRITICAL: UPDATE cross-tenant phải return ErrNotFound, KHÔNG ghi DB.
func TestUpdateByID_OutOfScope_NoWrite(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil) // user-1

	err := repo.UpdateByID(c, ids["C"], map[string]any{"label": "HACKED"}, nil)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}

	// Verify row C UNCHANGED.
	var row CanaryRow
	testDB.Table("canary_repo").Where("id = ?", ids["C"]).Take(&row)
	if row.Label != "C" {
		t.Errorf("CRITICAL: cross-tenant UPDATE wrote DB! label=%s", row.Label)
	}
}

// ───────────────────────── DeleteByID ─────────────────────────

func TestDeleteByID_InScope_OK(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	if err := repo.DeleteByID(c, ids["A"], nil); err != nil {
		t.Fatalf("DeleteByID(A): %v", err)
	}

	var n int64
	testDB.Table("canary_repo").Where("id = ?", ids["A"]).Count(&n)
	if n != 0 {
		t.Errorf("A still exists after delete")
	}
}

// CRITICAL: DELETE cross-tenant phải return ErrNotFound, KHÔNG xoá DB.
func TestDeleteByID_OutOfScope_NoDelete(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	err := repo.DeleteByID(c, ids["C"], nil)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}

	var n int64
	testDB.Table("canary_repo").Where("id = ?", ids["C"]).Count(&n)
	if n != 1 {
		t.Errorf("CRITICAL: cross-tenant DELETE wrote DB! remaining=%d", n)
	}
}

// ───────────────────────── UpdateWithUpdatedAt ─────────────────────────

func TestUpdateWithUpdatedAt_Match_BumpsUpdatedAt(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	err := repo.UpdateWithUpdatedAt(c, ids["A"], 0, map[string]any{"label": "v1"}, nil)
	if err != nil {
		t.Fatalf("UpdateWithUpdatedAt: %v", err)
	}
	var row CanaryRow
	testDB.Table("canary_repo").Where("id = ?", ids["A"]).Take(&row)
	if row.UpdatedAt <= 0 {
		t.Errorf("updated_at not bumped, got=%d", row.UpdatedAt)
	}
	if row.Label != "v1" {
		t.Errorf("label not updated, got=%s", row.Label)
	}
	// Ghi lần 2 NGAY (cùng giây) với giá trị vừa đọc ⇒ phải tăng nghiêm ngặt; giá trị cũ ⇒ conflict.
	prev := row.UpdatedAt
	if err := repo.UpdateWithUpdatedAt(c, ids["A"], prev, map[string]any{"label": "v2"}, nil); err != nil {
		t.Fatalf("second update: %v", err)
	}
	testDB.Table("canary_repo").Where("id = ?", ids["A"]).Take(&row)
	if row.UpdatedAt <= prev {
		t.Errorf("updated_at must strictly increase, prev=%d got=%d", prev, row.UpdatedAt)
	}
	if err := repo.UpdateWithUpdatedAt(c, ids["A"], prev, map[string]any{"label": "stale"}, nil); !errors.Is(err, errs.ErrVersionConflict) {
		t.Errorf("stale updated_at same second: want ErrVersionConflict, got %v", err)
	}
}

func TestUpdateWithUpdatedAt_Mismatch_ReturnsConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	err := repo.UpdateWithUpdatedAt(c, ids["A"], 999, map[string]any{"label": "stale"}, nil)
	if !errors.Is(err, errs.ErrVersionConflict) {
		t.Errorf("want ErrVersionConflict, got %v", err)
	}
}

// CRITICAL: out-of-scope record → ErrNotFound (KHÔNG leak qua ErrVersionConflict).
func TestUpdateWithUpdatedAt_OutOfScope_ReturnsNotFound(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	ids := seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil) // user-1 ở branch 10; C thuộc branch 20

	err := repo.UpdateWithUpdatedAt(c, ids["C"], 0, map[string]any{"label": "HACKED"}, nil)
	if !errors.Is(err, errs.ErrNotFound) {
		t.Errorf("want ErrNotFound (no leak), got %v", err)
	}

	// Verify row C UNCHANGED.
	var row CanaryRow
	testDB.Table("canary_repo").Where("id = ?", ids["C"]).Take(&row)
	if row.Label != "C" || row.UpdatedAt != 0 {
		t.Errorf("CRITICAL: cross-tenant UpdateWithUpdatedAt wrote DB! row=%+v", row)
	}
}

// ───────────────────────── Create ─────────────────────────

// Create KHÔNG scope — caller trách nhiệm set branch_id (và kiểm quyền chi nhánh).
// Test confirm INSERT đi qua, không có magic auto-fill/override từ ctx.
func TestCreate_NoAutoFillBranchID(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	seedRows(t)
	repo := New[CanaryRow](testDB, "canary_repo")
	c := ctxWith(10, nil)

	// Caller set branch_id=99 explicitly. BaseRepo phải tôn trọng giá trị
	// đó, KHÔNG override bằng branchActive=10 từ context.
	row := &CanaryRow{Label: "new", BranchID: up(99)}
	if err := repo.Create(c, row); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var got CanaryRow
	testDB.Table("canary_repo").Where("id = ?", row.ID).Take(&got)
	if got.BranchID == nil || *got.BranchID != 99 {
		t.Errorf("CRITICAL: Create overrode branch_id! got=%+v", got.BranchID)
	}
}

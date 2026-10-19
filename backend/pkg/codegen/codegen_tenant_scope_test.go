//go:build integration

package codegen

import "testing"

// findConfig: chi nhánh → toàn hệ thống (1 doanh nghiệp/DB; story-59 bỏ tầng thương hiệu).

// Không chi nhánh ⇒ lấy row có sẵn của bảng (hành vi cũ của caller không biết chi nhánh — giữ).
func TestFindConfigNoBranchFallsBackToAnyRow(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	tx := testDB.Begin()
	defer tx.Rollback()

	if err := tx.Exec("INSERT INTO branch (id, name, code, address) VALUES (201, 'Chi nhánh C', 'CNC', 'Địa chỉ C')").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`INSERT INTO config_code
		(branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
		VALUES (201, 'Phiếu thu', 'payment', 'PT', 1, 1, 2, 0, 0)`).Error; err != nil {
		t.Fatal(err)
	}

	// `payment` (bảng keep-set; trước dùng `warehouse` — bảng không còn trong schema mới, story-62).
	got := findConfig(tx, "payment", nil)
	if got == nil || got.BranchID == nil || *got.BranchID != 201 || got.BranchCode != "CNC" {
		t.Fatalf("findConfig(không chi nhánh) = %+v, muốn row chi nhánh 201 (CNC)", got)
	}
	if got := findConfig(tx, "bang_khong_co", nil); got != nil {
		t.Fatalf("bảng chưa seed = %+v, muốn nil", got)
	}
}

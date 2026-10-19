//go:build integration

// Integration test — `AppendID` round-trip qua DB thật (cột `user.branchId` kiểu JSON).
package userscope

import (
	"testing"

	"erpCluvix/internal/testhelpers"
)

func setUserColumnRaw(t *testing.T, userID uint, column, sqlExpr string) {
	t.Helper()
	if err := testDB.Exec("UPDATE user SET "+column+" = "+sqlExpr+" WHERE id = ?", userID).Error; err != nil {
		t.Fatalf("setUserColumnRaw(%s = %s): %v", column, sqlExpr, err)
	}
}

func readUserColumn(t *testing.T, userID uint, column string) *string {
	t.Helper()
	var row struct {
		Value *string `gorm:"column:value"`
	}
	if err := testDB.Table("user").Select(column+" AS value").Where("id = ?", userID).Take(&row).Error; err != nil {
		t.Fatalf("readUserColumn(%s): %v", column, err)
	}
	return row.Value
}

// TestAppendID_DBRoundTrip_EmptyLikeValues — 3 giá trị TÁI HIỆN ĐƯỢC thật trên cột JSON
// (`NULL`, JSON null literal `'null'`, mảng rỗng `'[]'`) đều cho kết quả `[newID]`.
func TestAppendID_DBRoundTrip_EmptyLikeValues(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	tenant, err := testhelpers.CreateTestTenant(testDB)
	if err != nil {
		t.Fatalf("CreateTestTenant: %v", err)
	}

	cases := []struct {
		name    string
		sqlExpr string
	}{
		{"sql_null", "NULL"},
		{"json_null_literal", "'null'"},
		{"empty_array", "'[]'"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userID := testhelpers.CreateUser(t, testDB, "us03", tenant)
			setUserColumnRaw(t, userID, ColumnBranchIDs, tc.sqlExpr)

			if err := AppendID(testDB, userID, ColumnBranchIDs, 999); err != nil {
				t.Fatalf("AppendID: %v", err)
			}

			got := readUserColumn(t, userID, ColumnBranchIDs)
			if got == nil || *got != "[999]" {
				t.Fatalf("branchId sau AppendID = %v, muốn [999]", got)
			}
		})
	}
}

// TestAppendID_DBRoundTrip_AlreadyPresent_Idempotent — id đã có trong mảng ⇒ không đổi, trả nil.
func TestAppendID_DBRoundTrip_AlreadyPresent_Idempotent(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	tenant, err := testhelpers.CreateTestTenant(testDB)
	if err != nil {
		t.Fatalf("CreateTestTenant: %v", err)
	}
	userID := testhelpers.CreateUser(t, testDB, "us03", tenant)
	setUserColumnRaw(t, userID, ColumnBranchIDs, "'[5, 7]'")

	if err := AppendID(testDB, userID, ColumnBranchIDs, 7); err != nil {
		t.Fatalf("AppendID: %v", err)
	}

	got := readUserColumn(t, userID, ColumnBranchIDs)
	if got == nil || *got != "[5, 7]" {
		t.Fatalf("branchId đã có id 7 vẫn phải giữ nguyên, hiện = %v", got)
	}
}

// TestAppendID_DBRoundTrip_InvalidJSON_NoOverwrite — cột chứa JSON hợp lệ nhưng KHÔNG phải mảng số
// (JSON string scalar) ⇒ AppendID trả error, cột giữ nguyên giá trị cũ (không âm thầm ghi đè).
func TestAppendID_DBRoundTrip_InvalidJSON_NoOverwrite(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	tenant, err := testhelpers.CreateTestTenant(testDB)
	if err != nil {
		t.Fatalf("CreateTestTenant: %v", err)
	}
	userID := testhelpers.CreateUser(t, testDB, "us03", tenant)
	// JSON hợp lệ về mặt cú pháp MySQL (cột JSON chấp nhận) nhưng không phải mảng số.
	setUserColumnRaw(t, userID, ColumnBranchIDs, `'"corrupt"'`)

	if err := AppendID(testDB, userID, ColumnBranchIDs, 999); err == nil {
		t.Fatal("AppendID với JSON không phải mảng số phải trả error")
	}

	got := readUserColumn(t, userID, ColumnBranchIDs)
	if got == nil || *got != `"corrupt"` {
		t.Fatalf("branchId phải giữ nguyên '\"corrupt\"' sau lỗi, hiện = %v", got)
	}
}

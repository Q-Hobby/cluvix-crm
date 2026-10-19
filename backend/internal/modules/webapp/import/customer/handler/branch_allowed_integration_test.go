//go:build integration

// Story-40 (E6/D15): kiểm chi nhánh của dòng import dùng `importBranchAllowed` (chỉ còn điều kiện
// chi nhánh). Giữ nguyên 2 điều kiện: chi nhánh thuộc quyền người
// gọi (`branchIds` của session) VÀ là chi nhánh có thật.
//
// (Handler `ImportCustomerFromExcel` đọc file trên máy chủ theo env IMPORT_CUSTOMER_FILE — nhánh
// env có unit test riêng `import_file_env_test.go`; ở đây test đúng hàm kiểm handler gọi cho từng dòng.)
package handler

import (
	"log"
	"os"
	"testing"

	"erpCluvix/internal/testhelpers"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB

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

func TestImportBranchAllowed(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	other := testhelpers.CreateBranch(t, testDB)

	c := testhelpers.TenantCtx(tn.BranchID, 1) // branchIds = [tn.BranchID]
	require.True(t, importBranchAllowed(c, testDB, tn.BranchID), "chi nhánh trong quyền + có thật ⇒ cho phép")
	require.False(t, importBranchAllowed(c, testDB, other), "chi nhánh có thật nhưng NGOÀI quyền ⇒ từ chối")

	c.Set("branchIds", []uint{tn.BranchID, 987654321})
	require.False(t, importBranchAllowed(c, testDB, 987654321), "trong quyền nhưng KHÔNG tồn tại ⇒ từ chối")

	// Session chỉ có chi nhánh (TenantCtx) vẫn cho phép chi nhánh hợp lệ.
	c2 := testhelpers.TenantCtx(tn.BranchID, 1)
	require.True(t, importBranchAllowed(c2, testDB, tn.BranchID))
}

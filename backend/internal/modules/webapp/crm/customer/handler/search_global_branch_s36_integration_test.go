//go:build integration

package handler

// Story-36 (E6/D15): SearchGlobal chỉ còn vế chi nhánh, vế BRANCH phải giữ y hệt —
// branchActive thắng branchIds, luôn kèm `branch_id IS NULL`, và user_option id=18 (cross-branch)
// BẬT thì bỏ lọc chi nhánh. Test viết TRƯỚC khi sửa (xanh trên code cũ), giữ xanh sau khi sửa.

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	customerService "erpCluvix/internal/modules/webapp/crm/customer/service"
	"erpCluvix/internal/testhelpers"
)

// setCrossBranchOption đặt user_option id=18 và khôi phục giá trị cũ khi test xong. Story-62:
// SchemaAll không chạy file seed ⇒ tự chèn dòng id=18 (khuôn seed_lookup) nếu thiếu, xoá khi xong.
func setCrossBranchOption(t *testing.T, val int) {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Table("user_option").Where("id = 18").Count(&n).Error)
	if n == 0 {
		require.NoError(t, testDB.Exec("INSERT IGNORE INTO user_option (id, name, options, status, SortOrder) VALUES (18, 'Hiển thị dữ liệu toàn chi nhánh', 0, 1, 18)").Error)
		t.Cleanup(func() { testDB.Exec("DELETE FROM user_option WHERE id = 18") })
	}
	require.NoError(t, testDB.Table("user_option").Where("id = 18").Count(&n).Error)
	require.EqualValues(t, 1, n, "không dựng được user_option id=18 — không dựng được ca cross-branch")
	var old int
	require.NoError(t, testDB.Table("user_option").Where("id = 18").Pluck("options", &old).Error)
	require.NoError(t, testDB.Exec("UPDATE user_option SET options = ? WHERE id = 18", val).Error)
	t.Cleanup(func() { testDB.Exec("UPDATE user_option SET options = ? WHERE id = 18", old) })
}

// searchGlobalAs gọi SearchGlobal với phạm vi session tự dựng. branchActive = 0 ⇒ không đặt khoá.
func searchGlobalAs(t *testing.T, keyword string, branchActive uint, branchIDs []uint) []uint {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/crm/customer/search-global?keyword="+url.QueryEscape(keyword), nil)
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
	}
	if branchIDs != nil {
		c.Set("branchIds", branchIDs)
	}
	h := &Handler{DB: testDB, CustomerSvc: customerService.NewCustomerService(testDB)}
	h.SearchGlobal(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var rows []struct {
		ID uint `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rows), "body: %s", w.Body.String())
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestSearchGlobal_BranchScopeGiuNguyen(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	kw := fmt.Sprintf("S36SG%d", time.Now().UnixNano())

	mk := func(suffix string, branch *uint) uint {
		id := uint(testhelpers.CreateScopedCustomer(t, testDB, tn, branch))
		require.NoError(t, testDB.Exec("UPDATE customer SET fullname = ? WHERE id = ?", kw+suffix, id).Error)
		return id
	}
	branchA := tn.BranchID
	inA := mk("A", &branchA)
	inB := mk("B", &branchB)
	noBranch := mk("N", nil)

	t.Run("CrossBranchTat_BranchActiveA", func(t *testing.T) {
		setCrossBranchOption(t, 0)
		got := searchGlobalAs(t, kw, branchA, []uint{branchA, branchB})
		require.ElementsMatch(t, []uint{inA, noBranch}, got, "chi nhánh A chỉ thấy A + chưa gán chi nhánh")
	})
	t.Run("CrossBranchTat_BranchActiveB", func(t *testing.T) {
		setCrossBranchOption(t, 0)
		got := searchGlobalAs(t, kw, branchB, []uint{branchA, branchB})
		require.ElementsMatch(t, []uint{inB, noBranch}, got)
	})
	t.Run("CrossBranchTat_ChiCoBranchIds", func(t *testing.T) {
		setCrossBranchOption(t, 0)
		got := searchGlobalAs(t, kw, 0, []uint{branchB})
		require.ElementsMatch(t, []uint{inB, noBranch}, got, "không có branch active ⇒ lọc theo danh sách branchIds")
	})
	t.Run("CrossBranchBat_ThayMoiChiNhanh", func(t *testing.T) {
		setCrossBranchOption(t, 1)
		got := searchGlobalAs(t, kw, branchA, []uint{branchA})
		require.ElementsMatch(t, []uint{inA, inB, noBranch}, got, "option 18 BẬT ⇒ bỏ lọc chi nhánh")
	})
}

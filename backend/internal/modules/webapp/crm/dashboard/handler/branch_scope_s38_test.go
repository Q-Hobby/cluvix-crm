package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newBranchOnlyContext dựng gin.Context chỉ có branch scope — mô phỏng session sau story-38
// (session chỉ còn phạm vi chi nhánh).
func newBranchOnlyContext(branchActive uint, branchIDs []uint) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
	}
	if len(branchIDs) > 0 {
		c.Set("branchIds", branchIDs)
	}
	return c
}

// Neo hành vi branch của buildDashboardScope (story-38): branchActive thắng branchIDs; ngoặc
// OR branch_id IS NULL giữ nguyên.
func TestBuildDashboardScope_BranchActiveGiuNguyen(t *testing.T) {
	c := newBranchOnlyContext(5, nil)
	frag, args := buildDashboardScope(c, "a")
	want := " AND (a.branch_id = ? OR a.branch_id IS NULL)"
	if frag != want {
		t.Fatalf("frag = %q, want %q", frag, want)
	}
	if len(args) != 1 || args[0] != uint(5) {
		t.Fatalf("args = %v, want [5]", args)
	}
}

func TestBuildDashboardScope_BranchIDsGiuNguyen(t *testing.T) {
	c := newBranchOnlyContext(0, []uint{2, 3})
	frag, args := buildDashboardScope(c, "a")
	want := " AND (a.branch_id IN ? OR a.branch_id IS NULL)"
	if frag != want {
		t.Fatalf("frag = %q, want %q", frag, want)
	}
	if len(args) != 1 {
		t.Fatalf("args = %v, want 1 arg", args)
	}
}

func TestBuildDashboardScope_KhongCoBranchThiKhongCoVe(t *testing.T) {
	c := newBranchOnlyContext(0, nil)
	frag, args := buildDashboardScope(c, "a")
	if frag != "" || len(args) != 0 {
		t.Fatalf("frag=%q args=%v, want rong", frag, args)
	}
}

// buildDashboardScopeOpt: branchID param override thắng context, vẫn giữ ngoặc OR IS NULL.
func TestBuildDashboardScopeOpt_OverrideBranchGiuNguyen(t *testing.T) {
	c := newBranchOnlyContext(5, nil)
	frag, args := buildDashboardScopeOpt(c, "a", 9)
	want := " AND (a.branch_id = ? OR a.branch_id IS NULL)"
	if frag != want {
		t.Fatalf("frag = %q, want %q", frag, want)
	}
	if len(args) != 1 || args[0] != 9 {
		t.Fatalf("args = %v, want [9]", args)
	}
}

// tenantCacheKey: PHẢI phân biệt theo branch (chống cache poisoning xuyên chi nhánh).
func TestTenantCacheKey_PhanBietTheoBranchActive(t *testing.T) {
	c1 := newBranchOnlyContext(1, nil)
	c2 := newBranchOnlyContext(2, nil)
	if tenantCacheKey(c1) == tenantCacheKey(c2) {
		t.Fatalf("cache key phai khac nhau giua 2 branch active khac nhau")
	}
}

func TestTenantCacheKey_PhanBietTheoBranchIDs(t *testing.T) {
	c1 := newBranchOnlyContext(0, []uint{1, 2})
	c2 := newBranchOnlyContext(0, []uint{3})
	if tenantCacheKey(c1) == tenantCacheKey(c2) {
		t.Fatalf("cache key phai khac nhau giua 2 tap branchIds khac nhau")
	}
}

package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Story-41 (từ story-58 QA-C F3): `IsBranchAllowedInCompany` (0 test) bị xoá; helper chi nhánh CÒN
// LẠI là `IsBranchAllowed` (caller: import/customer importBranchAllowed, …). Neo 4 nhánh của nó
// trước khi xoá helper company để không ai "đơn giản hoá" nó thành luôn-true.
func TestS41IsBranchAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCtx := func(ids []uint) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		if ids != nil {
			c.Set("branchIds", ids)
		}
		return c
	}
	cases := []struct {
		name   string
		ids    []uint
		branch uint
		want   bool
	}{
		{"branchID = 0 ⇒ từ chối", []uint{10}, 0, false},
		{"branchID = 0 kể cả user không giới hạn ⇒ từ chối", nil, 0, false},
		{"trong quyền ⇒ cho phép", []uint{10, 11}, 11, true},
		{"ngoài quyền ⇒ từ chối", []uint{10, 11}, 12, false},
		{"branchIds rỗng = không giới hạn ⇒ cho phép", nil, 99, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsBranchAllowed(newCtx(tc.ids), tc.branch); got != tc.want {
				t.Errorf("IsBranchAllowed(ids=%v, %d) = %v, muốn %v", tc.ids, tc.branch, got, tc.want)
			}
		})
	}
}

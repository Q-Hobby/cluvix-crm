package testhelpers

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// TenantCtx returns a *gin.Context with tenant and auth fields pre-set,
// mirroring what the auth middleware injects for real requests.
// Use this in handler-level tests that call tenant.ScopedDB or middleware helpers.
func TenantCtx(branchID, userID uint) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)

	// Keys used by pkg/tenant/scope.go and internal/middleware/auth.go. Story-41: session không còn
	// khoá thương hiệu.
	c.Set("branchActiveId", branchID)
	c.Set("branchIds", []uint{branchID})
	c.Set("userID", userID)

	return c
}

//go:build integration

package testhelpers

import (
	"github.com/gin-gonic/gin"
)

// AuthMiddleware returns a gin.HandlerFunc that injects tenant and auth keys into
// the context, mirroring what JWTAuth + AuthResponseHeaders does for real requests.
// Use this in handler HTTP tests to avoid needing a real JWT.
func AuthMiddleware(branchID, userID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("branchActiveId", branchID)
		c.Set("branchIds", []uint{branchID})
		c.Set("userID", userID)
		c.Next()
	}
}

// NewGin returns a gin.Engine in TestMode with the default recovery middleware.
// The caller is responsible for registering routes.
func NewGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

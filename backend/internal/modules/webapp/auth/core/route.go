package core

import (
	"erpCluvix/internal/modules/webapp/auth/core/handler"

	"github.com/gin-gonic/gin"
)

// New constructs the core auth Handler. Exposed so router/auth.go can wire
// the public endpoints (Login, RefreshToken, Logout) onto the unauth api group.
func New(d handler.Deps) *handler.Handler { return handler.New(d) }

func RegisterRoutes(authed gin.IRouter, h *handler.Handler) {
	// Logout KHÔNG đăng ký ở đây — nó là public route (xem auth/router.go):
	// phải consume được refresh cookie kể cả khi access token đã hết hạn.
	authed.POST("/auth/logout-all", h.LogoutAll)
	authed.POST("/auth/clear-cache", h.ClearCache)
	authed.GET("/auth/me", h.Me)
	authed.GET("/auth/permission", h.Permission)
}

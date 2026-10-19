// Package auth — domain entrypoint. 2 nhóm endpoint: public (api) + authed.
package auth

import (
	"erpCluvix/internal/deps"
	auditHandler "erpCluvix/internal/modules/webapp/auth/audit/handler"
	"erpCluvix/internal/modules/webapp/auth/core"
	coreHandler "erpCluvix/internal/modules/webapp/auth/core/handler"
	deviceHandler "erpCluvix/internal/modules/webapp/auth/device/handler"
	"erpCluvix/internal/modules/webapp/auth/rbac"
	rbacHandler "erpCluvix/internal/modules/webapp/auth/rbac/handler"
	twofaHandler "erpCluvix/internal/modules/webapp/auth/twofa/handler"
	twofaSvc "erpCluvix/internal/modules/webapp/auth/twofa/service"

	"github.com/gin-gonic/gin"
)

func Register(api *gin.RouterGroup, authed *gin.RouterGroup, d deps.Deps) {
	tfSvc := &twofaSvc.Service{DB: d.DB, Cache: d.Cache, Issuer: d.TotpIssuer}

	depsCore := coreHandler.Deps{
		DB:       d.DB,
		Cache:    d.Cache,
		TokenSvc: d.TokenSvc,
		RbacSvc:  d.RbacSvc,
		TwoFASvc: tfSvc,
		Prod:     d.Prod,
	}
	h := core.New(depsCore)

	// Public
	pub := api.Group("/auth")
	{
		pub.POST("/login", h.Login)
		pub.POST("/login/2fa", h.Login2FA) // bước 2 sau khi login trả requires_2fa
		pub.POST("/refresh-token", h.RefreshToken)
		// Logout public: consume refresh cookie được cả khi access token hết hạn
		// (handler tự guard CSRF qua Sec-Fetch-Site + revoke bearer best-effort).
		pub.POST("/logout", h.Logout)
	}

	// Authenticated
	core.RegisterRoutes(authed, h)
	rbac.RegisterRoutes(authed, rbacHandler.Deps{
		DB:       d.DB,
		Cache:    d.Cache,
		TokenSvc: d.TokenSvc,
		RbacSvc:  d.RbacSvc,
	})

	// Device management (self-service — chỉ thiết bị của chính user)
	dev := deviceHandler.New(deviceHandler.Deps{DB: d.DB, TokenSvc: d.TokenSvc})
	authed.GET("/auth/devices", dev.List)
	authed.POST("/auth/devices/:id/revoke", dev.Revoke)

	// Security audit log (super-user)
	audit := auditHandler.New(auditHandler.Deps{DB: d.DB, RbacSvc: d.RbacSvc})
	authed.GET("/auth/audit-log", audit.List)

	// 2FA (TOTP) self-service + admin reset (dùng lại tfSvc đã tạo ở trên)
	tf := twofaHandler.New(twofaHandler.Deps{DB: d.DB, Svc: tfSvc, RbacSvc: d.RbacSvc})
	authed.GET("/auth/2fa/status", tf.Status)
	authed.POST("/auth/2fa/setup", tf.Setup)
	authed.POST("/auth/2fa/verify-setup", tf.VerifySetup)
	authed.POST("/auth/2fa/disable", tf.Disable)
	authed.POST("/auth/2fa/reset/:userId", tf.Reset)
}

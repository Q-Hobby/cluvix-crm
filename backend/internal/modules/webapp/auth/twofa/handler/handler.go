package handler

import (
	"net/http"
	"strconv"

	"erpCluvix/internal/middleware"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	twofaSvc "erpCluvix/internal/modules/webapp/auth/twofa/service"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/seclog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	DB      *gorm.DB
	Svc     *twofaSvc.Service
	RbacSvc *authSvc.RbacService
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// Status GET /auth/2fa/status — FE profile hiển thị đã bật chưa.
func (h *Handler) Status(c *gin.Context) {
	userID := middleware.GetUserID(c)
	c.JSON(http.StatusOK, gin.H{"enabled": h.Svc.IsEnabled(userID)})
}

// Setup POST /auth/2fa/setup — sinh secret pending, trả QR URI + secret.
func (h *Handler) Setup(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "token_invalid")})
		return
	}
	var username string
	h.DB.Table("user").Select("username").Where("id = ?", userID).Scan(&username)

	uri, secret, err := h.Svc.Setup(userID, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "server_error")})
		return
	}
	c.JSON(http.StatusOK, gin.H{"uri": uri, "secret": secret})
}

// VerifySetup POST /auth/2fa/verify-setup {code} — bật 2FA, trả backup codes 1 lần.
func (h *Handler) VerifySetup(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "required")})
		return
	}
	codes, err := h.Svc.VerifySetup(userID, req.Code)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "twofa_invalid_code")})
		return
	}
	seclog.Event(c, "2fa_enrolled")
	// user_scope cache giữ Totp state → xóa để enforce cập nhật ngay.
	middleware.InvalidateUserScope(h.Svc.Cache, userID)
	c.JSON(http.StatusOK, gin.H{"backup_codes": codes})
}

// Disable POST /auth/2fa/disable {code}.
func (h *Handler) Disable(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "required")})
		return
	}
	if err := h.Svc.Disable(userID, req.Code); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "twofa_invalid_code")})
		return
	}
	seclog.Event(c, "2fa_disabled")
	middleware.InvalidateUserScope(h.Svc.Cache, userID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": i18n.T(c, "updated")})
}

// Reset POST /auth/2fa/reset/:userId — super-user gỡ 2FA cho user (mất thiết bị).
func (h *Handler) Reset(c *gin.Context) {
	actor := middleware.GetUserID(c)
	if actor == 0 || h.RbacSvc == nil || !h.RbacSvc.IsSuperUser(actor) {
		c.JSON(http.StatusForbidden, gin.H{"message": i18n.T(c, "forbidden")})
		return
	}
	targetID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil || targetID == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "required")})
		return
	}
	if err := h.Svc.Reset(uint(targetID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "server_error")})
		return
	}
	c.Set("userID", uint(targetID)) // seclog gắn user bị reset
	seclog.Event(c, "2fa_reset")
	middleware.InvalidateUserScope(h.Svc.Cache, uint(targetID))
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": i18n.T(c, "updated")})
}

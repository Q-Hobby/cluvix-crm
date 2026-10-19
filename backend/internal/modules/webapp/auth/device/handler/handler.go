package handler

import (
	"net/http"
	"strconv"

	"erpCluvix/internal/middleware"
	devdao "erpCluvix/internal/modules/webapp/auth/device/dao"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/seclog"
	"erpCluvix/pkg/token"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	DB       *gorm.DB
	TokenSvc *token.Service
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) dao() *devdao.Dao { return &devdao.Dao{DB: h.DB} }

// List trả thiết bị của chính user, đánh dấu thiết bị nào đang có refresh session sống.
func (h *Handler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "token_invalid")})
		return
	}

	devices, err := h.dao().ListByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "server_error")})
		return
	}

	active := h.TokenSvc.ActiveFingerprints(userID)
	out := make([]gin.H, 0, len(devices))
	for _, d := range devices {
		out = append(out, gin.H{
			"id":          d.ID,
			"device_name": d.DeviceName,
			"last_ip":     d.LastIP,
			"last_seen":   d.LastSeen,
			"created_at":  d.CreatedAt,
			"is_active":   active[d.Fingerprint], // đang có refresh token sống
		})
	}
	c.JSON(http.StatusOK, out)
}

// Revoke đăng xuất 1 thiết bị của chính user: xoá refresh token + set status revoked.
func (h *Handler) Revoke(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "token_invalid")})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "required")})
		return
	}

	// GetOwned chặn thao tác lên thiết bị của user khác (scope user-level).
	dev, err := h.dao().GetOwned(uint(id), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": i18n.T(c, "data_not_found")})
		return
	}

	h.TokenSvc.RevokeRefreshByFingerprint(userID, dev.Fingerprint)
	if err := h.dao().MarkRevoked(dev.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "server_error")})
		return
	}
	seclog.Event(c, "device_revoked", seclog.FP(dev.Fingerprint))

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": i18n.T(c, "logout_success")})
}

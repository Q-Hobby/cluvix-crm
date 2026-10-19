package user_profile

import (
	"erpCluvix/internal/modules/webapp/config/user_profile/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/user-profile/get-user-profile", h.GetUserProfileSelf)
	cfg.GET("/user-profile/get-overview-user", h.GetOverviewUser)
	cfg.GET("/user-profile/:id", h.GetUserProfile)
	cfg.POST("/user-profile/switch-profile", h.SwitchProfile)
	cfg.POST("/user-profile/update-user-profile", h.UpdateUserProfile)
	cfg.POST("/user-profile/create-avatar-user", h.CreateAvatarUser)
}

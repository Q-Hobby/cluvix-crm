package hr

import (
	"erpCluvix/internal/deps"
	"erpCluvix/internal/modules/webapp/hr/user"
	userHandler "erpCluvix/internal/modules/webapp/hr/user/handler"

	"github.com/gin-gonic/gin"
)

func Register(authed *gin.RouterGroup, d deps.Deps) {
	g := authed.Group("/hr")
	user.RegisterRoutes(g, userHandler.Deps{DB: d.DB, UserSvc: d.UserSvc, RbacSvc: d.RbacSvc, DownloadURL: d.DownloadURL})
}

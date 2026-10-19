package user

import (
	"erpCluvix/internal/modules/webapp/hr/user/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /hr/user/* endpoints.
func RegisterRoutes(hr gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	hr.GET("/user", h.ListUser)
	hr.GET("/user/get-all-users", h.GetAllUsers)
	hr.GET("/user/:id", h.GetUser)
	hr.POST("/user", h.CreateUser)
	hr.PUT("/user/:id", h.UpdateUser)
	hr.POST("/user/change-password", h.ChangePassword)
	hr.POST("/user/update-password", h.UpdatePassword)
	hr.PUT("/user/reset-password", h.ResetPassword)
	hr.POST("/user/import-users", h.ImportUsers)
	hr.DELETE("/user/:id", h.DeleteUser)
}

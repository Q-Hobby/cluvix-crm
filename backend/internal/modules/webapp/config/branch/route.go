package branch

import (
	"erpCluvix/internal/modules/webapp/config/branch/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(cfg gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	cfg.GET("/branch", h.ListBranch)
	cfg.GET("/branch/:id", h.GetBranch)
	cfg.POST("/branch", h.CreateBranch)
	cfg.POST("/branch/:id", h.UpdateBranch)
	cfg.PUT("/branch/:id", h.UpdateBranch)
	cfg.POST("/branch/status-open-appointment", h.ToggleBranchOpenAppointment)
	cfg.DELETE("/branch/:id", h.DeleteBranch)
}

// Package dashboard_layout — bố cục dashboard CRM tuỳ biến (port từ ERP epic crm-dashboard-customizable).
package dashboard_layout

import (
	"erpCluvix/internal/modules/webapp/crm/dashboard_layout/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes mounts /crm-dashboard/layout endpoints dưới nhóm /crm.
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	g := crm.Group("/crm-dashboard/layout")
	g.GET("", h.Get)
	g.PUT("", h.Save)
	g.DELETE("", h.Reset)
	g.PUT("/brand-default", h.SaveBrandDefault)
	g.DELETE("/brand-default", h.DeleteBrandDefault)
}

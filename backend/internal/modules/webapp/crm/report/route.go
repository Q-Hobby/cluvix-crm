// Package report — CRM report endpoints (Plan B.x 2026-05-16): consolidated
// from old internal/handlers/report/. Wires /crm/report/* group + the
// standalone /crm/order-service-used/report-service-used endpoint.
package report

import (
	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/report/handler"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	// Export báo cáo (`?_export=1`) chặn thật theo manageCrmReport — catalog không có ext riêng cho
	// báo cáo; FE gác nút Export các màn báo cáo cùng quyền này (S-7).
	exportGuard := middleware.RequireExportPerm(d.RbacSvc, define.PermManageCrmReport)
	reportGrp := crm.Group("/report", exportGuard)
	reportGrp.GET("/appointment-statistics", h.AppointmentStatistics)
	reportGrp.GET("/branch-statistics", h.BranchStatistics)
	reportGrp.GET("/source-statistics", h.SourceStatistics)
	reportGrp.GET("/group-source-statistics", h.GroupSourceStatistics)
	reportGrp.GET("/province-statistics", h.ProvinceStatistics)
	reportGrp.GET("/country-statistics", h.CountryStatistics)
	reportGrp.GET("/category-statistics", h.CategoryStatistics)
	reportGrp.GET("/interested-statistics", h.InterestedStatistics)
	reportGrp.GET("/service-type-statistics", h.ServiceTypeStatistics)
	reportGrp.GET("/revenue", h.Revenue)
	reportGrp.GET("/telesales-online", h.TelesalesOnline)
	reportGrp.GET("/effective-advise", h.EffectiveAdvise)
	reportGrp.GET("/branch-appointment", h.BranchAppointment)
	reportGrp.GET("/appointment-consultation", h.AppointmentConsultationReport)

	// Standalone endpoint outside /report group
	crm.GET("/order-service-used/report-service-used", exportGuard, h.ReportServiceUsed)
}

// Package revenue — nhóm báo cáo DOANH THU của CRM (/crm/report/revenue/*), port từ ERP
// accountant (payment-by-date, payment-by-method, payment-branch/list, list-refund-deposit-service).
package revenue

import (
	"github.com/gin-gonic/gin"

	"erpCluvix/internal/deps"
	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/report/revenue/handler"
	"erpCluvix/pkg/define"
)

// Register gắn /report/revenue/* vào group `rg` = group `/crm` (cùng group crm/router.go truyền cho
// crmReport.RegisterRoutes). ⚠ `/crm/report/revenue` (không đuôi) là endpoint KHÁC (báo cáo doanh thu
// marketing của crm/report/handler) — các route ở đây đều có đuôi nên không đụng nhau.
func Register(rg *gin.RouterGroup, d *deps.Deps) {
	h := handler.New(handler.Deps{DB: d.DB})

	// Export (`?_export=1` và endpoint `/export` riêng) chặn thật theo manageCrmReport (S-7).
	g := rg.Group("/report/revenue", middleware.RequireExportPerm(d.RbacSvc, define.PermManageCrmReport))
	g.GET("/by-date", h.ReportPaymentByDate)
	g.GET("/by-date/export", middleware.RequirePermStrict(d.RbacSvc, define.PermManageCrmReport), h.ReportPaymentByDateExport)
	g.GET("/by-method", h.ReportPaymentByMethod)
	g.GET("/branch-receipts", h.ListBranchReceipts)
	g.GET("/refund-deposit-service", h.ListRefundDepositService)
}

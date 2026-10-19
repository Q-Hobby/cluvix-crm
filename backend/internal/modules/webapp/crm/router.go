package crm

import (
	"erpCluvix/internal/deps"
	configIndexeddb "erpCluvix/internal/modules/webapp/config/indexeddb"
	configIndexeddbHandler "erpCluvix/internal/modules/webapp/config/indexeddb/handler"
	"erpCluvix/internal/modules/webapp/crm/appointment"
	appointmentHandler "erpCluvix/internal/modules/webapp/crm/appointment/handler"
	"erpCluvix/internal/modules/webapp/crm/attachment"
	attachmentHandler "erpCluvix/internal/modules/webapp/crm/attachment/handler"
	"erpCluvix/internal/modules/webapp/crm/customer"
	customerHandler "erpCluvix/internal/modules/webapp/crm/customer/handler"
	"erpCluvix/internal/modules/webapp/crm/customer_detail"
	customerDetailHandler "erpCluvix/internal/modules/webapp/crm/customer_detail/handler"
	crmDashboard "erpCluvix/internal/modules/webapp/crm/dashboard"
	crmDashboardHandler "erpCluvix/internal/modules/webapp/crm/dashboard/handler"
	crmDashboardLayout "erpCluvix/internal/modules/webapp/crm/dashboard_layout"
	crmDashboardLayoutHandler "erpCluvix/internal/modules/webapp/crm/dashboard_layout/handler"
	"erpCluvix/internal/modules/webapp/crm/exchange_rate"
	exchangeRateHandler "erpCluvix/internal/modules/webapp/crm/exchange_rate/handler"
	"erpCluvix/internal/modules/webapp/crm/note"
	noteHandler "erpCluvix/internal/modules/webapp/crm/note/handler"
	"erpCluvix/internal/modules/webapp/crm/order_plan"
	otpHandler "erpCluvix/internal/modules/webapp/crm/order_plan/handler"
	"erpCluvix/internal/modules/webapp/crm/order_service_used"
	osuHandler "erpCluvix/internal/modules/webapp/crm/order_service_used/handler"
	"erpCluvix/internal/modules/webapp/crm/payment"
	paymentHandler "erpCluvix/internal/modules/webapp/crm/payment/handler"
	"erpCluvix/internal/modules/webapp/crm/reminder"
	reminderHandler "erpCluvix/internal/modules/webapp/crm/reminder/handler"
	crmReport "erpCluvix/internal/modules/webapp/crm/report"
	crmReportDebt "erpCluvix/internal/modules/webapp/crm/report/debt"
	crmReportHandler "erpCluvix/internal/modules/webapp/crm/report/handler"
	crmReportRevenue "erpCluvix/internal/modules/webapp/crm/report/revenue"
	crmReportStaff "erpCluvix/internal/modules/webapp/crm/report/staff"
	"erpCluvix/internal/modules/webapp/crm/tinymce"
	tinymceHandler "erpCluvix/internal/modules/webapp/crm/tinymce/handler"

	"github.com/gin-gonic/gin"
)

func Register(authed *gin.RouterGroup, d deps.Deps) {
	db := d.DB
	dash := crmDashboard.New(crmDashboardHandler.Deps{DB: db, Cache: d.Cache})

	// IndexedDB sync — URL giữ /crm/indexeddb/sync nhưng module ở config.
	configIndexeddb.RegisterRoutes(authed, configIndexeddbHandler.Deps{DB: db})

	g := authed.Group("/crm")
	customer.RegisterRoutes(g, customerHandler.Deps{DB: db, Store: d.Store, RbacSvc: d.RbacSvc})
	attachment.RegisterRoutes(g, attachmentHandler.Deps{DB: db, PrivateStore: d.PrivateStore, PublicStore: d.Store, SignedURLTTL: d.SignedURLTTL})
	appointment.RegisterRoutes(g, appointmentHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
	order_plan.RegisterRoutes(g, otpHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
	payment.RegisterRoutes(g, paymentHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
	order_service_used.RegisterRoutes(g, osuHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
	customer_detail.RegisterRoutes(g, customerDetailHandler.Deps{DB: db})
	crmReport.RegisterRoutes(g, crmReportHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
	crmReportRevenue.Register(g, &d)
	crmReportDebt.Register(g, &d)
	crmReportStaff.Register(g, &d)
	reminder.RegisterRoutes(g, reminderHandler.Deps{DB: db})
	note.RegisterRoutes(g, noteHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
	exchange_rate.RegisterRoutes(g, exchangeRateHandler.Deps{})
	tinymce.RegisterRoutes(g, tinymceHandler.Deps{Store: d.Store})

	// 1 dash method reused under /report (inline — not part of crmReport).
	g.GET("/report/compare-appointment", dash.CrmCompareAppointment)

	// CRM Dashboard
	crmDashboard.RegisterRoutes(g, dash)
	// Bố cục dashboard tuỳ biến (user / mặc định thương hiệu).
	crmDashboardLayout.RegisterRoutes(g, crmDashboardLayoutHandler.Deps{DB: db, RbacSvc: d.RbacSvc})
}

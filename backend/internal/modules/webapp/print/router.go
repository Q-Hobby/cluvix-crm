package print

import (
	"erpCluvix/internal/deps"
	"erpCluvix/internal/middleware"
	exportPkg "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/internal/modules/webapp/print/appointment"
	appointmentHandler "erpCluvix/internal/modules/webapp/print/appointment/handler"
	"erpCluvix/internal/modules/webapp/print/order_plan"
	orderPlanHandler "erpCluvix/internal/modules/webapp/print/order_plan/handler"
	"erpCluvix/internal/modules/webapp/print/payment"
	paymentHandler "erpCluvix/internal/modules/webapp/print/payment/handler"
	"erpCluvix/internal/modules/webapp/print/payment_in_day"
	paymentInDayHandler "erpCluvix/internal/modules/webapp/print/payment_in_day/handler"
	"erpCluvix/internal/modules/webapp/print/reception_payment"
	receptionPaymentHandler "erpCluvix/internal/modules/webapp/print/reception_payment/handler"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

func Register(authed *gin.RouterGroup, d deps.Deps) {
	g := authed.Group("/print")
	order_plan.RegisterRoutes(g, orderPlanHandler.Deps{DB: d.DB})
	payment.RegisterRoutes(g, paymentHandler.Deps{DB: d.DB})
	reception_payment.RegisterRoutes(g, receptionPaymentHandler.Deps{DB: d.DB})
	appointment.RegisterRoutes(g, appointmentHandler.Deps{DB: d.DB})
	payment_in_day.RegisterRoutes(g, paymentInDayHandler.Deps{DB: d.DB})

	// Export Excel — free funcs từ pkg export, wire trực tiếp dưới /print/.
	db := d.DB
	// Endpoint chỉ để xuất Excel lịch hẹn theo giờ (kèm SĐT khách) ⇒ cần extAppointment, chặn thật (S-7).
	g.GET("/appointment-byhour-list-export", middleware.RequirePermStrict(d.RbacSvc, define.PermExtAppointment), func(c *gin.Context) {
		exportPkg.ExportAppointmentByhourList(c, db)
	})
}

// Package appointment — module root.
//
// Plan B.2 (2026-05-16): Full layered (handler + service + repo).
// Note: GET /crm/appointment/get-by-customer wired SEPARATELY ở router/crm.go
// vì handler GetAppointmentByCustomer ở customer_detail.go (vẫn trong crm package).
package appointment

import (
	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/appointment/handler"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers /crm/appointment/* EXCEPT /get-by-customer.
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	appt := crm.Group("/appointment")
	{
		// `?_export=1` = file Excel danh sách lịch hẹn (kèm SĐT khách) ⇒ cần extAppointment (S-7).
		appt.GET("", middleware.RequireExportPerm(d.RbacSvc, define.PermExtAppointment), h.ListAppointment)
		appt.GET("/appointment-schedule", h.AppointmentSchedule)
		appt.GET("/appointment-schedule-byhour", h.AppointmentScheduleByHour)
		appt.GET("/appointment-schedule-byhour-list", h.AppointmentScheduleByHourList)
		appt.GET("/appointment-schedule-byhour-grouped", h.AppointmentScheduleByHourGrouped)
		appt.GET("/appointment-overview", h.AppointmentOverview)
		appt.GET("/get-appointment-chart-status", h.GetAppointmentChartStatus)
		appt.GET("/appointment-date-in-day", h.AppointmentDateInDay)
		appt.GET("/:id", h.GetAppointment)
		appt.POST("", h.CreateAppointment)
		appt.POST("/customer-checkin", func(c *gin.Context) {
			c.Params = append(c.Params, gin.Param{Key: "id", Value: c.Query("id")})
			h.CustomerCheckin(c)
		})
		appt.PUT("/:id/update-appointment", h.UpdateAppointment)
		appt.PUT("/:id/customer-checkin", h.CustomerCheckin)
		appt.POST("/:id/add-note", h.AddNote)
		appt.PUT("/:id/update-appointment-cancel", h.UpdateAppointmentCancel)
		appt.PUT("/:id/update-confirm-appointment", h.UpdateConfirmAppointment)
		appt.DELETE("/:id", h.DeleteAppointment)
	}
}

// Package reminder — module root.
//
// Plan B.2 (2026-05-16): Migrate reminder CRUD ra layered module.
package reminder

import (
	"erpCluvix/internal/modules/webapp/crm/reminder/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes gắn endpoint /crm/reminder/* vào parent group.
// reminder-of-customer (static) register TRƯỚC /:id để gin không capture.
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	r := crm.Group("/reminder")
	{
		r.GET("", h.ListReminder)
		r.GET("/reminder-of-customer", h.ReminderOfCustomer)
		r.GET("/:id", h.GetReminder)
		r.POST("", h.CreateReminder)
		r.PUT("/:id", h.UpdateReminder)
		r.DELETE("/:id", h.DeleteReminder)
	}
}

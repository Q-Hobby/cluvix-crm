// Package customer_detail — module root, cross-cutting view aggregator.
//
// Plan B.2 (2026-05-16) — last CRM migration: endpoint rải rác qua nhiều
// path khác nhau (appointment, payment, order-service-used,
// customer/:id; customer-care gỡ ở crm-standalone story-11). Module này KHÔNG phải single-resource — là
// "view aggregator layer".
package customer_detail

import (
	"erpCluvix/internal/modules/webapp/crm/customer_detail/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes — gắn 4 endpoint qua các route prefix khác nhau dưới crm group.
// Caller phải pass parent crm group đã wrap admin auth.
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	// Carve-out endpoints scatter qua nhiều parent group:
	crm.GET("/appointment/get-by-customer", h.GetAppointmentByCustomer)
	crm.GET("/payment/get-payment-detail-for-customer", h.GetPaymentDetailForCustomer)
	crm.GET("/order-service-used/list-service-used-customer", h.ListServiceUsedCustomer)
	crm.GET("/customer/:id/timeline-customer", h.TimelineCustomer)
}

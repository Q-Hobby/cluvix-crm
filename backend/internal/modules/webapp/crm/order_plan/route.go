// Package order_plan — module root, expose RegisterRoutes.
//
// Plan B.1 (2026-05-16): Migrate handler ra layered module. Service vẫn ở
// internal/services/crm/ vì cross-file dependency (loyalty, clv_trigger,
// order_service_used_validate) — surgical compromise.
package order_plan

import (
	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/order_plan/handler"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes gắn route group /order-plan (story-49: gộp nhóm URL cũ của đơn hàng vào đây).
// Caller (router/crm.go) phải pass parent group đã wrap admin auth.
//
// GET "" = danh sách + export (`?_export=1`) cho màn CRM › Quản lý đơn hàng — port từ ERP
// GET /accountant/order-plan (CRM không có module accountant). Route tĩnh đăng ký TRƯỚC `/:id`.
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	op := crm.Group("/order-plan")
	{
		// `?_export=1` chở trọn đơn hàng + doanh thu/công nợ + SĐT khách. Catalog không có extPlanOrder ⇒
		// dùng extPayment (màn port từ nhóm kế toán cùng các màn phiếu thu), chặn thật (S-7).
		op.GET("", middleware.RequireExportPerm(d.RbacSvc, define.PermExtPayment), h.ListOrderPlan)
		op.GET("/:id", h.GetOrderPlan)
		// Endpoint riêng cho màn CHI TIẾT ĐƠN (SlideOver) — xem docblock GetOrderPlanDetail.
		op.GET("/:id/detail", h.GetOrderPlanDetail)
		op.POST("", h.CreateOrderPlan)
		op.POST("/create-order", h.CreateOrderPlan)
		op.PUT("/:id", h.UpdateOrderPlan)
		op.PUT("/:id/update-order", h.UpdateOrderPlan)
		op.PUT("/service-used/:id/change-service", h.ChangeServiceForOSU)
		op.GET("/:id/print-order-plan", func(c *gin.Context) {
			c.Redirect(301, "/api/print/order-plan/"+c.Param("id"))
		})
		op.DELETE("/:id", h.DeleteOrderPlan)
	}
}

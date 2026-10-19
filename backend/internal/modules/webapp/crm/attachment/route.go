// Package attachment — module root.
package attachment

import (
	"erpCluvix/internal/modules/webapp/crm/attachment/handler"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes gắn các endpoint attachment vào CRM group.
//
// Routes:
//
//	GET    /crm/customer/:id/attachments  — list tất cả file của 1 khách hàng
//	POST   /crm/customer/:id/attachments  — upload file gắn thẳng vào customer
//	POST   /crm/attachment                — upload file với ref_type/ref_id tùy ý
//	GET    /crm/attachment/:id/download   — tải file (kho private, stream qua backend có auth)
//	POST   /crm/attachment/:id/signed-url — URL ký ngắn hạn xem/phát/tải trực tiếp (không cần JWT)
//	DELETE /crm/attachment/:id            — xóa file
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
	h := handler.New(d)

	crm.GET("/customer/:id/attachments", h.ListByCustomer)
	crm.POST("/customer/:id/attachments", h.UploadForCustomer)

	att := crm.Group("/attachment")
	{
		att.POST("", h.Upload)
		att.GET("/:id/download", h.Download)
		att.POST("/:id/signed-url", h.SignedURL)
		att.DELETE("/:id", h.Delete)
	}
}

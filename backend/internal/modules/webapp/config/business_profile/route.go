// Package business_profile — route `GET/PUT /api/config/business-profile` (story-63).
package business_profile

import (
	"erpCluvix/internal/modules/webapp/config/business_profile/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(authed gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	g := authed.Group("/business-profile")
	g.GET("", h.GetBusinessProfile)
	g.PUT("", h.UpdateBusinessProfile)
}

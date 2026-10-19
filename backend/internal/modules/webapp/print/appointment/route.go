package appointment

import (
	"erpCluvix/internal/modules/webapp/print/appointment/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(p gin.IRouter, d handler.Deps) {
	h := handler.New(d)
	p.GET("/appointment-byhour-list", h.PrintAppointmentByhourList)
}

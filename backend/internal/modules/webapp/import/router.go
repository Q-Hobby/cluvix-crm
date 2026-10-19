package importmodule

import (
	"erpCluvix/internal/deps"
	"erpCluvix/internal/modules/webapp/import/customer"
	customerHandler "erpCluvix/internal/modules/webapp/import/customer/handler"

	"github.com/gin-gonic/gin"
)

func Register(authed *gin.RouterGroup, d deps.Deps) {
	g := authed.Group("/import")

	customer.RegisterRoutes(g, customerHandler.Deps{DB: d.DB})
}

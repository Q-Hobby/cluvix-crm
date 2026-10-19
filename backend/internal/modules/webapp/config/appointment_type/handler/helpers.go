package handler

import (
	"strings"

	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

func requireName(c *gin.Context, name string) bool {
	if strings.TrimSpace(name) == "" {
		response.Validate(c, i18n.T(c, "name_required"))
		return false
	}
	return true
}

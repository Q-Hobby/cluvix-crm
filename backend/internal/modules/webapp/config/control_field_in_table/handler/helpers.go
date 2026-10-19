package handler

import (
	"strings"

	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/validation"

	"github.com/gin-gonic/gin"
)

//nolint:unused
func requireName(c *gin.Context, name string) bool {
	if strings.TrimSpace(name) == "" {
		response.Validate(c, i18n.T(c, "name_required"))
		return false
	}
	return true
}

//nolint:unused
func requireFields(c *gin.Context, pairs [][2]any) bool {
	v := validation.New()
	for _, p := range pairs {
		v.Required(p[0], strFromAny(p[1]))
	}
	if v.HasErrors() {
		response.Validate(c, v.Error())
		return false
	}
	return true
}

//nolint:unused
func strFromAny(a any) string {
	if s, ok := a.(string); ok {
		return s
	}
	return ""
}

//nolint:unused
func parseUint(s string) uint {
	var id uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + uint(c-'0')
		}
	}
	return id
}

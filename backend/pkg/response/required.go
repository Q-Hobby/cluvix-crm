package response

import (
	"strings"

	"erpCluvix/pkg/validation"

	"github.com/gin-gonic/gin"
)

// RequireName: shared cho config master "name only" tables.
// Trả false + đã response nếu name rỗng. Caller chỉ cần `if !response.RequireName(c, item.Name) { return }`.
func RequireName(c *gin.Context, name string) bool {
	if strings.TrimSpace(name) == "" {
		Validate(c, "Tên không được để trống")
		return false
	}
	return true
}

// RequireFields: helper cho handler có nhiều required field. Caller pass cặp (value, label).
//
//	if !response.RequireFields(c, [][2]any{{item.Name, "Tên"}, {item.Code, "Mã"}}) { return }
func RequireFields(c *gin.Context, pairs [][2]any) bool {
	v := validation.New()
	for _, p := range pairs {
		label, _ := p[1].(string)
		v.Required(p[0], label)
	}
	if v.HasErrors() {
		Validate(c, v.Error())
		return false
	}
	return true
}

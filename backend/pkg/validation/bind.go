package validation

import (
	"errors"
	"sync"

	"erpCluvix/pkg/phone"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Bind parses JSON body vào dto + chạy go-playground/validator. Trả error
// nguyên gốc để caller route qua response handler (parse error → 422 với
// field name, validator.ValidationErrors → 422 với i18n message).
//
// Cách dùng chuẩn:
//
//	var input UpdateLeadFormInput
//	if err := validation.Bind(c, &input); err != nil {
//	    response.BindOrValidate(c, err)
//	    return
//	}
//
// So với `c.ShouldBindJSON` trực tiếp: same behavior, nhưng centralized để
// tương lai thêm logic (vd auto-trim string, sanitize HTML) chỉ sửa 1 chỗ.
func Bind(c *gin.Context, dto any) error {
	return c.ShouldBindJSON(dto)
}

// BindQuery — same cho query string (GET endpoints).
func BindQuery(c *gin.Context, dto any) error {
	return c.ShouldBindQuery(dto)
}

// IsValidationError reports whether err là validator.ValidationErrors
// (tag-fail) khác với parse error (cannot unmarshal). Dùng để route response.
func IsValidationError(err error) bool {
	var ve validator.ValidationErrors
	return errors.As(err, &ve)
}

// ── Custom tags ──

var registerOnce sync.Once

// RegisterCustomTags đăng ký các tag custom vào Gin binding validator.
// Gọi 1 lần lúc startup (idempotent qua sync.Once).
func RegisterCustomTags() {
	registerOnce.Do(func() {
		v, ok := binding.Validator.Engine().(*validator.Validate)
		if !ok {
			return
		}
		_ = v.RegisterValidation("phone_vn", func(fl validator.FieldLevel) bool {
			s, ok := fl.Field().Interface().(string)
			if !ok {
				return false
			}
			return phone.IsValidMobile(s)
		})
	})
}

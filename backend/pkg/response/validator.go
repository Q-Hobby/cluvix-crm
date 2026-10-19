package response

import (
	"erpCluvix/pkg/i18n"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// TranslateValidationErrors converts validator errors using i18n
func TranslateValidationErrors(c *gin.Context, err error) string {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	var messages []string
	for _, e := range errs {
		field := i18n.FieldLabel(c, e.Field())
		switch e.Tag() {
		case "required":
			messages = append(messages, i18n.TF(c, "required", field))
		case "min":
			messages = append(messages, i18n.TF(c, "min", field, e.Param()))
		case "max":
			messages = append(messages, i18n.TF(c, "max", field, e.Param()))
		case "email":
			messages = append(messages, i18n.TF(c, "email", field))
		case "oneof":
			messages = append(messages, i18n.TF(c, "oneof", field, e.Param()))
		case "alphanumunicode", "alphanum":
			messages = append(messages, i18n.TF(c, "alphanum", field))
		case "numeric":
			messages = append(messages, i18n.TF(c, "numeric", field))
		case "gte":
			messages = append(messages, i18n.TF(c, "gte", field, e.Param()))
		case "lte":
			messages = append(messages, i18n.TF(c, "lte", field, e.Param()))
		default:
			messages = append(messages, i18n.TF(c, "invalid", field, e.Tag()))
		}
	}

	result := ""
	for i, m := range messages {
		if i > 0 {
			result += "; "
		}
		result += m
	}
	return result
}

package i18n

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	Vi = "vi"
	En = "en"
)

var messages = map[string]map[string]string{
	Vi: vi,
	En: en,
}

// T translates a message key based on user locale
func T(c *gin.Context, key string) string {
	locale := GetLocale(c)
	if msg := lookup(locale, key); msg != "" {
		return msg
	}
	if msg := lookup(Vi, key); msg != "" {
		return msg
	}
	return key
}

// TF translates with format args
func TF(c *gin.Context, key string, args ...interface{}) string {
	locale := GetLocale(c)
	format := key
	if msg := lookup(locale, key); msg != "" {
		format = msg
	} else if msg := lookup(Vi, key); msg != "" {
		format = msg
	}
	if len(args) > 0 {
		return fmt.Sprintf(format, args...)
	}
	return format
}

// TL translates by explicit locale — dùng cho ngữ cảnh KHÔNG có gin context
// (goroutine nền, log). Fallback vi rồi trả key như T.
func TL(locale, key string) string {
	if msg := lookup(normalizeLocale(locale), key); msg != "" {
		return msg
	}
	if msg := lookup(Vi, key); msg != "" {
		return msg
	}
	return key
}

// TLF — TL với format args (mirror TF).
func TLF(locale, key string, args ...interface{}) string {
	format := TL(locale, key)
	if len(args) > 0 {
		return fmt.Sprintf(format, args...)
	}
	return format
}

// TryT looks up a key; returns translated string if found, empty string if not.
func TryT(c *gin.Context, key string) string {
	locale := GetLocale(c)
	if msg := lookup(locale, key); msg != "" {
		return msg
	}
	if msg := lookup(Vi, key); msg != "" {
		return msg
	}
	return ""
}

// FieldLabel returns translated field label
func FieldLabel(c *gin.Context, field string) string {
	key := "field." + strings.ToLower(field)
	label := T(c, key)
	if label == key {
		return field
	}
	return label
}

// GetLocale extracts locale from gin context
func GetLocale(c *gin.Context) string {
	if locale, exists := c.Get("locale"); exists {
		if l, ok := locale.(string); ok && l != "" {
			return normalizeLocale(l)
		}
	}
	if lang := c.GetHeader("Accept-Language"); lang != "" {
		return normalizeLocale(lang)
	}
	return Vi
}

// AddMessages adds custom messages at runtime
func AddMessages(locale string, msgs map[string]string) {
	if _, ok := messages[locale]; !ok {
		messages[locale] = make(map[string]string)
	}
	for k, v := range msgs {
		messages[locale][k] = v
	}
}

func lookup(locale, key string) string {
	if msgs, ok := messages[locale]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	return ""
}

func normalizeLocale(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "en") {
		return En
	}
	// locale lạ (kể cả ja/jp/fil đã bỏ D5) rơi về mặc định vi.
	return Vi
}

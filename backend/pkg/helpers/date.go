package helpers

import (
	"regexp"
	"strings"
	"time"
)

// dateOnlyRegex match đầu chuỗi có dạng YYYY-MM-DD.
var dateOnlyRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})`)

// ParseDateOnly chuẩn hóa chuỗi datetime/ISO 8601 về dạng "YYYY-MM-DD" và xác
// nhận đó là một ngày thật tồn tại (round-trip qua time.Parse) — KHÔNG chỉ
// đúng hình dạng regex (vd "20213-03-20", "2026-02-30" phải bị từ chối).
//
// Hỗ trợ:
//   - "1986-08-12T00:00:00Z"        → "1986-08-12", true
//   - "1986-08-12T00:00:00+07:00"   → "1986-08-12", true
//   - "1986-08-12 00:00:00"         → "1986-08-12", true
//   - "1986-08-12"                  → "1986-08-12", true
//   - "12/08/1986" (dd/mm/yyyy)     → "1986-08-12", true
//   - ""/chuỗi không parse được     → "", false
func ParseDateOnly(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	// ISO 8601 hoặc datetime — lấy 10 ký tự đầu nếu match YYYY-MM-DD, rồi
	// verify đó là ngày thật (regex tự nó chấp nhận cả "9999-99-99").
	if m := dateOnlyRegex.FindString(s); m != "" {
		if t, err := time.Parse("2006-01-02", m); err == nil && t.Format("2006-01-02") == m {
			return m, true
		}
		return "", false
	}
	// Format dd/mm/yyyy phổ biến — parse rồi reformat.
	for _, layout := range []string{"02/01/2006", "2/1/2006", "02-01-2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), true
		}
	}
	return "", false
}

// NormalizeDateOnly chuẩn hóa chuỗi ngày về "YYYY-MM-DD" cho cột MySQL DATE.
// Chuỗi rỗng hoặc không phải ngày thật → "" (an toàn, coi như thiếu giá trị) —
// KHÔNG để chuỗi rác lọt xuống SQL gây MySQL Error 1292 (incident 2026-08-19:
// birthday "20213-03-20" giữ nguyên rồi crash raw UPDATE ở prod).
func NormalizeDateOnly(s string) string {
	v, ok := ParseDateOnly(s)
	if !ok {
		return ""
	}
	return v
}

// NormalizeDateOnlyPtr — convenience cho `*string` field.
// nil/empty → nil. Otherwise normalize và update in-place.
func NormalizeDateOnlyPtr(p **string) {
	if p == nil || *p == nil {
		return
	}
	v := NormalizeDateOnly(**p)
	if v == "" {
		*p = nil
		return
	}
	**p = v
}

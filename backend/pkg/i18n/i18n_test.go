package i18n

import "testing"

// D5 (story-22 AC6): chỉ còn vi + en; mọi locale khác (kể cả ja/jp/fil đã bỏ) rơi về Vi.
func TestNormalizeLocale(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ja", Vi},
		{"jp", Vi},
		{"ja-JP", Vi},
		{"fil", Vi},
		{"fil-PH", Vi},
		{"fr", Vi},
		{"", Vi},
		{"vi-VN", Vi},
		{"en", En},
		{"en-US", En},
		{" EN ", En},
	}
	for _, c := range cases {
		if got := normalizeLocale(c.in); got != c.want {
			t.Errorf("normalizeLocale(%q) = %q, muốn %q", c.in, got, c.want)
		}
	}
}

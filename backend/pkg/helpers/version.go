package helpers

import "strings"

// CompareVersions so sánh 2 version "a.b.c": -1 nếu a < b, 0 nếu bằng, 1 nếu a > b.
// Mỗi segment lấy phần số đứng đầu ("3+45" → 3, "2-beta" → 2); segment thiếu = 0 —
// an toàn với version Flutter dạng "1.2.3+45" (build metadata).
func CompareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		va, vb := 0, 0
		if i < len(pa) {
			va = leadingInt(pa[i])
		}
		if i < len(pb) {
			vb = leadingInt(pb[i])
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// leadingInt parse phần chữ số liên tục đứng đầu chuỗi; không có chữ số → 0.
func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	v := 0
	ok := false
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		v = v*10 + int(r-'0')
		ok = true
	}
	if !ok {
		return 0
	}
	return v
}

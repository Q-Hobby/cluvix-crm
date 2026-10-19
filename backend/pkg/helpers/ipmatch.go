package helpers

import (
	"encoding/json"
	"net"
	"strings"
)

// IPInAllowlist trả true nếu ip khớp bất kỳ entry nào trong list. Entry hỗ trợ:
//   - Exact IP: "203.0.113.5" (so cả dạng chuỗi lẫn parse để chuẩn hoá IPv6)
//   - CIDR: "203.0.113.0/24", "2001:db8::/32"
//
// List rỗng → false (caller tự quyết định "rỗng = không giới hạn").
func IPInAllowlist(ip string, list []string) bool {
	parsed := net.ParseIP(ip)
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && parsed != nil && cidr.Contains(parsed) {
				return true
			}
			continue
		}
		if entry == ip {
			return true
		}
		if ep := net.ParseIP(entry); ep != nil && parsed != nil && ep.Equal(parsed) {
			return true
		}
	}
	return false
}

// ValidateIPOrCIDRList kiểm `raw` là JSON mảng chuỗi, mỗi phần tử là IP hoặc CIDR hợp lệ (cùng
// hình dạng `parseIPList` ở `internal/middleware/response_headers.go` đọc để enforce allowlist).
// `raw` rỗng/NULL ⇒ hợp lệ (rỗng = không giới hạn IP). Trả false nếu JSON không phải mảng chuỗi,
// hoặc có phần tử không parse được thành IP/CIDR.
func ValidateIPOrCIDRList(raw []byte) bool {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return true
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return false
	}
	for _, entry := range list {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return false
		}
		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return false
			}
			continue
		}
		if net.ParseIP(entry) == nil {
			return false
		}
	}
	return true
}

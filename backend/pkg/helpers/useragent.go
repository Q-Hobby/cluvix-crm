package helpers

import "strings"

// DeviceNameFromUA rút gọn User-Agent thành tên thiết bị đọc được: "Chrome — macOS".
// Chỉ nhận diện browser/OS phổ biến — đủ cho danh sách "thiết bị đang đăng nhập",
// không nhằm thay fingerprint (thứ quyết định định danh thiết bị).
func DeviceNameFromUA(ua string) string {
	if ua == "" {
		return ""
	}

	browser := "Browser"
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}

	os := ""
	switch {
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	}

	if os == "" {
		return browser
	}
	return browser + " — " + os
}

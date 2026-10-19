// Package phone cung cấp validate và normalize số điện thoại Việt Nam.
// Đây là single source of truth — dùng thay cho regex rải rác khắp codebase.
//
// Hỗ trợ ba dạng đầu vào: "0912345678", "+84912345678", "84912345678".
// Tất cả output Normalize đều là dạng E.164 không có dấu + ("84XXXXXXXXX")
// — đúng format mà Zalo ZNS API yêu cầu.
package phone

import (
	"regexp"
	"strings"
)

// localMobileRe khớp số di động VN dạng local (0XXXXXXXXX, 10 chữ số).
//
// Đầu số hợp lệ (3 chữ số đầu của số local):
//   03[2-9]  Viettel
//   05[2-9]  Vietnamobile (052,056,058), Reddi (055), Gmobile (059)
//   07[06-9] Mobifone (070, 076-079)
//   08[1-9]  Vinaphone (081-086,088), Mobifone (089)
//   09[0-9]  tất cả nhà mạng (090-099)
//
// Loại trừ có chủ đích:
//   01x  đầu số cũ, hết hiệu lực tháng 11/2018
//   070  không cấp phát (Mobifone dùng 070 từ 2019, nhưng là 07[06-9])
//   080  không cấp phát
var localMobileRe = regexp.MustCompile(`^0(3[2-9]|5[2-9]|7[06-9]|8[1-9]|9[0-9])\d{7}$`)

// IsValidMobile trả true nếu phone là số di động VN hợp lệ.
// Chấp nhận: "0912345678", "+84912345678", "84912345678".
// Bỏ qua khoảng trắng đầu/cuối.
func IsValidMobile(phone string) bool {
	return localMobileRe.MatchString(toLocal(phone))
}

// Normalize chuyển phone sang dạng "84XXXXXXXXX" (E.164, không có "+").
// Trả ("", false) nếu phone không phải số di động VN hợp lệ.
// Đây là hàm nên dùng trước khi gọi Zalo ZNS API.
func Normalize(phone string) (string, bool) {
	local := toLocal(phone)
	if !localMobileRe.MatchString(local) {
		return "", false
	}
	return "84" + local[1:], true
}

// NormalizeMust chuyển sang dạng "84XXXXXXXXX" mà không validate.
// Dùng khi số đã được validate trước, hoặc cho backward-compat.
// Handles: "0912..." → "84912...", "84912..." → "84912...", "+84912..." → "84912...".
func NormalizeMust(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.TrimPrefix(p, "+")
	if strings.HasPrefix(p, "84") {
		return p
	}
	return "84" + strings.TrimPrefix(p, "0")
}

// toLocal chuẩn hóa về dạng "0XXXXXXXXX" để khớp với localMobileRe.
// Xử lý: "84XXXXXXXXX" và "+84XXXXXXXXX" → "0XXXXXXXXX".
func toLocal(phone string) string {
	p := strings.TrimSpace(phone)
	p = strings.TrimPrefix(p, "+")
	if strings.HasPrefix(p, "84") && len(p) == 11 {
		return "0" + p[2:]
	}
	return p
}

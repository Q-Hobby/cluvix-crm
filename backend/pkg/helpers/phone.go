package helpers

import "regexp"

var rePhoneNonDigit = regexp.MustCompile(`[^0-9]`)

// PhoneVariants expands a phone string into all plausible representations
// (with/without country code, with/without leading 0, with +) so a single
// stored phone can be matched against user-input variations.
//
// Nguồn của vấn đề: ZMA lưu số dạng Zalo trả về (`84906904884`), web CRM lưu
// dạng người dùng gõ (`0906904884`) → so sánh `phone = ?` luôn trượt và sinh
// khách trùng. Mọi chỗ dò trùng số điện thoại PHẢI dùng hàm này.
func PhoneVariants(phone, telephoneCode string) []string {
	cleaned := rePhoneNonDigit.ReplaceAllString(phone, "")
	if cleaned == "" {
		return []string{phone}
	}

	variants := []string{cleaned}
	codeLen := len(telephoneCode)

	switch {
	case telephoneCode != "" && len(cleaned) > codeLen && cleaned[:codeLen] == telephoneCode:
		local := cleaned[codeLen:]
		variants = append(variants, "0"+local, local, "+"+cleaned)
	case cleaned[0] == '0':
		local := cleaned[1:]
		variants = append(variants, telephoneCode+local, local, "+"+telephoneCode+local)
	default:
		variants = append(variants, "0"+cleaned, telephoneCode+cleaned, "+"+telephoneCode+cleaned)
	}

	seen := map[string]bool{}
	var unique []string
	for _, v := range variants {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}
	return unique
}

// PhoneLast9 trả 9 chữ số cuối của số điện thoại (định danh thực của thuê bao
// VN), hoặc "" nếu chuỗi có dưới 10 chữ số — quá ngắn để coi là số hợp lệ nên
// không được dùng làm khoá dò trùng (dễ khớp nhầm hàng loạt).
func PhoneLast9(phone string) string {
	cleaned := rePhoneNonDigit.ReplaceAllString(phone, "")
	if len(cleaned) < 10 {
		return ""
	}
	return cleaned[len(cleaned)-9:]
}

// PhoneMatchCond dựng mảnh WHERE khớp một số điện thoại ở MỌI định dạng đang
// tồn tại trong DB. Trả về (sql, args) đã bọc sẵn ngoặc để AND an toàn với các
// điều kiện scope khác:
//
//	cond, args := helpers.PhoneMatchCond("phone", phone, define.TelephoneCodeVN)
//	db.Table("customer").Where(cond, args...).Where("company_id = ?", cid).Count(&n)
//
// `column` do code truyền (hằng số trong repo), KHÔNG nhận từ input người dùng.
func PhoneMatchCond(column, phone, telephoneCode string) (string, []any) {
	variants := PhoneVariants(phone, telephoneCode)
	if last9 := PhoneLast9(phone); last9 != "" {
		return "(" + column + " IN ? OR " + column + " LIKE ?)", []any{variants, "%" + last9}
	}
	return "(" + column + " IN ?)", []any{variants}
}

package service

import (
	"time"

	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/validation"
)

// ValidateFromMap method để handler trong package khác gọi mà không cần thêm import.
func (s *CustomerService) ValidateFromMap(body map[string]interface{}, isCreate bool) error {
	return validateCustomerFromMap(body, isCreate)
}

// validateCustomerFromMap — port từ be/common/models/Customer.php::rules() static portion.
// Skip: dynamic rules từ ControlFieldInTable (Yii2 đọc DB → field list theo role) + custom
// validators (checkPhone, validateTelesalePermission, filter convertToTitleCase) — đã có
// guard riêng trong handler/migration phase.
//
// isCreate=true → fullname required (Yii2 SCENARIO_VALIDATE_CREATE — handler đã check sẵn,
// đặt ở đây để dual-source-of-truth). Update không bắt buộc fullname (partial update).
func validateCustomerFromMap(body map[string]interface{}, isCreate bool) error {
	v := validation.New()

	if isCreate {
		v.Required(helpers.ToString(body["fullname"]), "Họ tên")
	}

	if email := helpers.ToString(body["email"]); email != "" {
		v.Email(email, "Email")
	}
	if yearBorn := helpers.ToInt64(body["year_born"]); yearBorn != 0 {
		v.Range(yearBorn, 1900, 3000, "Năm sinh")
	}
	// birthday đã qua NormalizeDateOnly (FilterAllowed) nhưng hàm đó chỉ format lại chuỗi
	// parse được — chuỗi sai dạng (vd năm 5 chữ số "20213-03-20") bị giữ nguyên để MySQL
	// tự reject (Error 1292), lộ SQL error thô ra ngoài thay vì validate message. Chặn ở
	// đây bằng round-trip parse thật.
	if bd, ok := body["birthday"]; ok && bd != nil {
		if s, isStr := bd.(string); isStr && s != "" {
			if t, err := time.Parse("2006-01-02", s); err != nil || t.Format("2006-01-02") != s {
				v.Add("Ngày sinh không hợp lệ")
			}
		}
	}
	if ct, ok := body["customerType"]; ok && ct != nil {
		// 1=NEW, 2=OLD
		v.In(int(helpers.ToInt64(ct)), []int{1, 2}, "Loại khách")
	}

	v.MaxLen(helpers.ToString(body["fullname"]), 100, "Họ tên")
	v.MaxLen(helpers.ToString(body["loai_message"]), 100, "Loại tin nhắn")
	v.MaxLen(helpers.ToString(body["nickname"]), 50, "Tên gọi khác")
	v.MaxLen(helpers.ToString(body["code"]), 50, "Mã khách hàng")
	v.MaxLen(helpers.ToString(body["old_code"]), 50, "Mã cũ")
	v.MaxLen(helpers.ToString(body["phone"]), 25, "SĐT")
	v.MaxLen(helpers.ToString(body["other_phone"]), 25, "SĐT khác")
	v.MaxLen(helpers.ToString(body["address"]), 500, "Địa chỉ")
	v.MaxLen(helpers.ToString(body["address_other"]), 500, "Địa chỉ khác")
	v.MaxLen(helpers.ToString(body["link_message"]), 500, "Link tin nhắn")
	v.MaxLen(helpers.ToString(body["chat_message"]), 500, "Chat tin nhắn")
	v.MaxLen(helpers.ToString(body["avatar"]), 500, "Avatar")

	if v.HasErrors() {
		return v
	}
	return nil
}

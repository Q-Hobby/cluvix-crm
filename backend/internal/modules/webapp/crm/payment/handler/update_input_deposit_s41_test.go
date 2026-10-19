package handler

import (
	"encoding/json"
	"testing"
)

// Story-41 (chốt từ story-37 QA-C): body PUT payment/deposit có field `deposit_id` (cùng
// customer_id / serviceUsed_id / appointment_id) nhưng `ToParams` KHÔNG được phát các key đó sang
// service — nếu phát, UpdatePaymentCustomer nhánh chuyển cọc sẽ tính theo cọc do client chọn
// (không qua validateDepositUse, không so customer) ⇒ IDOR. Lớp thứ hai (service
// mergePaymentParams) neo ở crm/service/payment_update_deposit_s41_test.go.
func TestS41UpdatePaymentInput_ToParams_KhongPhatDepositID(t *testing.T) {
	var in UpdatePaymentInput
	raw := `{"updated_at":5,"branch_id":1,"amount":"100","note":"n",
		"deposit_id":999,"customer_id":998,"serviceUsed_id":997,"appointment_id":996}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if in.DepositID == nil || *in.DepositID != 999 {
		t.Fatalf("đối chứng: DTO phải bind được deposit_id từ body, got %v", in.DepositID)
	}
	m := in.ToParams()
	for _, k := range []string{"deposit_id", "customer_id", "serviceUsed_id", "appointment_id"} {
		if _, ok := m[k]; ok {
			t.Errorf("ToParams phát key %q — Update không được đổi liên kết này", k)
		}
	}
	if _, ok := m["note"]; !ok {
		t.Fatalf("đối chứng: ToParams phải phát key note — test vô nghĩa")
	}
}

package service

import (
	"testing"

	"github.com/shopspring/decimal"

	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
)

// Story-41 (chốt từ story-37 QA-C): PUT payment KHÔNG được đổi `deposit_id` (cùng các liên kết
// customer_id / serviceUsed_id / appointment_id). Nhánh chuyển cọc của UpdatePaymentCustomer tính
// `deposit_after` theo `current.DepositId` mà không gọi validateDepositUse — an toàn CHỈ VÌ id cọc
// luôn là giá trị đã lưu. Test này neo lớp thứ hai (mergePaymentParams bỏ qua các key đó); lớp thứ
// nhất (DTO `UpdatePaymentInput.ToParams` không phát key) neo ở crm/payment/handler.
func TestS41MergePaymentParams_KhongDoiDepositID(t *testing.T) {
	dep, cust, su, appt := 11, 22, 33, 44
	p := &crmPaymentDao.Payment{DepositId: &dep, CustomerId: &cust, ServiceUsedId: &su, AppointmentId: &appt}
	(&PaymentService{}).mergePaymentParams(p, map[string]interface{}{
		"amount":         decimal.NewFromInt(100),
		"deposit_id":     999,
		"customer_id":    998,
		"serviceUsed_id": 997,
		"appointment_id": 996,
	})
	if p.DepositId == nil || *p.DepositId != dep {
		t.Errorf("deposit_id bị đổi qua Update: %v — muốn giữ %d", p.DepositId, dep)
	}
	if p.CustomerId == nil || *p.CustomerId != cust {
		t.Errorf("customer_id bị đổi qua Update: %v — muốn giữ %d", p.CustomerId, cust)
	}
	if p.ServiceUsedId == nil || *p.ServiceUsedId != su {
		t.Errorf("serviceUsed_id bị đổi qua Update: %v — muốn giữ %d", p.ServiceUsedId, su)
	}
	if p.AppointmentId == nil || *p.AppointmentId != appt {
		t.Errorf("appointment_id bị đổi qua Update: %v — muốn giữ %d", p.AppointmentId, appt)
	}
	// Đối chứng: merge CÓ chạy (key hợp lệ được áp).
	if !p.Amount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("amount = %s — merge không chạy, test vô nghĩa", p.Amount)
	}
}

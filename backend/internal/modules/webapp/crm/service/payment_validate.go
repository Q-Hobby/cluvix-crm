package service

import (
	crmPaymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/validation"
)

// validatePaymentCommon — port từ be/common/models/Payment.php::rules()
// Áp dụng cho mọi Create flow (createPayment choke point) và Update sau merge.
//
// Required: amount > 0 (đã check trước), company_id (đã check ở CreatePaymentClinic;
//
//	các flow khác inject từ order/svc — nếu nil, để GORM trả lỗi NOT NULL).
//
// Required when payment_for ∉ {CLINIC_INCOME, CLINIC_EXPENSE}: customer_id.
// Enum: payment_for ∈ valid set; deposit_type ∈ {0, RESERVE, TEMP, KG}.
// Cross-field: payment_of_date >= payment_time.
// Number: deposit_after >= 0.
// String: bank_code max 255, note max 300.
func validatePaymentCommon(p *crmPaymentDao.Payment) error {
	v := validation.New()

	if p.PaymentFor != nil {
		v.In(*p.PaymentFor, []int{
			define.PaymentForRefundService,
			define.PaymentForRefundDeposit,
			define.PaymentForDeposit,
			define.PaymentForOrder,
			define.PaymentForMoveDeposit,
			define.PaymentForRefundDepositKG,
			define.PaymentForClinicIncome,
			define.PaymentForClinicExpense,
		}, "Loại thanh toán")

		// customer_id required khi không phải thu/chi phòng khám
		if *p.PaymentFor != define.PaymentForClinicIncome && *p.PaymentFor != define.PaymentForClinicExpense {
			v.Required(p.CustomerId, "Khách hàng")
		}
	}

	if p.DepositType != nil {
		v.In(*p.DepositType, []int{
			0,
			define.PaymentTypeDepositReserve,
			define.PaymentTypeDepositTemp,
			define.PaymentTypeDepositKG,
		}, "Loại đặt cọc")
	}

	if p.PaymentTime != nil && p.PaymentOfDate != nil && *p.PaymentOfDate < *p.PaymentTime {
		v.Add("Ngày thanh toán không thể trước ngày thu tiền")
	}

	if p.DepositAfter != nil && p.DepositAfter.IsNegative() {
		v.Add("Tiền đặt cọc sau khi thanh toán không thể nhỏ hơn 0")
	}

	v.Required(p.BranchId, "Chi nhánh")
	v.MaxLen(p.BankCode, 255, "Mã hóa đơn ngân hàng")
	v.MaxLen(p.Note, 300, "Ghi chú")

	if v.HasErrors() {
		return v
	}
	return nil
}

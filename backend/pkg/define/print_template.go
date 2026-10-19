package define

// ValidPrintTemplateTypes — danh sách template_type hợp lệ.
// Mỗi type map 1-1 với 1 print page trên FE.
// Thêm type mới: thêm vào đây + thêm placeholder list trên FE.
var ValidPrintTemplateTypes = map[string]bool{
	PrintTemplatePayment:           true,
	PrintTemplateDeposit:           true,
	PrintTemplateRefundDeposit:     true,
	PrintTemplateRefundConsignment: true,
	PrintTemplateRefundService:     true,
	PrintTemplateOrder:             true,
	PrintTemplateMultiPayment:      true,
	PrintTemplateReceptionPayment:  true,
	PrintTemplatePaymentInDay:      true,
}

const (
	PrintTemplatePayment           = "payment"
	PrintTemplateDeposit           = "deposit"
	PrintTemplateRefundDeposit     = "refund_deposit"
	PrintTemplateRefundConsignment = "refund_consignment"
	PrintTemplateRefundService     = "refund_service"
	PrintTemplateOrder             = "order"
	PrintTemplateMultiPayment      = "multi_payment"
	PrintTemplateReceptionPayment  = "reception_payment"
	PrintTemplatePaymentInDay      = "payment_in_day"
)

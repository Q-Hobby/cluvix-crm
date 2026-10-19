package errs

import "errors"

var ErrMissingPaymentFor = errors.New("missing_payment_for")
var ErrInvalidPaymentFor = errors.New("invalid_payment_for")
var ErrMissingParentID = errors.New("missing_parent_id")
var ErrOrderNotFound = errors.New("order_not_found")
var ErrNoPayableService = errors.New("no_payable_service")
var ErrInvalidServiceList = errors.New("invalid_service_list")
var ErrAmountMustBePositive = errors.New("amount_must_be_positive")
var ErrServicesMultipleCustomers = errors.New("services_multiple_customers")
var ErrMissingDepositID = errors.New("missing_deposit_id")
var ErrRefundExceedsDeposit = errors.New("refund_exceeds_deposit")
var ErrRefundExceedsEscrow = errors.New("refund_exceeds_escrow")
var ErrMissingServiceUsedID = errors.New("missing_service_used_id")
var ErrRefundExceedsPaid = errors.New("refund_exceeds_paid")
var ErrDepositNotFound = errors.New("deposit_not_found")

// ErrDepositCustomerMismatch — phiếu dùng cọc (hoàn cọc / hoàn ký gửi / trả dịch vụ bằng cọc) trỏ tới cọc,
// dòng dịch vụ hoặc đơn của KHÁCH KHÁC / công ty khác (deposit_id, serviceUsed_id lấy từ body).
var ErrDepositCustomerMismatch = errors.New("deposit_customer_mismatch")
var ErrTransferExceedsDeposit = errors.New("transfer_exceeds_deposit")
var ErrPaymentNotFound = errors.New("payment_not_found")
var ErrCannotChangeTransferType = errors.New("cannot_change_transfer_type")
var ErrDepositRemainingNegative = errors.New("deposit_remaining_negative")
var ErrDepositUsedCannotUpdate = errors.New("deposit_used_cannot_update")
var ErrDepositUsedCannotDelete = errors.New("deposit_used_cannot_delete")
var ErrDepositReturnNotFound = errors.New("deposit_return_not_found")
var ErrRefundNotFound = errors.New("refund_not_found")
var ErrServiceRefundedCannotPay = errors.New("service_refunded_cannot_pay")
var ErrPaymentBeforeOrderDate = errors.New("payment_before_order_date")
var ErrEscrowNotFound = errors.New("escrow_not_found")

// ErrPaymentBusyRetry — transaction tiền dính deadlock MySQL 1213 lặp lại sau số lần thử tối đa
// (nhiều thao tác cùng khách cùng lúc). Không ghi gì; người dùng bấm lại.
var ErrPaymentBusyRetry = errors.New("payment_busy_retry")

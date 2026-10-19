package dto

import "github.com/shopspring/decimal"

// PaymentListDTO — shape cho list/detail endpoints trả mảng payment.
type PaymentListDTO struct {
	ID            uint             `json:"id"`
	Code          string           `json:"code"`
	CustomerId    *int             `gorm:"column:customer_id" json:"customer_id"`
	ParentId      *int             `gorm:"column:parent_id" json:"parent_id"`
	ServiceUsedId *int             `gorm:"column:serviceUsed_id" json:"serviceUsed_id"`
	BranchId      int              `gorm:"column:branch_id" json:"branch_id"`
	Method        int              `json:"method"`
	PaymentTime   *int64           `gorm:"column:payment_time" json:"payment_time"`
	PaymentOfDate *int64           `gorm:"column:payment_of_date" json:"payment_of_date"`
	Amount        decimal.Decimal  `json:"amount"`
	DepositAfter  *decimal.Decimal `gorm:"column:deposit_after" json:"deposit_after"`
	DepositType   *int             `gorm:"column:deposit_type" json:"deposit_type"`
	PaymentFor    *int             `gorm:"column:payment_for" json:"payment_for"`
	Note          string           `json:"note"`
	CreatedBy     *uint            `gorm:"column:created_by" json:"created_by"`
	CreatedAt     int64            `gorm:"column:created_at" json:"created_at"`
	UpdatedBy     *uint            `gorm:"column:updated_by" json:"updated_by"`
	UpdatedAt     int64            `gorm:"column:updated_at" json:"updated_at"`
}

// PaymentTimelineDTO — shape rút gọn cho timeline customer.
type PaymentTimelineDTO struct {
	PaymentFor    *int            `gorm:"column:payment_for" json:"payment_for"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentTime   *int64          `gorm:"column:payment_time" json:"payment_time"`
	PaymentOfDate *int64          `gorm:"column:payment_of_date" json:"payment_of_date"`
	Note          string          `json:"note"`
	Method        int             `json:"method"`
}

// PaymentSumDTO — aggregate cho report endpoints.
type PaymentSumDTO struct {
	Total decimal.Decimal `json:"total"`
	Count int64           `json:"count"`
}

// PaymentPrintDTO — shape cho print receipt (PDF render).
type PaymentPrintDTO struct {
	ID            uint             `json:"id"`
	Code          string           `json:"code"`
	CustomerId    *int             `gorm:"column:customer_id" json:"customer_id"`
	BranchId      int              `gorm:"column:branch_id" json:"branch_id"`
	Method        int              `json:"method"`
	PaymentTime   *int64           `gorm:"column:payment_time" json:"payment_time"`
	PaymentOfDate *int64           `gorm:"column:payment_of_date" json:"payment_of_date"`
	Amount        decimal.Decimal  `json:"amount"`
	DepositAfter  *decimal.Decimal `gorm:"column:deposit_after" json:"deposit_after"`
	PaymentFor    *int             `gorm:"column:payment_for" json:"payment_for"`
	Note          string           `json:"note"`
}

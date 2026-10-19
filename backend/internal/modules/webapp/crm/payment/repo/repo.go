// Package payment — Phase 4 prototype repository layer.
//
// Mục đích: tách read queries của payment service khỏi GORM cụ thể để service
// có thể unit-test pure (không cần MySQL container). Đây là PROTOTYPE — chỉ 1
// method để chứng minh pattern. Khi rollout đầy đủ (xem refactor-roadmap.md §4),
// extract dần các read query khác trong payment.go::beforeSave/afterSave.
//
// Pattern:
//
//	Reader interface  →  defines domain-level queries (no SQL leak in signature)
//	gormReader        →  production impl backed by *gorm.DB
//	test fake reader  →  any struct satisfying Reader for unit tests
//
// Transaction-awareness: methods nhận `*gorm.DB` tx để chạy bên trong transaction
// caller mở. Mock impl ignore param này.
package repo

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PaymentSnapshot — current persisted updated_at (khoá lạc quan D6) + amount cho optimistic lock check.
type PaymentSnapshot struct {
	UpdatedAt int64           `gorm:"column:updated_at"`
	Amount    decimal.Decimal `gorm:"column:amount"`
}

// Reader exposes read queries used by crm.PaymentService.
type Reader interface {
	// ServiceUsedRefund returns COALESCE(refundServiceUsed, 0) của order_service_used row.
	// Dùng trong beforeSave để chặn payment lên service đã hoàn (PaymentForOrder, PaymentForMoveDeposit).
	ServiceUsedRefund(tx *gorm.DB, serviceUsedID int) (decimal.Decimal, error)

	// ServiceUsedPlanID returns order_plan_id của order_service_used row.
	// Dùng trong beforeSave (insert) để set Payment.ParentId.
	ServiceUsedPlanID(tx *gorm.DB, serviceUsedID int) (*int, error)

	// PaymentSnapshot reads current updated_at + amount cho optimistic lock + change detect.
	// Returns ErrPaymentNotFound semantics ở caller nếu row missing.
	PaymentSnapshot(tx *gorm.DB, paymentID uint) (PaymentSnapshot, error)
}

// NewGormReader returns the production Reader backed by GORM.
func NewGormReader() Reader { return gormReader{} }

type gormReader struct{}

func (gormReader) ServiceUsedRefund(tx *gorm.DB, serviceUsedID int) (decimal.Decimal, error) {
	var v decimal.Decimal
	err := tx.Table("order_service_used").
		Select("COALESCE(refundServiceUsed, 0)").
		Where("id = ?", serviceUsedID).
		Scan(&v).Error
	return v, err
}

func (gormReader) ServiceUsedPlanID(tx *gorm.DB, serviceUsedID int) (*int, error) {
	var planID *int
	err := tx.Table("order_service_used").
		Select("order_plan_id").
		Where("id = ?", serviceUsedID).
		Scan(&planID).Error
	return planID, err
}

func (gormReader) PaymentSnapshot(tx *gorm.DB, paymentID uint) (PaymentSnapshot, error) {
	var s PaymentSnapshot
	err := tx.Table("payment").Select("updated_at, amount").Where("id = ?", paymentID).Take(&s).Error
	return s, err
}

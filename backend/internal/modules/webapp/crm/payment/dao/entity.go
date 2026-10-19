package dao

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type Payment struct {
	ID                uint             `gorm:"primaryKey" json:"id" form:"-"`
	Code              string           `gorm:"uniqueIndex;size:50" json:"code,omitempty"`
	OldCode           string           `gorm:"column:old_code;size:50;comment:Mã phiếu thu/chi cũ (sync đối tác)" json:"old_code,omitempty"`
	CreatedBy         *uint            `json:"created_by,omitempty"`
	UpdatedBy         *uint            `json:"updated_by,omitempty"`
	CreatedAt         int64            `json:"created_at"`
	UpdatedAt         int64            `json:"updated_at"`
	CustomerId        *int             ` gorm:"column:customer_id;index;comment:ID khách hàng" json:"customer_id"`
	ParentId          *int             ` gorm:"column:parent_id;index;comment:Mã id của đơn hàng" json:"parent_id"`
	ServiceUsedId     *int             ` gorm:"column:serviceUsed_id;index;comment:Thanh toán cho dịch vụ nào" json:"serviceUsed_id"`
	BranchId          int              ` gorm:"column:branch_id;index;not null;comment:Chi nhánh" json:"branch_id"`
	Method            int              ` gorm:"index;not null;comment:Hình thức thanh toán" json:"method"`
	CardTypeId        *int             ` gorm:"column:card_type_id;index;comment:Phương thức thanh toán" json:"card_type_id"`
	BankCode          string           ` gorm:"column:bank_code;size:255;comment:Mã hóa đơn ngân hàng" json:"bank_code"`
	PaymentTime       *int64           ` gorm:"column:payment_time;index;comment:Ngày thu tiền" json:"payment_time"`
	PaymentOfDate     *int64           ` gorm:"column:payment_of_date;index;comment:Ngày thanh toán" json:"payment_of_date"`
	Amount            decimal.Decimal  ` gorm:"type:decimal(20,2);not null;comment:Tổng tiền thanh toán" json:"amount"`
	AmountType        *int             ` gorm:"column:amount_type;comment:Loại số tiền" json:"amount_type"`
	DepositAfter      *decimal.Decimal ` gorm:"column:deposit_after;type:decimal(20,2);default:0.00;comment:Tiền đặt cọc sau khi thanh toán" json:"deposit_after"`
	DepositId         *int             ` gorm:"column:deposit_id;index;comment:ID payment đặt cọc gốc (dùng khi chuyển cọc)" json:"deposit_id"`
	DepositType       *int             ` gorm:"column:deposit_type;default:0;comment:Loại đặt cọc" json:"deposit_type"`
	PaymentFor        *int             ` gorm:"column:payment_for;index;default:0;comment:1-Hoàn dịch vụ, 2-Hoàn cọc, 3-Đặt cọc, 4-Thanh toán, 5-Chuyển cọc, 6-Hoàn ký gửi, 7-Thu tiền, 8-Chi tiền" json:"payment_for"`
	Note              string           ` gorm:"comment:Ghi chú" json:"note"`
	AppointmentId     *int             ` gorm:"column:appointment_id;index;comment:Lịch hẹn liên quan (FK → appointment)" json:"appointment_id"`
	ServiceIdSnapshot *int             ` gorm:"column:service_id_snapshot;comment:service_id của order_service_used tại thời điểm payment này phát sinh, NULL = chưa từng đổi dịch vụ" json:"service_id_snapshot"`
}

func (Payment) TableName() string { return "payment" }

type OrderInstallment struct {
	ID                 uint             `gorm:"primaryKey" json:"id" form:"-"`
	Status             *int             `gorm:"default:1" json:"status,omitempty"`
	CreatedAt          int64            `gorm:"column:created_at" json:"created_at"`
	OrderServiceUsedId int              ` gorm:"column:order_service_used_id;index;not null;comment:ID của order_service_used" json:"order_service_used_id"`
	PeriodNumber       int              ` gorm:"column:period_number;not null;comment:Kỳ thứ mấy (1,2,3...)" json:"period_number"`
	Amount             decimal.Decimal  ` gorm:"type:decimal(20,2);not null;comment:Số tiền kỳ này" json:"amount"`
	DueDate            time.Time        ` gorm:"column:due_date;index;not null;comment:Ngày đến hạn" json:"due_date"`
	PaidDate           *time.Time       ` gorm:"column:paid_date;comment:Ngày thanh toán thực tế" json:"paid_date"`
	PaidAmount         *decimal.Decimal ` gorm:"column:paid_amount;type:decimal(20,2);default:0.00;comment:Số tiền đã thanh toán" json:"paid_amount"`
	Note               string           ` gorm:"comment:Ghi chú" json:"note"`
}

func (OrderInstallment) TableName() string { return "order_installment" }

type LogPayment struct {
	ID            uint           `gorm:"primaryKey" json:"id" form:"-"`
	PaymentId     *int           ` gorm:"column:payment_id;index;comment:Thanh toán" json:"payment_id"`
	ActionType    *int           ` gorm:"column:action_type;default:2;comment:Loại hành động: 1: create, 2: update, 3: delete" json:"action_type"`
	ChangedFields datatypes.JSON `gorm:"column:changed_fields;type:json" json:"changed_fields"`
	ChangedBy     *int           ` gorm:"column:changed_by;index" json:"changed_by"`
	ChangedAt     *int           ` gorm:"column:changed_at" json:"changed_at"`
}

func (LogPayment) TableName() string { return "log_payment" }

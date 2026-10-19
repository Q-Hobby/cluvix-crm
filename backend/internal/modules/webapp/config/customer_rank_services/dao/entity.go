package dao

import (
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type CustomerRankServices struct {
	ID              uint             `gorm:"primaryKey" json:"id" form:"-"`
	Status          *int             `gorm:"default:1" json:"status,omitempty"`
	CreatedAt       int64            `json:"created_at"`
	RankId          int              ` gorm:"column:rank_id;index;not null;comment:ID hạng thành viên" json:"rank_id"`
	ServiceIds      datatypes.JSON   `gorm:"column:service_ids;type:json;comment:Danh sách ID dịch vụ (JSON array)" json:"service_ids"`
	DiscountPercent *decimal.Decimal ` gorm:"column:discount_percent;default:0.00;comment:% giảm giá riêng cho các dịch vụ này" json:"discount_percent"`
}

func (CustomerRankServices) TableName() string { return "customer_rank_services" }

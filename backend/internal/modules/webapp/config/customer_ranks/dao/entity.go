package dao

import "github.com/shopspring/decimal"

type CustomerRanks struct {
	ID              uint             `gorm:"primaryKey" json:"id" form:"-"`
	Status          *int             `gorm:"default:1" json:"status,omitempty"`
	CreatedAt       int64            `json:"created_at"`
	Name            string           ` gorm:"size:100;not null;comment:Tên hạng (VIP, Gold, Silver, Bronze)" json:"name"`
	MinPoints       int              ` gorm:"column:min_points;not null;default:0;comment:Điểm tối thiểu để đạt hạng" json:"min_points"`
	MaxPoints       *int             ` gorm:"column:max_points;comment:Điểm tối đa (NULL = không giới hạn)" json:"max_points"`
	DiscountPercent *decimal.Decimal ` gorm:"column:discount_percent;default:0.00;comment:% giảm giá chung (chỉ áp dụng khi không có dịch vụ riêng)" json:"discount_percent"`
	PriorityLevel   *int             ` gorm:"column:priority_level;default:0;comment:Mức ưu tiên đặt lịch" json:"priority_level"`
	ColorCode       string           ` gorm:"column:color_code;size:50;comment:Màu hiển thị (#FF0000 hoặc tên màu)" json:"color_code"`
	Description     string           ` gorm:"comment:Mô tả quyền lợi" json:"description"`
}

func (CustomerRanks) TableName() string { return "customer_ranks" }

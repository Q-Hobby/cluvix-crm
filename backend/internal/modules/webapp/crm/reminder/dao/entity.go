package dao

import (
	"erpCluvix/internal/models"

	"gorm.io/datatypes"
)

type Reminder struct {
	models.BaseModelWithCode
	CustomerId  *int           ` gorm:"column:customer_id;index;comment:Khách hàng" json:"customer_id"`
	Type        *int           ` gorm:"comment:Loại" json:"type"`
	StartTime   *int64         ` gorm:"column:start_time;comment:Thời gian bắt đầu" json:"start_time"`
	Note        string         ` gorm:"not null;comment:Nội dung lời nhắc" json:"note"`
	RelatedUser datatypes.JSON `gorm:"column:relatedUser;type:json" json:"relatedUser"`
}

func (Reminder) TableName() string { return "reminder" }

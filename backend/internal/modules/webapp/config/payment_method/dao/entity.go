package dao

import "erpCluvix/internal/models"

type PaymentMethod struct {
	models.SimpleModel
	Name      string ` gorm:"uniqueIndex;size:255;not null" json:"name"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	CreatedBy *int   ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy *int   ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (PaymentMethod) TableName() string { return "payment_method" }

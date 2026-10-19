package dao

import "erpCluvix/internal/models"

type CardType struct {
	models.SimpleModel
	Name      string ` gorm:"uniqueIndex;size:255;not null;comment:Tên" json:"name"`
	Method    int    `gorm:"not null;comment:Áp dụng cho hình thức thanh toán nào (FK → payment_method)" json:"method"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	Note      string ` gorm:"comment:Ghi chú" json:"note"`
	CreatedBy *int   ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy *int   ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (CardType) TableName() string { return "card_type" }

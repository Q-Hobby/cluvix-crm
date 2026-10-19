package dao

import "erpCluvix/internal/models"

type AppointmentType struct {
	models.SimpleModel
	Name          string `gorm:"uniqueIndex;size:255;comment:Tên loại lịch hẹn" json:"name"`
	IsTypeRevenue *int   `gorm:"column:isTypeRevenue;default:0;comment:Loại hẹn tính doanh số (1=có, 0=không)" json:"isTypeRevenue"`
	SortOrder     *int   `gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	CreatedBy     *int   `gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy     *int   `gorm:"column:updated_by;index" json:"updated_by"`
}

func (AppointmentType) TableName() string { return "appointment_type" }

package dao

import "erpCluvix/internal/models"

type ServiceType struct {
	ID        uint  `gorm:"primaryKey" json:"id" form:"-"`
	Status    *int  `gorm:"default:1" json:"status,omitempty"`
	CreatedBy *uint `json:"created_by,omitempty"`
	UpdatedBy *uint `json:"updated_by,omitempty"`
	models.Timestamps
	Name      string ` gorm:"size:255;not null" json:"name"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
}

func (ServiceType) TableName() string { return "service_type" }

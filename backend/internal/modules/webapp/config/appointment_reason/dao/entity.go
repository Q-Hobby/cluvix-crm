package dao

import "erpCluvix/internal/models"

type AppointmentReason struct {
	models.SimpleModel
	Name         string `gorm:"size:255;comment:Lý do hẹn" json:"name"`
	SortOrder    *int   `gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	TimeNhaclich *int   `gorm:"column:time_nhaclich;comment:Thời gian nhắc lịch (phút/giờ/ngày tùy config)" json:"time_nhaclich"`
	CreatedBy    *int   `gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy    *int   `gorm:"column:updated_by;index" json:"updated_by"`
}

func (AppointmentReason) TableName() string { return "appointment_reason" }

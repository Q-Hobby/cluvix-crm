package dao

import "erpCluvix/internal/models"

type AppointmentIsAccepted struct {
	models.SimpleModel
	Name       string `gorm:"uniqueIndex;size:255;not null;comment:Tên trạng thái tư vấn" json:"name"`
	Severity   string `gorm:"size:25;default:info;comment:Màu hiển thị trạng thái (info, success, warning, danger)" json:"severity"`
	Desc       string `gorm:"comment:Mô tả" json:"desc"`
	SortOrder  *int   `gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	IsAccepted int8   `gorm:"column:isAccepted;default:0;comment:1 = trạng thái này tính là đồng ý/chốt" json:"isAccepted"`
	CreatedBy  *int   `gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy  *int   `gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (AppointmentIsAccepted) TableName() string { return "appointment_is_accepted" }

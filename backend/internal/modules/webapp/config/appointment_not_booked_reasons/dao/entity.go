package dao

import "erpCluvix/internal/models"

type AppointmentNotBookedReasons struct {
	models.SimpleModel
	ParentId  *int   `gorm:"column:parent_id;index;comment:FK → parent (danh mục cha, hỗ trợ 2 cấp)" json:"parent_id"`
	Name      string `gorm:"size:255;not null;comment:Tên lý do chưa ra hẹn lịch" json:"name"`
	Desc      string `gorm:"comment:Mô tả" json:"desc"`
	SortOrder *int   `gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	CreatedBy *int   `gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy *int   `gorm:"column:updated_by;index" json:"updated_by"`
}

func (AppointmentNotBookedReasons) TableName() string { return "appointment_not_booked_reasons" }

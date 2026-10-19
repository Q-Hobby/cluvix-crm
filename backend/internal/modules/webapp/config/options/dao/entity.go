package dao

import "erpCluvix/internal/models"

type UserOption struct {
	models.SimpleModel
	Name      string ` gorm:"uniqueIndex;size:255;not null;comment:Tên option" json:"name"`
	Desc      string ` gorm:"comment:Nội dung chi tiết" json:"desc"`
	Options   int    ` gorm:"not null;comment:Option đúng hoặc sai" json:"options"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	CreatedBy *int   ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy *int   ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (UserOption) TableName() string { return "user_option" }

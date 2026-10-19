package dao

import (
	"erpCluvix/internal/models"

	"gorm.io/datatypes"
)

type GroupLocal struct {
	models.SimpleModel
	Name               string         ` gorm:"uniqueIndex;size:255;not null;comment:Tên khu vực" json:"name"`
	Desc               string         ` gorm:"comment:Nội dung chi tiết" json:"desc"`
	SortOrder          *int           ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	LocationProvinceId datatypes.JSON `gorm:"column:locationProvinceId;type:json;comment:Danh sách tỉnh thành" json:"locationProvinceId"`
	CreatedBy          *int           ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy          *int           ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (GroupLocal) TableName() string { return "group_local" }

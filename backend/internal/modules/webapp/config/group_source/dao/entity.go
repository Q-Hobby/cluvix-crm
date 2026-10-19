package dao

import (
	"erpCluvix/internal/models"

	"gorm.io/datatypes"
)

type GroupSource struct {
	models.SimpleModel
	Name      string         ` gorm:"uniqueIndex;size:255;not null;comment:Tên nhóm nguồn" json:"name"`
	Desc      string         ` gorm:"comment:Nội dung chi tiết" json:"desc"`
	SortOrder *int           ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	Online    *int           ` gorm:"default:0;comment:Xác định nhóm nguồn online" json:"online"`
	SourceId  datatypes.JSON `gorm:"column:sourceId;type:json;comment:Danh sách nguồn khách" json:"sourceId"`
	// Story-39 (E6/D15): bỏ field JSON companyId (cột NULL được ở schema cũ); bỏ hẳn ở story-25/62.
	CreatedBy *int ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy *int ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (GroupSource) TableName() string { return "group_source" }

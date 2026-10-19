package dao

import "erpCluvix/internal/models"

type CustomerCategory struct {
	models.SimpleModel
	Name string ` gorm:"uniqueIndex;size:255;not null;comment:Tên nhóm khách hàng" json:"name"`
	// Story-39 (E6/D15): bỏ field JSON companyId (cột NULL được ở schema cũ); bỏ hẳn ở story-25/62.
	SortOrder    *int `gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	TimeNhaclich *int `gorm:"column:time_nhaclich;comment:Thời gian nhắc lịch chăm sóc (số giờ hoặc ngày tùy type_nhaclich)" json:"time_nhaclich"`
	TypeNhaclich *int `gorm:"column:type_nhaclich;default:1;comment:Loại thời gian nhắc lịch: 1=giờ, 2=ngày" json:"type_nhaclich"`
	CreatedBy    *int ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy    *int ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (CustomerCategory) TableName() string { return "customer_category" }

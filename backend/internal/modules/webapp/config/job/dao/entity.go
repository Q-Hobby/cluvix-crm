package dao

import "erpCluvix/internal/models"

type Job struct {
	models.SimpleModel
	Name      string ` gorm:"index;size:255;not null;comment:Tên nghề nghiệp" json:"name"`
	Desc      string ` gorm:"comment:Mô tả nghề nghiệp" json:"desc"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
}

func (Job) TableName() string { return "job" }

package dao

import "erpCluvix/internal/models"

type Setting struct {
	models.SimpleModel
	Name      string ` gorm:"size:255" json:"name"`
	Key       string ` gorm:"uniqueIndex;size:64;comment:Từ khóa" json:"key"`
	Value     string ` gorm:"size:1000;comment:Giá trị" json:"value"`
	CreatedBy *int   ` gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy *int   ` gorm:"column:updated_by;index" json:"updated_by"`
}

func (Setting) TableName() string { return "setting" }

type ConfigSystems struct {
	ID      uint   `gorm:"primaryKey" json:"id" form:"-"`
	Content string ` gorm:"comment:Giá trị" json:"content"`
}

func (ConfigSystems) TableName() string { return "config_systems" }

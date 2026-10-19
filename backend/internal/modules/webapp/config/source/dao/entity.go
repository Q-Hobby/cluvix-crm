package dao

import "erpCluvix/internal/models"

type Source struct {
	models.SimpleModel
	Name      string ` gorm:"uniqueIndex;size:255" json:"name"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder"`
	// Story-39 (E6/D15): bỏ field JSON companyId (cột NULL được ở schema cũ ⇒ dòng mới NULL =
	// dùng chung); cột bỏ hẳn ở schema mới (story-25/62).
	CreatedBy *int ` gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy *int ` gorm:"column:updated_by;index" json:"updated_by"`
}

func (Source) TableName() string { return "source" }

package dao

import "erpCluvix/internal/models"

type NoteTemplate struct {
	models.SimpleModel
	TypeId    *int   ` gorm:"column:type_id;comment:Loại mẫu template" json:"type_id"`
	Name      string ` gorm:"size:255;not null;comment:Tên template" json:"name"`
	Content   string ` gorm:"comment:Nội dung" json:"content"`
	SortOrder *int   ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	CreatedBy *int   ` gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy *int   ` gorm:"column:updated_by;index" json:"updated_by"`
}

func (NoteTemplate) TableName() string { return "note_template" }

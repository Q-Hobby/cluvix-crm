package dao

import (
	"html"
	"time"

	"erpCluvix/pkg/helpers"

	"gorm.io/gorm"
)

type Note struct {
	ID         uint   `gorm:"primaryKey" json:"id" form:"-"`
	Note       string `json:"note"`
	CustomerId *int   `gorm:"column:customer_id;index" json:"customer_id"`
	Pinned     *int   `json:"pinned"`
	PinnedAt   *int64 `gorm:"column:pinned_at" json:"pinned_at"`
	PinnedBy   *int   `gorm:"column:pinned_by;index" json:"pinned_by"`
	CreatedAt  *int64 `gorm:"column:created_at;autoCreateTime:false" json:"created_at"`
	CreatedBy  *int   `gorm:"column:created_by;index" json:"created_by"`
}

func (Note) TableName() string { return "note" }

func (m *Note) BeforeCreate(tx *gorm.DB) error {
	if m.CreatedAt == nil {
		m.CreatedAt = new(time.Now().Unix())
	}
	m.Note = helpers.MbUcfirst(html.UnescapeString(m.Note))
	return nil
}

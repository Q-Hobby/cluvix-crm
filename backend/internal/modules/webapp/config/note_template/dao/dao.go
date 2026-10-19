package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[NoteTemplate]
}

type Dao struct {
	*repository.BaseRepo[NoteTemplate]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[NoteTemplate](db, "note_template")}
}

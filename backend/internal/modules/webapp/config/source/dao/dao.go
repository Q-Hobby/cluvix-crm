package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[Source]
}

type Dao struct {
	*repository.BaseRepo[Source]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[Source](db, "source")}
}

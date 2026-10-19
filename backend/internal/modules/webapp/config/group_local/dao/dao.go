package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[GroupLocal]
}

type Dao struct {
	*repository.BaseRepo[GroupLocal]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[GroupLocal](db, "group_local")}
}

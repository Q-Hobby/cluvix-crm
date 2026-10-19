package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[GroupSource]
}

type Dao struct {
	*repository.BaseRepo[GroupSource]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[GroupSource](db, "group_source")}
}

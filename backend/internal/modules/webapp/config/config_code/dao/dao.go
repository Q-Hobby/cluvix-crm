package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[ConfigCode]
}

type Dao struct {
	*repository.BaseRepo[ConfigCode]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[ConfigCode](db, "config_code")}
}

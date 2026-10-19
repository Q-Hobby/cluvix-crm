package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[Service]
}

type Dao struct {
	*repository.BaseRepo[Service]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[Service](db, "service")}
}

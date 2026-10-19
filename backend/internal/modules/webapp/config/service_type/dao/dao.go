package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[ServiceType]
}

type Dao struct {
	*repository.BaseRepo[ServiceType]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[ServiceType](db, "service_type")}
}

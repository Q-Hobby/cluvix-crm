package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[LocationProvince]
}

type Dao struct {
	*repository.BaseRepo[LocationProvince]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[LocationProvince](db, "location_province")}
}

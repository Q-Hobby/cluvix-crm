package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[LocationCountry]
}

type Dao struct {
	*repository.BaseRepo[LocationCountry]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[LocationCountry](db, "location_country")}
}

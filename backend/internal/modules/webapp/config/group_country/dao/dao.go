package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[GroupCountry]
}

type Dao struct {
	*repository.BaseRepo[GroupCountry]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[GroupCountry](db, "group_country")}
}

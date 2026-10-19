package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[Ethnicity]
}

type Dao struct {
	*repository.BaseRepo[Ethnicity]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[Ethnicity](db, "ethnicity")}
}

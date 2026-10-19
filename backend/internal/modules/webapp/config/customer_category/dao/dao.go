package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[CustomerCategory]
}

type Dao struct {
	*repository.BaseRepo[CustomerCategory]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[CustomerCategory](db, "customer_category")}
}

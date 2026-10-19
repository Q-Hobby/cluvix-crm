package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[CustomerRanks]
}

type Dao struct {
	*repository.BaseRepo[CustomerRanks]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[CustomerRanks](db, "customer_ranks")}
}

package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[CustomerRankServices]
}

type Dao struct {
	*repository.BaseRepo[CustomerRankServices]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[CustomerRankServices](db, "customer_rank_services")}
}

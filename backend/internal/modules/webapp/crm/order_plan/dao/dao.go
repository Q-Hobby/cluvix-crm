package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[OrderPlan]
}

type Dao struct {
	*repository.BaseRepo[OrderPlan]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[OrderPlan](db, "order_plan")}
}

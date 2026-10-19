package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[OrderServiceUsed]
}

type Dao struct {
	*repository.BaseRepo[OrderServiceUsed]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[OrderServiceUsed](db, "order_service_used")}
}

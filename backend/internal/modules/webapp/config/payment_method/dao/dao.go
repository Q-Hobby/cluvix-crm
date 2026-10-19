package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[PaymentMethod]
}

type Dao struct {
	*repository.BaseRepo[PaymentMethod]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[PaymentMethod](db, "payment_method")}
}

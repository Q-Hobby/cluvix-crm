package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[BankAccount]
}

type Dao struct {
	*repository.BaseRepo[BankAccount]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[BankAccount](db, "bank_accounts")}
}

package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[UserOption]
}

type Dao struct {
	*repository.BaseRepo[UserOption]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[UserOption](db, "user_option")}
}

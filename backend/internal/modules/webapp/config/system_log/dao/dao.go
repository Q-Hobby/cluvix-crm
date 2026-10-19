package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[SystemLog]
}

type Dao struct {
	*repository.BaseRepo[SystemLog]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[SystemLog](db, "system_log")}
}

package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[ControlFieldInTable]
}

type Dao struct {
	*repository.BaseRepo[ControlFieldInTable]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[ControlFieldInTable](db, "control_field_in_table")}
}

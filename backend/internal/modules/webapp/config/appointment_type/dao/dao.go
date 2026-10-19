package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[AppointmentType]
}

type Dao struct {
	*repository.BaseRepo[AppointmentType]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[AppointmentType](db, "appointment_type")}
}

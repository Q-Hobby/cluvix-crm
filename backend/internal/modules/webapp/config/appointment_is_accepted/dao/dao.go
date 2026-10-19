package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[AppointmentIsAccepted]
}

type Dao struct {
	*repository.BaseRepo[AppointmentIsAccepted]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[AppointmentIsAccepted](db, "appointment_is_accepted")}
}

package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[AppointmentNotBookedReasons]
}

type Dao struct {
	*repository.BaseRepo[AppointmentNotBookedReasons]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[AppointmentNotBookedReasons](db, "appointment_not_booked_reasons")}
}

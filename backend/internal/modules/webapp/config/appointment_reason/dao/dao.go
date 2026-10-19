package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[AppointmentReason]
}

type Dao struct {
	*repository.BaseRepo[AppointmentReason]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[AppointmentReason](db, "appointment_reason")}
}

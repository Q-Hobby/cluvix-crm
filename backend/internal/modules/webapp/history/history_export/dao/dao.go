package dao

import (
	"erpCluvix/pkg/repository"

	"gorm.io/gorm"
)

type IDao interface {
	repository.BaseInterface[HistoryExport]
}

type Dao struct {
	*repository.BaseRepo[HistoryExport]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
	return &Dao{repository.New[HistoryExport](db, "history_export")}
}

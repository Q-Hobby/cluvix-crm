package handler

import "gorm.io/gorm"

type Deps struct{ DB *gorm.DB }
type Handler struct{ DB *gorm.DB }

func New(d Deps) *Handler { return &Handler{DB: d.DB} }

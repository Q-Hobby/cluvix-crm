// Package handler — Reminder CRUD.
//
// Plan B.2 (2026-05-16): Migrate reminder ra layered module. Handler-only — table
// có company_id (tenant-scoped), không có service file. Logic gồm CRUD +
// optimistic lock (B.3) + code generation (codegen.Generate qua config_code).
package handler

import "gorm.io/gorm"

type Deps struct {
	DB *gorm.DB
}

type Handler struct {
	DB *gorm.DB
}

func New(d Deps) *Handler { return &Handler{DB: d.DB} }

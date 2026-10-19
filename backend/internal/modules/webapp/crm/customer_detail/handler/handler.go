// Package handler — Cross-resource view aggregator (NOT a single resource module).
//
// Plan B.2 (2026-05-16) — last CRM migration: 6 endpoints aggregate data từ
// appointment / payment / treatment / order_service_used cho
// customer detail view. Vì cross-cutting, route đăng ký rải rác qua nhiều
// parent groups (xem route.go).
package handler

import (
	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct {
	DB *gorm.DB
}

func New(d Deps) *Handler { return &Handler{DB: d.DB} }

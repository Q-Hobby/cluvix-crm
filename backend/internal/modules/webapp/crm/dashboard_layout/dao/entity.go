package dao

import (
	"gorm.io/datatypes"

	"erpCluvix/internal/models"
)

// DashboardLayout — bố cục dashboard tuỳ biến. 1 dòng / (user, dashboard); `UserID` nil = mặc định thương
// hiệu (1 DB = 1 thương hiệu, KHÔNG có company_id). UNIQUE (COALESCE(user_id,0), dashboard_key).
// `models.BaseModelNoStatus` lo created_by/updated_by (hook blame) + created_at/updated_at (Unix).
type DashboardLayout struct {
	models.BaseModelNoStatus
	UserID       *uint          `gorm:"column:user_id" json:"user_id"`
	DashboardKey string         `gorm:"column:dashboard_key;not null" json:"dashboard_key"`
	Config       datatypes.JSON `gorm:"column:config;type:json;not null" json:"config"`
}

func (DashboardLayout) TableName() string { return "dashboard_layout" }

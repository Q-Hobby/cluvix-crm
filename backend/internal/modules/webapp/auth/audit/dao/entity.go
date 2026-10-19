package dao

import "gorm.io/datatypes"

// SecurityAuditLog — 1 dòng nhật ký bảo mật (persist từ pkg/seclog).
// Auth-level, toàn DB (1 doanh nghiệp/DB).
type SecurityAuditLog struct {
	ID        uint64         `gorm:"primaryKey"                         json:"id" form:"-"`
	Event     string         `gorm:"size:40;not null;index:idx_audit_event,priority:1" json:"event"`
	UserID    *uint          `gorm:"column:user_id"                     json:"user_id"`
	IP        string         `gorm:"column:ip;size:45"                  json:"ip"`
	RequestID string         `gorm:"column:request_id;size:32"          json:"request_id"`
	Username  string         `gorm:"size:255"                           json:"username"`
	Detail    datatypes.JSON `gorm:"column:detail"                      json:"detail"`
	CreatedAt int64          `gorm:"column:created_at;not null"         json:"created_at"`
}

func (SecurityAuditLog) TableName() string { return "security_audit_log" }

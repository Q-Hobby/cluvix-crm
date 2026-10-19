package dao

// UserDevice — thiết bị đã login của user (device binding, auth-level).
// KHÔNG có company_id/branch_id: gắn trực tiếp user (global entity), cùng nhóm bảng auth.
// Status dùng define.UserDeviceTrusted/Pending/Revoked. Cột này là chỗ sẵn cho
// flow "admin duyệt thiết bị" và 2FA/WebAuthn per-device về sau.
type UserDevice struct {
	ID          uint    `gorm:"primaryKey"                                        json:"id" form:"-"`
	UserID      uint    `gorm:"column:user_id;not null;index:idx_user_device_user" json:"user_id"`
	Fingerprint string  `gorm:"size:64;not null"                                  json:"-"`
	DeviceName  *string `gorm:"column:device_name;size:255"                       json:"device_name"`
	Status      int     `gorm:"not null;default:1"                                json:"status"`
	LastIP      *string `gorm:"column:last_ip;size:45"                            json:"last_ip"`
	LastSeen    *int64  `gorm:"column:last_seen"                                  json:"last_seen"`
	CreatedAt   *int64  `gorm:"column:created_at;autoCreateTime:false"            json:"created_at"`
	UpdatedAt   *int64  `gorm:"column:updated_at;autoUpdateTime:false"            json:"updated_at"`
}

func (UserDevice) TableName() string { return "user_device" }

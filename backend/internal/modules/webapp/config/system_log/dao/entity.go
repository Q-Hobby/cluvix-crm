package dao

type SystemLog struct {
	ID       uint   `gorm:"primaryKey" json:"id" form:"-"`
	Level    *int   ` gorm:"index" json:"level"`
	Category string ` gorm:"index;size:255" json:"category"`
	LogTime  *int64 ` gorm:"column:log_time" json:"log_time"`
	Prefix   string ` json:"prefix"`
	Message  string ` json:"message"`
}

func (SystemLog) TableName() string { return "system_log" }

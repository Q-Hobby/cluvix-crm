package dao

type HistoryExport struct {
	ID             uint   `gorm:"primaryKey" json:"id" form:"-"`
	PositionExport string ` gorm:"column:positionExport;size:500;comment:Vị trí export" json:"positionExport"`
	QuantityRecord *int   ` gorm:"column:quantityRecord;comment:Số lượng dữ liệu" json:"quantityRecord"`
	IpAddress      string ` gorm:"column:ip_address;size:25;comment:Địa chỉ IP export" json:"ip_address"`
	UserAgent      string ` gorm:"column:userAgent;comment:Thông tin máy user export" json:"userAgent"`
	CreatedBy      *int   ` gorm:"column:createdBy;index;comment:Người export" json:"createdBy"`
	CreatedAt      *int   ` gorm:"column:createdAt;comment:Thời gian export" json:"createdAt"`
}

func (HistoryExport) TableName() string { return "history_export" }

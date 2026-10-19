package dao

type ConfigCode struct {
	ID                uint   `gorm:"primaryKey" json:"id" form:"-"`
	BranchId          int    `gorm:"column:branch_id;not null;default:1;comment:FK → branch" json:"branch_id"`
	Name              string `gorm:"size:255;comment:Tên bảng dữ liệu" json:"name"`
	ParentTable       string `gorm:"column:parent_table;size:100;comment:Tên bảng DB tương ứng" json:"parent_table"`
	InfixCode         string `gorm:"column:infix_code;size:50;comment:Trung tố mã code (LH, KH, DH...)" json:"infix_code"`
	IncludeBranchCode bool   `gorm:"column:include_branch_code;not null;default:1;comment:Có ghép mã chi nhánh vào mã sinh tự động không" json:"include_branch_code"`
	MonthlyReset      bool   `gorm:"column:monthly_reset;not null;default:1;comment:Serial có reset theo tháng (kèm YYMM trong mã) không" json:"monthly_reset"`
	SerialLength      int    `gorm:"column:serial_length;not null;default:2;comment:Độ dài tối thiểu số serial, tự nới rộng khi tràn" json:"serial_length"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
	CreatedBy         *int   `gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy         *int   `gorm:"column:updated_by;index" json:"updated_by"`
}

func (ConfigCode) TableName() string { return "config_code" }

type ConfigCodeMonthly struct {
	ID           uint `gorm:"primaryKey" json:"id" form:"-"`
	ConfigCodeId int  `gorm:"column:config_code_id;index;not null;comment:FK → config_code" json:"config_code_id"`
	Year         int  `gorm:"not null;comment:Năm" json:"year"`
	Month        int  `gorm:"not null;comment:Tháng" json:"month"`
	LastNumber   int  `gorm:"column:last_number;not null;default:0;comment:Số serial cuối cùng trong tháng" json:"last_number"`
}

func (ConfigCodeMonthly) TableName() string { return "config_code_monthly" }

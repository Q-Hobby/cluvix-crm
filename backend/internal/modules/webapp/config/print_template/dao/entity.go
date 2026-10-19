package dao

// PrintTemplate — mẫu in HTML theo chi nhánh (branch_id = 0: mẫu chung).
type PrintTemplate struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"                        json:"id" form:"-"`
	BranchId     uint   `gorm:"column:branch_id;index;not null;default:0"       json:"branch_id"`
	TemplateType string `gorm:"column:template_type;size:50;not null"           json:"template_type"`
	Name         string `gorm:"column:name;size:255;not null"                   json:"name"`
	Description  string `gorm:"column:description;type:text"                   json:"description"`
	TemplateHtml string `gorm:"column:template_html;type:longtext;not null"     json:"template_html"`
	IsDefault    int    `gorm:"column:is_default;not null;default:0"            json:"is_default"`
	IsActive     int    `gorm:"column:is_active;not null;default:1"             json:"is_active"`
	CreatedAt    int64  `gorm:"column:created_at;not null;autoCreateTime:unix"  json:"created_at"`
	UpdatedAt    int64  `gorm:"column:updated_at;not null;autoUpdateTime:unix"  json:"updated_at"`
	CreatedBy    uint   `gorm:"column:created_by;not null;default:0"            json:"created_by"`
	UpdatedBy    uint   `gorm:"column:updated_by;not null;default:0"            json:"updated_by"`
}

func (PrintTemplate) TableName() string { return "print_templates" }

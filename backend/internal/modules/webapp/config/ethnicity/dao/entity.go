package dao

type Ethnicity struct {
	ID            uint   `gorm:"primaryKey" json:"id" form:"-"`
	Name          string `gorm:"size:100;not null;comment:Tên dân tộc" json:"name"`
	CodeEthnicity string `gorm:"column:codeEthnicity;index;size:20;comment:Mã dân tộc" json:"codeEthnicity"`
	SortOrder     *int   `gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	Status        *int   `gorm:"default:1" json:"status"`
	CreatedAt     int64  `json:"created_at"`
}

func (Ethnicity) TableName() string { return "ethnicity" }

package dao

type Branch struct {
	ID                    uint   `gorm:"primaryKey" json:"id" form:"-"`
	Name                  string `gorm:"uniqueIndex;size:255" json:"name" form:"name"`
	Code                  string `gorm:"uniqueIndex;size:25" json:"code" form:"code"`
	Address               string `gorm:"size:255" json:"address" form:"address"`
	Hotline               string `gorm:"size:25" json:"hotline" form:"hotline"`
	Email                 string `gorm:"size:50" json:"email" form:"email"`
	StatusOpenAppointment *int   `gorm:"column:statusOpenAppointment;default:1;comment:1=nhận đặt hẹn, 0=không nhận" json:"statusOpenAppointment" form:"statusOpenAppointment"`
	Status                *int   `gorm:"default:1;comment:1=hiện, 0=ẩn" json:"status" form:"status"`
	SortOrder             *int   `gorm:"column:SortOrder;comment:Vị trí hiển thị" json:"SortOrder" form:"SortOrder"`
	CreatedAt             int64  `json:"created_at"`
	UpdatedAt             int64  `json:"updated_at"`
	CreatedBy             *int   `gorm:"column:created_by;index" json:"created_by"`
	UpdatedBy             *int   `gorm:"column:updated_by;index" json:"updated_by"`
}

func (Branch) TableName() string { return "branch" }

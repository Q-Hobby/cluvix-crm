package dao

import (
	"erpCluvix/internal/models"
)

// HrDepartment — danh mục phòng ban (D8, giữ dù HR trim — Telesale scope + CRM leadscope
// dựa vào phòng ban này qua `pkg/define` HrDepartment* id). Entity dời từ
// `internal/modules/webapp/hr/config/dao` (story-15, module `hr/config` không phải RBAC).
type HrDepartment struct {
	models.SimpleModel
	Name       string ` gorm:"uniqueIndex;size:255;not null;comment:Tên phòng ban" json:"name"`
	Desc       string ` gorm:"comment:Nội dung chi tiết" json:"desc"`
	HeadUserId *int   ` gorm:"column:head_user_id;index;comment:Trưởng phòng ban (FK user)" json:"head_user_id"`
	SortOrder  *int   ` gorm:"column:SortOrder;comment:Vị trí" json:"SortOrder"`
	CreatedBy  *int   ` gorm:"column:created_by;index;comment:Được tạo bởi ai" json:"created_by"`
	UpdatedBy  *int   ` gorm:"column:updated_by;index;comment:Người cập nhật gần đây nhất" json:"updated_by"`
}

func (HrDepartment) TableName() string { return "hr_department" }

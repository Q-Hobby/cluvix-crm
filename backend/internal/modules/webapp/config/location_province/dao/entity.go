package dao

import "erpCluvix/internal/models"

type LocationProvince struct {
	models.SimpleModel
	CodeProvince  string ` gorm:"column:codeProvince;size:10" json:"codeProvince"`
	Name          string ` gorm:"size:255" json:"name"`
	Slug          string ` gorm:"index;size:255" json:"slug"`
	Type          *int   ` gorm:"column:Type" json:"Type"`
	TelephoneCode *int   ` gorm:"column:TelephoneCode" json:"TelephoneCode"`
	ZipCode       string ` gorm:"column:ZipCode;size:255" json:"ZipCode"`
	CountryId     *int   ` gorm:"column:CountryId;index" json:"CountryId"`
	CountryCode   string ` gorm:"column:CountryCode;size:255" json:"CountryCode"`
	SortOrder     *int   ` gorm:"column:SortOrder" json:"SortOrder"`
	Language      string ` gorm:"index;not null;default:vi;comment:Language" json:"language"`
	IsDeleted     *int   ` gorm:"column:IsDeleted;default:0" json:"IsDeleted"`
	CreatedBy     *int   ` gorm:"column:created_by;index;default:1" json:"created_by"`
	UpdatedBy     *int   ` gorm:"column:updated_by;index;default:1" json:"updated_by"`
}

func (LocationProvince) TableName() string { return "location_province" }

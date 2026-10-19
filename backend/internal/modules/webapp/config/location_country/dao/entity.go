package dao

import "erpCluvix/internal/models"

type LocationCountry struct {
	models.SimpleModel
	CountryCode         string ` gorm:"column:CountryCode;size:255" json:"CountryCode"`
	CommonName          string ` gorm:"column:CommonName;size:255" json:"CommonName"`
	Slug                string ` gorm:"index;size:255" json:"slug"`
	FormalName          string ` gorm:"column:FormalName;size:255" json:"FormalName"`
	CountryType         string ` gorm:"column:CountryType;size:255" json:"CountryType"`
	CountrySubType      string ` gorm:"column:CountrySubType;size:255" json:"CountrySubType"`
	Sovereignty         string ` gorm:"column:Sovereignty;size:255" json:"Sovereignty"`
	Capital             string ` gorm:"column:Capital;size:255" json:"Capital"`
	CurrencyCode        string ` gorm:"column:CurrencyCode;size:255" json:"CurrencyCode"`
	CurrencyName        string ` gorm:"column:CurrencyName;size:255" json:"CurrencyName"`
	TelephoneCode       string ` gorm:"column:TelephoneCode;size:255" json:"TelephoneCode"`
	CountryCode3        string ` gorm:"column:CountryCode3;size:255" json:"CountryCode3"`
	CountryNumber       string ` gorm:"column:CountryNumber;size:255" json:"CountryNumber"`
	InternetCountryCode string ` gorm:"column:InternetCountryCode;size:255" json:"InternetCountryCode"`
	SortOrder           *int   ` gorm:"column:SortOrder" json:"SortOrder"`
	Language            string ` gorm:"index;not null;default:vi;comment:Language" json:"language"`
	Flags               string ` gorm:"column:Flags;size:255" json:"Flags"`
	IsDeleted           *int   ` gorm:"column:IsDeleted;default:0" json:"IsDeleted"`
	CreatedBy           *int   ` gorm:"column:created_by;index;default:1" json:"created_by"`
	UpdatedBy           *int   ` gorm:"column:updated_by;index;default:1" json:"updated_by"`
}

func (LocationCountry) TableName() string { return "location_country" }

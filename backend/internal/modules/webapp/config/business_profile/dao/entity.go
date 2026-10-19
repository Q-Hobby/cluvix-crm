// Package dao — entity cho bảng `business_profile` (D22/D23, story-62/63).
//
// Bảng 1 dòng, KHÔNG có `company_id`/`branch_id` (E6 — CRM standalone chỉ có 1 thương hiệu/1 DB,
// DB-per-customer). `pkg/brand` là nơi đọc DUY NHẤT cho mục đích nghiệp vụ (timezone/tiền tệ/2FA/IP
// allowlist/mã tiền tố) — module này chỉ phục vụ màn "Hồ sơ doanh nghiệp" (GET/PUT 1 dòng), không
// list/create/delete, không id trên path.
package dao

import (
	"erpCluvix/internal/models"

	"gorm.io/datatypes"
)

// BusinessProfile — GORM struct thuần, không method/business logic (Hard Rule entity layer).
type BusinessProfile struct {
	models.BaseModelNoStatus // id, created_by, updated_by, created_at, updated_at (không có status)

	Name            string         `gorm:"column:name" json:"name"`
	Code            string         `gorm:"column:code" json:"code"`
	Hotline         string         `gorm:"column:hotline" json:"hotline"`
	Address         string         `gorm:"column:address" json:"address"`
	Email           string         `gorm:"column:email" json:"email"`
	Website         string         `gorm:"column:website" json:"website"`
	LogoImage       string         `gorm:"column:logo_image" json:"logo_image"`
	TaxCode         string         `gorm:"column:tax_code" json:"tax_code"`
	Timezone        string         `gorm:"column:timezone" json:"timezone"`
	Currency        string         `gorm:"column:currency" json:"currency"`
	CurrencySymbol  string         `gorm:"column:currency_symbol" json:"currency_symbol"`
	CurrencyLocale  string         `gorm:"column:currency_locale" json:"currency_locale"`
	WeeklyRestDays  string         `gorm:"column:weekly_rest_days" json:"weekly_rest_days"`
	AllowedIps      datatypes.JSON `gorm:"column:allowed_ips" json:"allowed_ips"`
	Require2FA      int            `gorm:"column:require_2fa" json:"require_2fa"`
	Require2FARoles datatypes.JSON `gorm:"column:require_2fa_roles" json:"require_2fa_roles"`
	TotpTrustDays   int            `gorm:"column:totp_trust_days" json:"totp_trust_days"`
}

func (BusinessProfile) TableName() string { return "business_profile" }

// selectCols — explicit SELECT dùng cho GET (Hard Rule: KHÔNG SELECT *).
var SelectCols = []string{
	"id", "name", "code", "hotline", "address", "email", "website",
	"logo_image", "tax_code", "timezone", "currency", "currency_symbol",
	"currency_locale", "weekly_rest_days", "allowed_ips", "require_2fa", "require_2fa_roles",
	"totp_trust_days", "created_at", "updated_at", "created_by", "updated_by",
}

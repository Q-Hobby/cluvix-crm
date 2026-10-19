// Package dto — allowlist payload `POST /config/onboarding/bootstrap`.
//
// Cố ý KHÔNG bind thẳng vào bảng `business_profile`/entity `branch` (mass-assignment): field không có
// ở đây (`id`, `status`, `allowed_ips`, `require_2fa`, `require_2fa_roles`, `totp_trust_days`,
// `SortOrder`, branch `id`) bị `json.Unmarshal` bỏ qua im lặng — đúng hành vi mong muốn.
package dto

// CompanyInput — form field `company` (JSON string) = hồ sơ doanh nghiệp. Tên type/json tag giữ
// nguyên hợp đồng FE cũ (story-63/43 đổi tên).
type CompanyInput struct {
	Name           string `json:"name"`
	Code           string `json:"code"`
	BusinessTypeId int    `json:"business_type_id"`
	Hotline        string `json:"hotline"`
	Address        string `json:"address"`
	Email          string `json:"email"`
	Website        string `json:"website"`
	Timezone       string `json:"timezone"`
	Currency       string `json:"currency"`
	CurrencySymbol string `json:"currency_symbol"`
	CurrencyLocale string `json:"currency_locale"`
}

// BranchInput — form field `branch` (JSON string, bắt buộc).
type BranchInput struct {
	Name    string `json:"name"`
	Code    string `json:"code"`
	Address string `json:"address"`
	Hotline string `json:"hotline"`
	Email   string `json:"email"`
}

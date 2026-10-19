// Package brand — nguồn DUY NHẤT của thông tin thương hiệu + cấu hình chung toàn hệ thống
// (tên, logo, timezone, tiền tệ, IP allowlist, 2FA, trust-days, code tiền tố mã tự sinh).
//
// Bối cảnh (E6/D15, D22): CRM standalone chỉ có 1 thương hiệu / 1 DB (DB-per-customer). Code KHÔNG
// còn đọc "company đang active" từ session — mọi nơi cần thông tin thương hiệu gọi package này.
//
// Nguồn: bảng 1 dòng `business_profile` (D22, story-62 — trước đó đọc bảng thương hiệu cũ).
//
// Cache in-memory an toàn đồng thời, TTL `cacheTTL`; `Invalidate()` sau khi sửa hồ sơ. Lỗi query
// KHÔNG được cache (một lỗi DB thoáng qua không được "đóng băng" giá trị mặc định — cùng bài học
// `tz.Location`, story-09 notification-debt-cleanup). "Chưa có dòng" cũng KHÔNG cache: DB mới chưa
// onboarding sẽ có dòng ngay sau bước onboarding, không được bắt người dùng chờ hết TTL.
package brand

import (
	"errors"
	"sync"
	"time"

	"erpCluvix/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Giá trị mặc định khi chưa có dòng hồ sơ (hoặc cột rỗng) — khớp DEFAULT của bảng `business_profile`.
const (
	DefaultTimezone       = "Asia/Ho_Chi_Minh"
	DefaultCurrency       = "VND"
	DefaultCurrencySymbol = "₫"
	DefaultCurrencyLocale = "vi-VN"
	DefaultTotpTrustDays  = 30
)

// cacheTTL — trần độ trễ khi hồ sơ bị sửa ở TIẾN TRÌNH KHÁC (Invalidate chỉ xoá cache cục bộ).
const cacheTTL = 5 * time.Minute

// warnEvery — trần tần suất log "đọc hồ sơ thất bại" (đường nóng: mọi LocFromContext).
const warnEvery = time.Minute

// ErrNoProfile — DB chưa có dòng hồ sơ thương hiệu (chưa onboarding).
var ErrNoProfile = errors.New("brand: chưa có dòng hồ sơ doanh nghiệp (bảng business_profile rỗng)")

// Info — hồ sơ thương hiệu. Field chuỗi rỗng = chưa đặt (trừ các field đã điền mặc định ở dưới).
type Info struct {
	Name            string
	Code            string
	Hotline         string
	Address         string
	Email           string
	Website         string
	LogoImage       string
	TaxCode         string
	Timezone        string // luôn có giá trị (mặc định DefaultTimezone)
	Currency        string // luôn có giá trị (mặc định DefaultCurrency)
	CurrencySymbol  string // luôn có giá trị (mặc định DefaultCurrencySymbol)
	CurrencyLocale  string // luôn có giá trị (mặc định DefaultCurrencyLocale)
	WeeklyRestDays  string // CSV weekday 0=CN..6=T7, rỗng = mặc định Chủ nhật
	AllowedIPs      datatypes.JSON
	Require2FA      bool
	Require2FARoles datatypes.JSON
	TotpTrustDays   int
}

// row — hình dạng đọc từ bảng `business_profile`. KHÔNG `SELECT *`.
type row struct {
	Name            *string        `gorm:"column:name"`
	Code            *string        `gorm:"column:code"`
	Hotline         *string        `gorm:"column:hotline"`
	Address         *string        `gorm:"column:address"`
	Email           *string        `gorm:"column:email"`
	Website         *string        `gorm:"column:website"`
	LogoImage       *string        `gorm:"column:logo_image"`
	TaxCode         *string        `gorm:"column:tax_code"`
	Timezone        *string        `gorm:"column:timezone"`
	Currency        *string        `gorm:"column:currency"`
	CurrencySymbol  *string        `gorm:"column:currency_symbol"`
	CurrencyLocale  *string        `gorm:"column:currency_locale"`
	WeeklyRestDays  *string        `gorm:"column:weekly_rest_days"`
	AllowedIPs      datatypes.JSON `gorm:"column:allowed_ips"`
	Require2FA      *int           `gorm:"column:require_2fa"`
	Require2FARoles datatypes.JSON `gorm:"column:require_2fa_roles"`
	TotpTrustDays   *int           `gorm:"column:totp_trust_days"`
}

const selectCols = "name, code, hotline, address, email, website, " +
	"logo_image, tax_code, timezone, currency, currency_symbol, currency_locale, weekly_rest_days, " +
	"allowed_ips, require_2fa, require_2fa_roles, totp_trust_days"

type entry struct {
	info     Info
	loc      *time.Location
	loadedAt time.Time
}

var (
	mu     sync.RWMutex
	cached *entry

	warnMu   sync.Mutex
	warnLast time.Time
)

// Defaults — hồ sơ mặc định khi chưa có dòng.
func Defaults() Info {
	return Info{
		Timezone:       DefaultTimezone,
		Currency:       DefaultCurrency,
		CurrencySymbol: DefaultCurrencySymbol,
		CurrencyLocale: DefaultCurrencyLocale,
		TotpTrustDays:  DefaultTotpTrustDays,
	}
}

// load trả entry từ cache (còn hạn) hoặc đọc DB. Lỗi / chưa có dòng ⇒ KHÔNG ghi cache.
func load(db *gorm.DB) (*entry, error) {
	mu.RLock()
	e := cached
	mu.RUnlock()
	if e != nil && time.Since(e.loadedAt) < cacheTTL {
		return e, nil
	}

	// NewDB: gọi được từ bên trong hook GORM / transaction mà không kế thừa Statement của model
	// đang thao tác.
	var rows []row
	// Bảng 1 dòng; nếu lỡ có nhiều dòng ⇒ lấy id nhỏ nhất cho tất định — không bao giờ phụ thuộc
	// thứ tự vật lý.
	if err := db.Session(&gorm.Session{NewDB: true}).Table("business_profile").Select(selectCols).
		Order("id ASC").Limit(1).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoProfile
	}
	e = &entry{info: toInfo(rows[0]), loadedAt: time.Now()}
	e.loc = loadLocation(e.info.Timezone)

	mu.Lock()
	cached = e
	mu.Unlock()
	return e, nil
}

func toInfo(r row) Info {
	s := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	orDefault := func(p *string, d string) string {
		if p == nil || *p == "" {
			return d
		}
		return *p
	}
	in := Info{
		Name:            s(r.Name),
		Code:            s(r.Code),
		Hotline:         s(r.Hotline),
		Address:         s(r.Address),
		Email:           s(r.Email),
		Website:         s(r.Website),
		LogoImage:       s(r.LogoImage),
		TaxCode:         s(r.TaxCode),
		Timezone:        orDefault(r.Timezone, DefaultTimezone),
		Currency:        orDefault(r.Currency, DefaultCurrency),
		CurrencySymbol:  orDefault(r.CurrencySymbol, DefaultCurrencySymbol),
		CurrencyLocale:  orDefault(r.CurrencyLocale, DefaultCurrencyLocale),
		WeeklyRestDays:  s(r.WeeklyRestDays),
		AllowedIPs:      r.AllowedIPs,
		Require2FA:      r.Require2FA != nil && *r.Require2FA == 1,
		Require2FARoles: r.Require2FARoles,
		TotpTrustDays:   DefaultTotpTrustDays,
	}
	// 0 là giá trị HỢP LỆ ("luôn hỏi 2FA") — chỉ NULL mới rơi về mặc định.
	if r.TotpTrustDays != nil {
		in.TotpTrustDays = *r.TotpTrustDays
	}
	return in
}

func loadLocation(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	loc, _ := time.LoadLocation(DefaultTimezone)
	return loc
}

func warnLookupFailed(err error) {
	warnMu.Lock()
	now := time.Now()
	if !warnLast.IsZero() && now.Sub(warnLast) < warnEvery {
		warnMu.Unlock()
		return
	}
	warnLast = now
	warnMu.Unlock()
	if logger.L == nil {
		return
	}
	logger.L.Warn("brand: đọc hồ sơ thương hiệu thất bại — dùng mặc định cho lời gọi này, KHÔNG cache",
		zap.Error(err))
}

// Profile trả hồ sơ thương hiệu. Chưa có dòng ⇒ Defaults(). Lỗi DB ⇒ Defaults() cho ĐÚNG lời gọi
// này (thoái hoá an toàn, có log cảnh báo tiết chế), không cache.
func Profile(db *gorm.DB) Info {
	e, err := load(db)
	if err != nil {
		if !errors.Is(err, ErrNoProfile) {
			warnLookupFailed(err)
		}
		return Defaults()
	}
	return e.info
}

// ProfileStrict — như Profile nhưng TRẢ lỗi DB thay vì thoái hoá về Defaults(). Dùng cho CHÍNH SÁCH
// BẢO MẬT (2FA bắt buộc, IP allowlist — story-58): Defaults() nghĩa là "không bắt 2FA, không giới hạn
// IP", nên một lỗi DB thoáng qua mà rơi về Defaults() là tắt policy ÂM THẦM (fail-open). Caller PHẢI
// từ chối khi err != nil. Chưa có dòng hồ sơ (DB chưa onboarding) ⇒ (Defaults(), nil): không có hồ
// sơ thì không có policy (giữ hành vi cũ).
func ProfileStrict(db *gorm.DB) (Info, error) {
	e, err := load(db)
	if err != nil {
		if errors.Is(err, ErrNoProfile) {
			return Defaults(), nil
		}
		return Info{}, err
	}
	return e.info, nil
}

// Location trả múi giờ của thương hiệu (mặc định Asia/Ho_Chi_Minh).
func Location(db *gorm.DB) *time.Location {
	e, err := load(db)
	if err != nil {
		if !errors.Is(err, ErrNoProfile) {
			warnLookupFailed(err)
		}
		return loadLocation(DefaultTimezone)
	}
	return e.loc
}

// Code trả mã thương hiệu (tiền tố mã tự sinh — story-59). Rỗng nếu chưa đặt.
func Code(db *gorm.DB) string {
	return Profile(db).Code
}

// Invalidate xoá cache — gọi sau khi sửa / tạo dòng `business_profile`.
func Invalidate() {
	mu.Lock()
	cached = nil
	mu.Unlock()
}

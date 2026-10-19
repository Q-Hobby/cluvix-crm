// Package handler — GET/PUT `/api/config/business-profile` (story-63, D22/D23).
//
// Bảng 1 dòng, không id trên path, không list/create/delete. `pkg/brand` là nơi đọc DUY NHẤT cho
// mục đích nghiệp vụ khác (timezone/tiền tệ/2FA/IP allowlist/mã tiền tố) — handler này KHÔNG tự đọc
// bảng cho mục đích đó, chỉ phục vụ màn "Hồ sơ doanh nghiệp".
package handler

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/config/business_profile/dao"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/imaging"
	"erpCluvix/pkg/optlock"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Deps struct {
	DB    *gorm.DB
	Store storage.Store
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

// loadCurrent đọc dòng duy nhất (id nhỏ nhất, cùng quy ước với pkg/brand.load) — explicit SELECT,
// KHÔNG `SELECT *`. ok=false khi bảng chưa có dòng (chưa onboarding / DB test trắng).
func (h *Handler) loadCurrent() (dao.BusinessProfile, bool, error) {
	var rows []dao.BusinessProfile
	if err := h.DB.Select(dao.SelectCols).Order("id ASC").Limit(1).Find(&rows).Error; err != nil {
		return dao.BusinessProfile{}, false, err
	}
	if len(rows) == 0 {
		return defaultProfile(), false, nil
	}
	return rows[0], true, nil
}

// defaultProfile — giá trị mặc định khi chưa có dòng, khớp `pkg/brand.Defaults()` + DEFAULT cột.
func defaultProfile() dao.BusinessProfile {
	d := brand.Defaults()
	return dao.BusinessProfile{
		Timezone:       d.Timezone,
		Currency:       d.Currency,
		CurrencySymbol: d.CurrencySymbol,
		CurrencyLocale: d.CurrencyLocale,
		TotpTrustDays:  d.TotpTrustDays,
	}
}

// GET /api/config/business-profile
func (h *Handler) GetBusinessProfile(c *gin.Context) {
	item, _, err := h.loadCurrent()
	if err != nil {
		response.ServerError(c, err)
		return
	}
	response.Data(c, h.withURL(item))
}

// withURL — bản sao để trả FE: logo (KEY) ghép thành URL công khai. KHÔNG dùng bản này để lưu.
func (h *Handler) withURL(item dao.BusinessProfile) dao.BusinessProfile {
	item.LogoImage = storage.PublicURL(h.Store, item.LogoImage)
	return item
}

// businessProfileImagePrefix — prefix upload logo (dùng chung với onboarding); cũng là phạm vi
// DUY NHẤT được xoá ảnh cũ/dọn file.
const businessProfileImagePrefix = "business_profile/"

// PUT /api/config/business-profile — upsert dòng duy nhất (chưa có dòng ⇒ tạo mới, giống ngữ nghĩa
// upsert của `onboarding/service.upsertBusinessProfile`). Optimistic lock qua `updated_at`
// (pkg/optlock — bảng đã thêm vào tập khoá ở story-63): client phải gửi `updated_at` bằng giá trị
// GET trả về gần nhất (0 nếu GET trả dòng mặc định do chưa có dòng thật).
func (h *Handler) UpdateBusinessProfile(c *gin.Context) {
	current, existed, err := h.loadCurrent()
	if err != nil {
		response.ServerError(c, err)
		return
	}

	// Ảnh đang lưu trong DB — capture TRƯỚC khi bind payload đè lên `current`. Trước đây capture SAU
	// bind ⇒ "ảnh cũ" là giá trị client gửi, upload logo mới xong là xoá luôn file client chỉ định.
	oldLogo := current.LogoImage

	var expectedUpdatedAt int64
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart") {
		if response.RejectMultipartTooLarge(c) {
			return
		}
		expectedUpdatedAt = helpers.ToInt64(c.PostForm("updated_at"))
		bindBusinessProfileForm(c, &current)
	} else {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.HandleError(c, err)
			return
		}
		if v, ok := body["updated_at"]; ok {
			expectedUpdatedAt = helpers.ToInt64(v)
		}
		bindBusinessProfileJSON(body, &current)
	}

	if existed && expectedUpdatedAt != current.UpdatedAt {
		// Client chưa gửi đúng version đang lưu (payload thiếu/sai `updated_at`) — chặn SỚM, không
		// đợi tới UPDATE mới phát hiện, để trả đúng 409 (khoá lạc quan) thay vì âm thầm ghi đè khi
		// optlock chưa kịp đăng ký (phòng thủ chiều sâu, đường chính vẫn là WHERE updated_at=? bên dưới).
		response.Conflict(c, i18n.T(c, errs.ErrVersionConflict.Error()))
		return
	}
	if !existed && expectedUpdatedAt != 0 {
		response.Conflict(c, i18n.T(c, errs.ErrVersionConflict.Error()))
		return
	}
	// Chuỗi logo_image trong payload CHỈ được là ảnh hiện tại (FE gửi lại URL GET đã trả — so sau
	// ToKey, lưu lại dạng KEY) hoặc rỗng (xoá ảnh). Key khác ⇒ 422: trước đây nhận mọi key ⇒ gửi key
	// file người khác rồi lần sau gửi rỗng là xoá được file đó. Ảnh mới chỉ đến từ logoFile. Đặt SAU
	// kiểm version: client cũ gửi URL ảnh đã bị người khác đổi phải nhận 409.
	resolved, rerr := imaging.ResolvePayloadImage(h.Store, oldLogo, current.LogoImage)
	if rerr != nil {
		response.Validate(c, i18n.T(c, rerr.Error()))
		return
	}
	current.LogoImage = resolved

	if key := validateBusinessProfile(&current); key != "" {
		response.Validate(c, i18n.T(c, key))
		return
	}

	// Upload logo (pattern imaging.HandleFormImage như onboarding) — no-op nếu không phải
	// multipart / không có file mới.
	newLogo, err := imaging.HandleFormImageRaw(c, h.Store, "logoFile", businessProfileImagePrefix, nil)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if newLogo != "" {
		current.LogoImage = newLogo
	}

	userID := middleware.GetUserID(c)
	now := time.Now().Unix()
	current.UpdatedBy = &userID

	err = h.DB.Transaction(func(tx *gorm.DB) error {
		if !existed {
			current.CreatedAt = now
			current.UpdatedAt = now
			current.CreatedBy = &userID
			return tx.Create(&current).Error
		}
		// Ghi có khoá: WHERE updated_at = expected + bump GREATEST(now, updated_at+1); 0 dòng ⇒
		// errs.ErrVersionConflict (D6/D21, cùng khuôn config/print_template).
		return optlock.Save(tx, &current, expectedUpdatedAt)
	})
	if err != nil {
		// DB vẫn trỏ ảnh cũ ⇒ chỉ dọn file request này vừa upload, không đụng ảnh cũ.
		imaging.CleanupNewOnError(h.Store, oldLogo, newLogo, businessProfileImagePrefix)
		if errors.Is(err, errs.ErrVersionConflict) {
			response.Conflict(c, i18n.T(c, err.Error()))
			return
		}
		response.ServerError(c, err)
		return
	}

	imaging.DeleteOldIfChanged(h.Store, oldLogo, current.LogoImage, businessProfileImagePrefix)

	// Sau commit: cache hồ sơ (tz/tiền tệ/2FA/IP allowlist/mã tiền tố) phải đọc lại NGAY — 2FA/IP/
	// trust-days/timezone áp dụng cho request kế tiếp (story-58 Dev Notes, story-63 AC3). Scope cache
	// theo TỪNG user ⇒ xoá của mọi user, không chỉ người sửa (policy áp cho cả hệ thống).
	brand.Invalidate()
	middleware.InvalidateAllUserScopes(h.Cache)
	dexie.BumpDexieTable(h.DB, "business_profile")

	response.Updated(c, h.withURL(current))
}

// --- Bind helpers ---

// bindBusinessProfileForm — multipart: allowlist `c.PostForm`, ngữ nghĩa partial-update (field FE
// không gửi thì giữ giá trị cũ trong `item`) — KHÔNG dùng `c.ShouldBind` (zero-out field không gửi).
func bindBusinessProfileForm(c *gin.Context, item *dao.BusinessProfile) {
	if v := c.PostForm("name"); v != "" {
		item.Name = v
	}
	if v := c.PostForm("code"); v != "" {
		item.Code = v
	}
	if v := c.PostForm("hotline"); v != "" {
		item.Hotline = v
	}
	if v := c.PostForm("address"); v != "" {
		item.Address = v
	}
	if v := c.PostForm("email"); v != "" {
		item.Email = v
	}
	if v := c.PostForm("website"); v != "" {
		item.Website = v
	}
	if v := c.PostForm("tax_code"); v != "" {
		item.TaxCode = v
	}
	if v := c.PostForm("timezone"); v != "" {
		item.Timezone = v
	}
	if v := c.PostForm("currency"); v != "" {
		item.Currency = v
	}
	if v := c.PostForm("currency_symbol"); v != "" {
		item.CurrencySymbol = v
	}
	if v := c.PostForm("currency_locale"); v != "" {
		item.CurrencyLocale = v
	}
	if v, ok := c.GetPostForm("weekly_rest_days"); ok {
		item.WeeklyRestDays = v
	}
	// allowed_ips/require_2fa_roles: FE serialize mảng thành chuỗi JSON (vd `["1.2.3.4"]`).
	if v, ok := c.GetPostForm("allowed_ips"); ok {
		if v == "" {
			item.AllowedIps = nil
		} else if json.Valid([]byte(v)) {
			item.AllowedIps = datatypes.JSON(v)
		}
	}
	if v := c.PostForm("require_2fa"); v != "" {
		item.Require2FA = int(helpers.ParseUint(v))
	}
	if v, ok := c.GetPostForm("require_2fa_roles"); ok {
		if v == "" {
			item.Require2FARoles = nil
		} else if json.Valid([]byte(v)) {
			item.Require2FARoles = datatypes.JSON(v)
		}
	}
	if v := c.PostForm("totp_trust_days"); v != "" {
		item.TotpTrustDays = int(helpers.ParseUint(v))
	}
	// FE gửi "logo_image" rỗng khi user bấm xoá ảnh hiện có — không phân biệt được với
	// "field không gửi" nên phải check GetPostForm ok (khuôn config/company đã xoá).
	if v, ok := c.GetPostForm("logo_image"); ok && v == "" {
		item.LogoImage = ""
	}
}

// bindBusinessProfileJSON — JSON: merge theo key-presence (tránh zero-value overwrite).
func bindBusinessProfileJSON(body map[string]interface{}, item *dao.BusinessProfile) {
	if v, ok := body["name"]; ok {
		item.Name = helpers.ToString(v)
	}
	if v, ok := body["code"]; ok {
		item.Code = helpers.ToString(v)
	}
	if v, ok := body["hotline"]; ok {
		item.Hotline = helpers.ToString(v)
	}
	if v, ok := body["address"]; ok {
		item.Address = helpers.ToString(v)
	}
	if v, ok := body["email"]; ok {
		item.Email = helpers.ToString(v)
	}
	if v, ok := body["website"]; ok {
		item.Website = helpers.ToString(v)
	}
	if v, ok := body["tax_code"]; ok {
		item.TaxCode = helpers.ToString(v)
	}
	if v, ok := body["timezone"]; ok {
		item.Timezone = helpers.ToString(v)
	}
	if v, ok := body["currency"]; ok {
		item.Currency = helpers.ToString(v)
	}
	if v, ok := body["currency_symbol"]; ok {
		item.CurrencySymbol = helpers.ToString(v)
	}
	if v, ok := body["currency_locale"]; ok {
		item.CurrencyLocale = helpers.ToString(v)
	}
	if v, ok := body["weekly_rest_days"]; ok {
		item.WeeklyRestDays = helpers.ToString(v)
	}
	if v, ok := body["logo_image"]; ok {
		item.LogoImage = helpers.ToString(v)
	}
	if v, ok := body["allowed_ips"]; ok {
		if v == nil {
			item.AllowedIps = nil
		} else if b, err := json.Marshal(v); err == nil {
			item.AllowedIps = datatypes.JSON(b)
		}
	}
	if v, ok := body["require_2fa"]; ok {
		item.Require2FA = int(helpers.ToInt64(v))
	}
	if v, ok := body["require_2fa_roles"]; ok {
		if v == nil {
			item.Require2FARoles = nil
		} else if b, err := json.Marshal(v); err == nil {
			item.Require2FARoles = datatypes.JSON(b)
		}
	}
	if v, ok := body["totp_trust_days"]; ok {
		item.TotpTrustDays = int(helpers.ToInt64(v))
	}
}

// validateBusinessProfile — trả i18n key nếu payload sai, "" nếu hợp lệ.
func validateBusinessProfile(item *dao.BusinessProfile) string {
	if strings.TrimSpace(item.Timezone) == "" {
		return "business_profile_timezone_required"
	}
	if _, err := time.LoadLocation(item.Timezone); err != nil {
		return "business_profile_timezone_invalid"
	}
	if strings.TrimSpace(item.Currency) == "" || len(item.Currency) > 10 {
		return "business_profile_currency_invalid"
	}
	if len(item.CurrencySymbol) > 10 {
		return "business_profile_currency_invalid"
	}
	if len(item.CurrencyLocale) > 20 {
		return "business_profile_currency_invalid"
	}
	if !helpers.ValidateIPOrCIDRList(item.AllowedIps) {
		return "business_profile_allowed_ips_invalid"
	}
	if len(item.Require2FARoles) > 0 && !json.Valid(item.Require2FARoles) {
		return "business_profile_require_2fa_roles_invalid"
	}
	if item.TotpTrustDays < 0 {
		return "business_profile_totp_trust_days_invalid"
	}
	return ""
}

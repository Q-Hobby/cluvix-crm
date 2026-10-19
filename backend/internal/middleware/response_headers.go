package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/seclog"
	"erpCluvix/pkg/tz"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ipAllowlistMode: off | log | enforce (set 1 lần lúc boot qua SetIPAllowlistMode).
var ipAllowlistMode = "off"

// SetIPAllowlistMode cấu hình chế độ IP allowlist nhân viên (từ IP_ALLOWLIST_MODE).
func SetIPAllowlistMode(mode string) {
	if mode == "log" || mode == "enforce" {
		ipAllowlistMode = mode
	} else {
		ipAllowlistMode = "off"
	}
}

// BumpUserAuthVersion đánh dấu profile user đã thay đổi.
// FE đọc X-User-Auth-Version header → detect lệch → gọi /api/auth/me để refresh.
// Gọi sau khi admin update profile user (fullname, avatar, phone, ...).
// Không cần gọi khi revoke token (đổi permission/branch/company) — token refresh flow lo.
func BumpUserAuthVersion(ch *cache.RedisCache, userID uint) {
	if ch == nil {
		return
	}
	key := fmt.Sprintf("auth:user_auth_ver:%d", userID)
	_ = ch.Set(key, strconv.FormatInt(time.Now().Unix(), 10), 30*24*time.Hour)
}

// ResponseHeaders middleware — tương đương Yii2 response.beforeSend
// Inject headers vào mọi API response:
// - X-Server-Time
// - X-Version-Dexie, X-Version-Emr, Cluvix-Customer-Lead (cached 5 phút)
// - Cluvix-Reminder (cached 30s, chỉ khi authenticated)
//
// Phase 3 DI: nhận cache qua factory thay vì package-level var AppCache.
func ResponseHeaders(db *gorm.DB, c *cache.RedisCache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer = &responseHeaderWriter{
			ResponseWriter: ctx.Writer,
			db:             db,
			cache:          c,
		}
		ctx.Next()
	}
}

// responseHeaderWriter inject version headers ngay trước khi Write
type responseHeaderWriter struct {
	gin.ResponseWriter
	db         *gorm.DB
	cache      *cache.RedisCache
	headersSet bool
}

func (w *responseHeaderWriter) Write(data []byte) (int, error) {
	if !w.headersSet {
		w.setVersionHeaders()
		w.headersSet = true
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseHeaderWriter) WriteString(s string) (int, error) {
	if !w.headersSet {
		w.setVersionHeaders()
		w.headersSet = true
	}
	return w.ResponseWriter.WriteString(s)
}

func (w *responseHeaderWriter) setVersionHeaders() {
	dexie := getDexieVersionsHeader(w.db, w.cache)
	if dexie != "" {
		w.Header().Set("X-Dexie-Versions", dexie)
	}
}

func getDexieVersionsHeader(db *gorm.DB, ch *cache.RedisCache) string {
	if ch != nil {
		var v string
		if err := ch.Get("config:versions", &v); err == nil {
			return v
		}
	}
	v := dexie.SerializeDexieVersionsHeader(dexie.GetDexieVersions(db))
	if ch != nil {
		ch.Set("config:versions", v, 5*time.Minute)
	}
	return v
}

// getReminderCount — số nhắc việc hôm nay của người gọi. Khoá cache theo userID (dữ liệu của riêng
// người đó, không theo chi nhánh). Story-40: "hôm nay" theo múi giờ thương hiệu, bỏ tham số company.
func getReminderCount(db *gorm.DB, ch *cache.RedisCache, userID uint) int {
	cacheKey := fmt.Sprintf("reminder:count:%d", userID)

	if ch != nil {
		var count int
		if err := ch.Get(cacheKey, &count); err == nil {
			return count
		}
	}

	// Today start/end in brand timezone
	now := tz.Now(db)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Unix()

	var count int64
	db.Table("reminder").
		// status lấy từ hằng số `define.ReminderStatusPlaned` (=1), KHÔNG hardcode. Trước đây viết
		// `status = 0` — không khớp giá trị nào có thật (`models.BaseModel.Status` default 1, FE gửi 1)
		// ⇒ header `Cluvix-Reminder` luôn rỗng ⇒ hộp thoại "Lời nhắc chưa hoàn thành" trên web không
		// bao giờ tự mở. Cron `mobile.push_reminder_due` vốn đã dùng đúng hằng số này.
		Where("JSON_CONTAINS(relatedUser, ?) AND start_time BETWEEN ? AND ? AND status = ?",
			fmt.Sprintf("%d", userID), todayStart, todayEnd, define.ReminderStatusPlaned).
		Count(&count)

	result := int(count)

	if ch != nil {
		ch.Set(cacheKey, result, 30*time.Second)
	}

	return result
}

// AuthResponseHeaders — headers chỉ cho authenticated requests (reminder, updatePassword)
// UserScope holds tenant/branch scope loaded from DB, cached in Redis
//
// Story-58 (E6/D15): KHÔNG còn field company — session chỉ còn chi nhánh. Policy bảo mật (IP
// allowlist, 2FA bắt buộc) là cấu hình TOÀN HỆ THỐNG đọc qua `pkg/brand` (D22), không còn theo
// "company đang active". Entry cache cũ (còn 2 field company) vẫn unmarshal được —
// field thừa bị bỏ qua, và tự hết hạn sau 5'.
type UserScope struct {
	Locale         string   `json:"locale"`
	BranchActiveId uint     `json:"branchActive_id"`
	BranchIds      []uint   `json:"branchIds"`
	Status         int      `json:"status"`
	AllowedIPs     []string `json:"allowedIps"`   // effective: user override > hồ sơ thương hiệu
	IsSuper        bool     `json:"isSuper"`      // super-user bypass IP allowlist
	MustSetup2FA   bool     `json:"mustSetup2fa"` // policy bắt 2FA nhưng chưa enroll → chặn API tới khi setup
	// MustChangePassword — user.must_change_password = 1 (S-4/D-1): chặn API tới khi tự đổi mật khẩu.
	MustChangePassword bool `json:"mustChangePassword"`
}

// UserScopeFor — phạm vi chi nhánh + trạng thái của MỘT user BẤT KỲ, đọc được NGOÀI vòng đời
// request.
//
// ── Vì sao phải export ────────────────────────────────────────────────────────────────────────
// Mọi endpoint đều lấy phạm vi xem của người gọi từ `*gin.Context` (khoá `branchActiveId`,
// `branchIds`) do `AuthResponseHeaders` nạp từ `loadUserScope`.
// Có đúng một chỗ trong hệ cần phạm vi của người KHÁC người đang gọi: **fan-out realtime của
// teamchat** — tin gửi vào kênh gắn nghiệp vụ (`kind=4`) chỉ được đẩy tới người CÒN xem được bản
// ghi gốc, mà ở điểm publish không có `*gin.Context` của người NHẬN (story-28 QA pass 1 F1).
//
// ⛔ Đây là vỏ MỎNG quanh `loadUserScope`, KHÔNG phải bản đọc scope thứ hai. Tuyệt đối không chép
// truy vấn của nó ra nơi khác: hai bản đọc sẽ trôi lệch, và khi lệch thì lệch âm thầm về phía
// "thấy nhiều hơn được phép". Cùng `ch` ⇒ dùng CHUNG cache `user_scope:<id>` với đường request,
// nên fan-out và cửa HTTP luôn thấy cùng một phạm vi, và `InvalidateUserScope` hạ cả hai một lượt.
// `ch = nil` ⇒ đọc thẳng DB (đúng, chỉ chậm hơn).
//
// ⚠ Trả về giá trị THÔ. Người gọi phải tự quyết fail-closed: `Status == 0` nghĩa là KHÔNG đọc được
// scope (user không tồn tại, lỗi DB, hoặc không đọc được policy bảo mật) — phải coi như từ chối.
func UserScopeFor(db *gorm.DB, ch *cache.RedisCache, userID uint) UserScope {
	return loadUserScope(db, ch, userID)
}

func loadUserScope(db *gorm.DB, ch *cache.RedisCache, userID uint) UserScope {
	cacheKey := fmt.Sprintf("user_scope:%d", userID)

	if ch != nil {
		var s UserScope
		// Status=0 → cache entry từ trước khi có Status field → force re-read từ DB
		if err := ch.Get(cacheKey, &s); err == nil && s.Status != 0 {
			return s
		}
	}

	// Query DB: user + user_profile. Story-58: bỏ `LEFT JOIN company ON companyActive_id` — policy
	// bảo mật đọc qua pkg/brand ngay dưới.
	var row struct {
		Locale         *string `gorm:"column:locale"`
		BranchActiveId *uint   `gorm:"column:branchActive_id"`
		BranchId       string  `gorm:"column:branchId"`
		Status         int     `gorm:"column:status"`
		UserAllowedIPs *string `gorm:"column:user_allowed_ips"`
		TotpEnabled    int     `gorm:"column:totp_enabled"`
		MustChangePwd  int     `gorm:"column:must_change_password"`
	}
	db.Table("`user` u").
		Select("p.locale, p.branchActive_id, u.branchId, u.status, "+
			"u.allowed_ips AS user_allowed_ips, u.totp_enabled, u.must_change_password").
		Joins("LEFT JOIN user_profile p ON p.user_id = u.id").
		Where("u.id = ?", userID).
		Scan(&row)

	// Policy bảo mật toàn hệ thống (D22). FAIL-CLOSED: không đọc được hồ sơ ⇒ trả scope rỗng
	// (Status = 0 ⇒ AuthResponseHeaders 401) và KHÔNG cache — tuyệt đối không rơi về Defaults()
	// ("không bắt 2FA, không giới hạn IP"). Tương đương thân cũ: câu JOIN company hỏng ⇒ Status 0.
	policy, err := brand.ProfileStrict(db)
	if err != nil {
		if logger.L != nil {
			logger.L.Warn("loadUserScope: không đọc được policy bảo mật — từ chối request (fail-closed)",
				zap.Uint("userID", userID), zap.Error(err))
		}
		return UserScope{}
	}

	s := UserScope{}
	s.Status = row.Status
	if row.Locale != nil {
		s.Locale = *row.Locale
	}
	if row.BranchActiveId != nil {
		s.BranchActiveId = *row.BranchActiveId
	}
	// Parse JSON arrays
	if row.BranchId != "" {
		var ids []uint
		if err := json.Unmarshal([]byte(row.BranchId), &ids); err == nil {
			s.BranchIds = ids
		}
	}
	// IP allowlist: user override thắng hồ sơ thương hiệu; rỗng cả hai = không giới hạn.
	s.AllowedIPs = parseIPList(row.UserAllowedIPs)
	if len(s.AllowedIPs) == 0 {
		brandIPs := string(policy.AllowedIPs)
		s.AllowedIPs = parseIPList(&brandIPs)
	}
	// Super-user bypass — EXISTS 1 role super gán cho user.
	db.Raw("SELECT EXISTS(SELECT 1 FROM rbac_auth_assignment a "+
		"JOIN rbac_auth_item i ON a.item_name = i.name "+
		"WHERE a.user_id = ? AND i.type = 1 AND i.name IN ?)", userID, define.SuperRoles).
		Scan(&s.IsSuper)

	// 2FA policy: bắt buộc nếu hồ sơ require_2fa=1 HOẶC role của user ∈ require_2fa_roles.
	// MustSetup2FA = bắt buộc NHƯNG chưa enroll → AuthResponseHeaders chặn tới khi setup.
	require2FA := policy.Require2FA
	roles := string(policy.Require2FARoles)
	if !require2FA && roles != "" && roles != "[]" {
		var match int
		db.Raw("SELECT EXISTS(SELECT 1 FROM rbac_auth_assignment a "+
			"WHERE a.user_id = ? AND JSON_CONTAINS(?, JSON_QUOTE(a.item_name)))",
			userID, roles).Scan(&match)
		require2FA = match == 1
	}
	s.MustSetup2FA = require2FA && row.TotpEnabled != 1
	s.MustChangePassword = row.MustChangePwd == define.UserMustChangePasswordYes

	if ch != nil {
		ch.Set(cacheKey, s, 5*time.Minute)
	}

	return s
}

// parseIPList unmarshal JSON array IP/CIDR, bỏ entry rỗng. NULL/lỗi → nil.
func parseIPList(raw *string) []string {
	if raw == nil || *raw == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(*raw), &list); err != nil {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

// InvalidateUserScope clears cached scope when user changes branch/status.
// Caller passes its own cache reference (typically h.Cache from Deps).
func InvalidateUserScope(ch *cache.RedisCache, userID uint) {
	if ch != nil {
		ch.Delete(fmt.Sprintf("user_scope:%d", userID))
	}
}

// InvalidateAllUserScopes xoá cache scope của MỌI user — dùng khi policy bảo mật toàn hệ thống
// (business_profile: require_2fa, require_2fa_roles, allowed_ips) đổi. Scope cache 5 phút theo
// từng user nên chỉ xoá scope của người sửa thì user khác vẫn chạy policy cũ tới hết TTL.
func InvalidateAllUserScopes(ch *cache.RedisCache) {
	if ch != nil {
		ch.DeletePattern("user_scope:*")
	}
}

// is2FAExemptPath — path được đi qua khi user đang bị ép setup 2FA (để enroll được +
// FE load user rồi điều hướng vào trang setup).
func is2FAExemptPath(p string) bool {
	return strings.HasPrefix(p, "/api/auth/2fa/") ||
		p == "/api/auth/me" ||
		p == "/api/auth/permission" ||
		p == "/api/auth/logout"
}

// isPasswordChangeExemptPath — path được đi qua khi user còn cờ must_change_password: tự đổi
// mật khẩu + /me, /permission (FE nạp user rồi điều hướng vào màn đổi mật khẩu) + logout.
func isPasswordChangeExemptPath(p string) bool {
	return p == "/api/hr/user/update-password" ||
		p == "/api/auth/me" ||
		p == "/api/auth/permission" ||
		p == "/api/auth/logout"
}

// shouldLogIPDeny throttle seclog ip_not_allowed còn 1 lần / (user,ip) / 5 phút —
// tránh flood audit khi mọi request từ IP sai đều bắn. Fail-open (không cache → log).
func shouldLogIPDeny(ch *cache.RedisCache, userID uint, ip string) bool {
	if ch == nil {
		return true
	}
	key := ch.Prefix() + fmt.Sprintf("ip_deny_log:%d:%s", userID, ip)
	ok, err := ch.Client().SetNX(context.Background(), key, "1", 5*time.Minute).Result()
	if err != nil {
		return true
	}
	return ok
}

func AuthResponseHeaders(db *gorm.DB, ch *cache.RedisCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID > 0 {
			scope := loadUserScope(db, ch, userID)

			// Reject non-active users — status checked per-request via cached UserScope.
			// user_scope cache bị xóa khi admin đổi status → request ngay sau đó re-read từ DB.
			if scope.Status != define.UserStatusActive {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "account_not_active")})
				return
			}

			// Set all scope data into context
			if scope.Locale != "" {
				c.Set("locale", scope.Locale)
			}
			c.Set("branchActiveId", scope.BranchActiveId)
			c.Set("branchIds", scope.BranchIds)
			// Ép setup 2FA: policy bắt buộc mà chưa enroll → chặn mọi API trừ endpoint
			// setup 2FA + /me (để FE load user rồi điều hướng vào trang setup). FE guard
			// đưa user tới trang setup; đây là lớp enforce server-side (không bypass được).
			// Ép đổi mật khẩu (S-4/D-1): cờ must_change_password = 1 (admin seed mật khẩu mặc định
			// công khai, hoặc mật khẩu do người khác đặt lại) → chặn mọi API trừ tự đổi mật khẩu +
			// /me. Xét TRƯỚC gate 2FA và thay chỗ nó: gate 2FA không miễn update-password, gộp nối
			// tiếp sẽ khoá chết user vừa bị ép đổi mật khẩu vừa bị ép setup 2FA. Đổi mật khẩu xong
			// (revoke mọi session ⇒ đăng nhập lại) gate 2FA áp như thường.
			if scope.MustChangePassword {
				if !isPasswordChangeExemptPath(c.Request.URL.Path) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"message": i18n.T(c, "password_change_required"),
						"code":    define.ErrCodePasswordChangeRequired,
					})
					return
				}
			} else if scope.MustSetup2FA && !is2FAExemptPath(c.Request.URL.Path) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"message": i18n.T(c, "twofa_setup_required"),
					"code":    "2fa_setup_required",
				})
				return
			}

			// IP allowlist nhân viên (Hướng 2A). Super-user bypass; list rỗng = không
			// giới hạn.
			if ipAllowlistMode != "off" && !scope.IsSuper && len(scope.AllowedIPs) > 0 {
				if !helpers.IPInAllowlist(c.ClientIP(), scope.AllowedIPs) {
					if shouldLogIPDeny(ch, userID, c.ClientIP()) {
						seclog.Event(c, "ip_not_allowed")
					}
					if ipAllowlistMode == "enforce" {
						c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": i18n.T(c, "ip_not_allowed")})
						return
					}
				}
			}

			// X-User-Auth-Version: "{userID}:{timestamp}" — FE so sánh per-user,
			// chỉ user có userId khớp mới trigger re-fetch /api/auth/me.
			userAuthVer := "0"
			if ch != nil {
				var v string
				if err := ch.Get(fmt.Sprintf("auth:user_auth_ver:%d", userID), &v); err == nil && v != "" {
					userAuthVer = v
				}
			}
			c.Header("X-User-Auth-Version", fmt.Sprintf("%d:%s", userID, userAuthVer))

			reminderCount := getReminderCount(db, ch, userID)
			if reminderCount > 0 {
				c.Header("Cluvix-Reminder", strconv.Itoa(reminderCount))
			}
		}
		c.Next()
	}
}

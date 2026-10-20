package handler

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/seclog"

	devdao "erpCluvix/internal/modules/webapp/auth/device/dao"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	twofaSvc "erpCluvix/internal/modules/webapp/auth/twofa/service"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/token"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Deps struct {
	DB       *gorm.DB
	Cache    *cache.RedisCache
	TokenSvc *token.Service
	RbacSvc  *authSvc.RbacService
	TwoFASvc *twofaSvc.Service
	// Prod bật CSRF guard trên public Logout (dev FE/BE có thể khác site → skip).
	Prod bool
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

const (
	loginFailMax    = 10
	loginFailWindow = 5 * time.Minute

	// Per-account lockout — chặt hơn IP-limit (IP có thể là NAT nhiều user hợp lệ;
	// 1 account sai liên tục thì khả nghi hơn). Cùng window với IP-limit.
	loginFailUserMax = 5

	refreshCookieName = "refresh_token"
	refreshCookiePath = "/api/auth"

	// deviceTrustCookiePrefix + userID — token remember-device (HttpOnly, cùng path refresh).
	// Tách theo user để nhiều tài khoản trên cùng trình duyệt không ghi đè nhau.
	deviceTrustCookiePrefix = "device_trust_"

	// mobileTransportKey — mobile v1 (/api/mobile/v1/auth/*) set flag này trước khi
	// delegate vào Login/Login2FA/RefreshToken/Logout: refresh token đi trong JSON
	// body thay vì HttpOnly cookie (app native cất token ở secure storage, không
	// dùng cookie jar). Web giữ nguyên cookie — không nhận refresh từ body (XSS).
	mobileTransportKey = "mobileTokenTransport"
)

// MarkMobileTokenTransport — wrapper mobile v1 gọi trước khi delegate.
func MarkMobileTokenTransport(c *gin.Context) { c.Set(mobileTransportKey, true) }

func isMobileTransport(c *gin.Context) bool { return c.GetBool(mobileTransportKey) }

// isSecureRequest detects HTTPS — TLS direct or via reverse proxy (X-Forwarded-Proto).
// Required for Secure cookie flag: prod (HTTPS) → true, dev localhost (HTTP) → false.
func isSecureRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

// cookieSameSite chọn SameSite theo môi trường:
//   - HTTPS (TLS direct hoặc qua proxy): None — cho phép cross-site XHR (FE+BE khác domain
//     trong dev, hoặc khi triển khai trên subdomain khác). Bắt buộc Secure=true.
//   - HTTP (dev localhost): Lax — vì SameSite=None yêu cầu Secure=true, không hợp lệ trên HTTP.
//     Lax đủ dùng vì FE+BE cùng `localhost` trong setup HTTP dev.
func cookieSameSite(c *gin.Context) http.SameSite {
	if isSecureRequest(c) {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

// setRefreshCookie persists refresh_token as HttpOnly cookie scoped to /api/auth.
// JS cannot read the value (defends against XSS leaking the refresh token).
func setRefreshCookie(c *gin.Context, refreshToken string, refreshExpiresAt int64) {
	maxAge := int(refreshExpiresAt - time.Now().Unix())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetSameSite(cookieSameSite(c))
	c.SetCookie(
		refreshCookieName,
		refreshToken,
		maxAge,
		refreshCookiePath,
		"",
		isSecureRequest(c),
		true,
	)
}

// clearRefreshCookie removes the refresh_token cookie on logout.
func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(cookieSameSite(c))
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", isSecureRequest(c), true)
}

func deviceTrustCookieName(userID uint) string {
	return deviceTrustCookiePrefix + strconv.FormatUint(uint64(userID), 10)
}

// setDeviceTrustCookie lưu token remember-device dạng HttpOnly — JS không đọc được, giống refresh.
func setDeviceTrustCookie(c *gin.Context, userID uint, trustToken string, trustUntil int64) {
	maxAge := int(trustUntil - time.Now().Unix())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetSameSite(cookieSameSite(c))
	c.SetCookie(deviceTrustCookieName(userID), trustToken, maxAge, refreshCookiePath, "", isSecureRequest(c), true)
}

// deviceTrustToken lấy token remember-device client trình ra: web từ cookie HttpOnly,
// mobile từ body (cùng quy ước transport với refresh token).
func deviceTrustToken(c *gin.Context, userID uint, fromBody string) string {
	if isMobileTransport(c) {
		return fromBody
	}
	v, _ := c.Cookie(deviceTrustCookieName(userID))
	return v
}

func hashDeviceTrustToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// --- Login ---

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// DeviceFingerprint: SHA-256 base64url FE tính (fingerprint.ts). Optional —
	// thiếu chỉ log bất thường, không reject (client cũ / storage bị chặn).
	DeviceFingerprint string `json:"device_fingerprint"`
	// Locale: ngôn ngữ user đang chọn ở màn login (trước khi có token). Optional —
	// khác locale đang lưu trong user_profile → đồng bộ NGAY tại đây trước khi trả
	// response, để FE luôn nhận đúng locale mới nhất (loadUserData đọc SAU update này).
	Locale string `json:"locale"`
	// DeviceTrustToken CHỈ đọc khi mobile transport — web dùng cookie HttpOnly.
	DeviceTrustToken string `json:"device_trust_token"`
}

func (h *Handler) deviceDao() *devdao.Dao { return &devdao.Dao{DB: h.DB} }

// sanitizeFingerprint validate fingerprint từ client trước khi ghi DB/Redis:
// base64url charset, độ dài 8–64. Sai format → coi như không gửi.
func sanitizeFingerprint(fp string) string {
	fp = strings.TrimSpace(fp)
	if len(fp) < 8 || len(fp) > 64 {
		return ""
	}
	for _, r := range fp {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return ""
		}
	}
	return fp
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "required")})
		return
	}

	ip := c.ClientIP()
	// Rate-limit 2 tầng: theo IP (chống 1 nguồn brute nhiều account) + theo username
	// (chống credential-stuffing 1 account phân tán nhiều IP). Username lowercased vì
	// MySQL `username` collation case-insensitive → tránh bypass bằng đổi hoa/thường.
	unameKey := strings.ToLower(strings.TrimSpace(req.Username))
	if !h.checkLoginRateLimit(ip) || !h.checkLoginRateLimitUser(unameKey) {
		seclog.Event(c, "login_failed", seclog.Username(req.Username), zap.String("reason", "rate_limited"))
		retryAfter := h.loginRetryAfterSeconds(ip, unameKey)
		c.JSON(http.StatusTooManyRequests, gin.H{"message": i18n.T(c, "login_rate_limit"), "retry_after": retryAfter})
		return
	}

	// Find user by username (KHÔNG lọc status ở đây — để phân biệt not_found vs inactive
	// trong audit log; response vẫn generic "login_failed" chống user enumeration).
	var user struct {
		ID           uint   `json:"id"`
		Username     string `json:"username"`
		PasswordHash string `json:"-"`
		Status       int    `json:"status"`
	}
	if err := h.DB.Table("user").Select("id, username, password_hash, status").
		Where("username = ?", req.Username).
		First(&user).Error; err != nil {
		h.recordLoginFailure(ip)
		h.recordLoginFailureUser(unameKey)
		seclog.Event(c, "login_failed", seclog.Username(req.Username), zap.String("reason", "user_not_found"))
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "login_failed")})
		return
	}

	// Story-58 (E6/D15): audit KHÔNG còn gắn company (1 thương hiệu/DB; security_audit_log.company_id
	// NULL được). Trước đây resolve `user_profile.companyActive_id` để view audit theo company thấy.
	failFields := func(reason string) []zap.Field {
		return []zap.Field{seclog.Username(req.Username), seclog.UserID(user.ID), zap.String("reason", reason)}
	}

	// Reject inactive (thay cho status filter cũ ở WHERE) — reason=inactive.
	if user.Status != define.UserStatusActive {
		h.recordLoginFailure(ip)
		h.recordLoginFailureUser(unameKey)
		seclog.Event(c, "login_failed", failFields("inactive")...)
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "login_failed")})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.recordLoginFailure(ip)
		h.recordLoginFailureUser(unameKey)
		seclog.Event(c, "login_failed", failFields("bad_password")...)
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "login_failed")})
		return
	}
	h.clearLoginFailures(ip)
	h.clearLoginFailuresUser(unameKey)

	// Đồng bộ locale NGAY sau khi xác thực xong (trước loadUserData ở issueLoginSuccess) —
	// để response trả về user.userProfile.locale đã là giá trị mới nhất, FE khỏi phải tự
	// PATCH lại sau (tránh race điều hướng + tránh gọi thêm 1 API riêng).
	h.syncUserLocale(c, user.ID, req.Locale)

	fingerprint := sanitizeFingerprint(req.DeviceFingerprint)

	// 2FA challenge: user đã bật 2FA + thiết bị chưa được tin (hết hạn/mới) → yêu cầu
	// nhập mã. CHƯA cấp token. Ép-setup (policy bắt mà chưa bật) do AuthResponseHeaders lo.
	trustToken := deviceTrustToken(c, user.ID, req.DeviceTrustToken)
	if h.TwoFASvc != nil && h.TwoFASvc.IsEnabled(user.ID) && !h.deviceTrustedFor2FA(user.ID, fingerprint, trustToken) {
		challenge, err := h.TokenSvc.IssueChallengeToken(user.ID, fingerprint)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "token_create_error")})
			return
		}
		seclog.Event(c, "2fa_challenge", seclog.UserID(user.ID))
		c.JSON(http.StatusOK, gin.H{"ok": true, "requires_2fa": true, "challenge_token": challenge})
		return
	}

	h.issueLoginSuccess(c, user.ID, fingerprint, ip, 0)
}

// issueLoginSuccess: device upsert + seclog + cấp token + set cookie + response.
// Dùng chung cho login không-2FA và login sau khi verify 2FA. trustUntil > 0 → cấp token
// remember-device ngẫu nhiên, lưu hash + totp_trusted_until SAU khi upsert đã tạo row.
func (h *Handler) issueLoginSuccess(c *gin.Context, userID uint, fingerprint, ip string, trustUntil int64) {
	var trustToken string
	if fingerprint != "" {
		devName := helpers.DeviceNameFromUA(c.GetHeader("User-Agent"))
		if _, isNew, devErr := h.deviceDao().Upsert(userID, fingerprint, devName, ip, 0); devErr != nil {
			middleware.GetLogger(c).Error("user_device upsert failed", zap.Error(devErr))
		} else if isNew {
			seclog.Event(c, "new_device", seclog.UserID(userID), seclog.FP(fingerprint), zap.String("device", devName))
		}
		// Row device đã tồn tại (vừa upsert) → set trust an toàn. Lỗi sinh/ghi token chỉ
		// làm mất "nhớ thiết bị" (lần sau hỏi 2FA lại), không chặn đăng nhập.
		if trustUntil > 0 {
			t, err := token.RandomHex(32)
			if err == nil {
				err = h.DB.Table("user_device").Where("user_id = ? AND fingerprint = ?", userID, fingerprint).
					Updates(map[string]any{"totp_trusted_until": trustUntil, "totp_trust_hash": hashDeviceTrustToken(t)}).Error
			}
			if err != nil {
				middleware.GetLogger(c).Error("set device trust failed", zap.Error(err))
			} else {
				trustToken = t
			}
		}
	} else {
		seclog.Event(c, "login_no_fingerprint", seclog.UserID(userID))
	}
	seclog.Event(c, "login_success", seclog.UserID(userID), seclog.FP(fingerprint))

	result, err := h.TokenSvc.IssueAccess(userID, c.ClientIP(), fingerprint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": i18n.T(c, "token_create_error")})
		return
	}
	userData := h.loadUserData(userID)
	resp := gin.H{
		"ok":               true,
		"message":          "Login successful",
		"accessToken":      result.AccessToken,
		"expiresIn":        result.ExpiresIn,
		"expiresAt":        result.ExpiresAt,
		"refreshExpiresAt": result.RefreshExpiresAt,
		"tokenType":        result.TokenType,
		"user":             userData["user"],
		"permission":       userData["permission"],
	}
	if isMobileTransport(c) {
		resp["refreshToken"] = result.RefreshToken
		if trustToken != "" {
			resp["deviceTrustToken"] = trustToken
		}
	} else {
		setRefreshCookie(c, result.RefreshToken, result.RefreshExpiresAt)
		if trustToken != "" {
			setDeviceTrustCookie(c, userID, trustToken, trustUntil)
		}
	}
	c.JSON(http.StatusOK, resp)
}

// deviceTrustedFor2FA: thiết bị còn trong hạn skip 2FA (remember device). Fingerprint do client
// tự tính từ thông tin trình duyệt đoán được ⇒ KHÔNG đủ để bỏ 2FA; bắt buộc kèm token ngẫu nhiên
// server đã cấp lúc verify 2FA (so hash, constant-time). Thiết bị đã thu hồi không được tin.
func (h *Handler) deviceTrustedFor2FA(userID uint, fingerprint, trustToken string) bool {
	if fingerprint == "" || trustToken == "" {
		return false
	}
	var row struct {
		TotpTrustedUntil *int64
		TotpTrustHash    *string
	}
	h.DB.Table("user_device").Select("totp_trusted_until, totp_trust_hash").
		Where("user_id = ? AND fingerprint = ? AND status <> ?", userID, fingerprint, define.UserDeviceRevoked).
		Scan(&row)
	if row.TotpTrustedUntil == nil || *row.TotpTrustedUntil <= time.Now().Unix() || row.TotpTrustHash == nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(*row.TotpTrustHash), []byte(hashDeviceTrustToken(trustToken))) == 1
}

// syncUserLocale: locale FE gửi kèm lúc login (đang chọn ở màn login) khác locale đang lưu
// trong user_profile → update NGAY để loadUserData (chạy sau, trong issueLoginSuccess) đọc
// đúng giá trị mới — FE không còn phải tự gọi thêm API PATCH sau khi có token.
// Bỏ qua im lặng nếu locale rỗng/không hợp lệ hoặc lỗi DB (không chặn luồng đăng nhập).
func (h *Handler) syncUserLocale(c *gin.Context, userID uint, locale string) {
	if locale == "" || !define.UserProfileLocales[locale] {
		return
	}
	var current string
	h.DB.Table("user_profile").Select("locale").Where("user_id = ?", userID).Scan(&current)
	if current == locale {
		return
	}
	if err := h.DB.Table("user_profile").Where("user_id = ?", userID).
		Update("locale", locale).Error; err != nil {
		middleware.GetLogger(c).Error("sync locale on login failed", zap.Error(err), zap.Uint("userID", userID))
	}
}

// totpTrustDays — số ngày "nhớ thiết bị" sau 2FA: cấu hình TOÀN HỆ THỐNG trên hồ sơ thương hiệu
// (story-58, D22 — trước đây `company.totp_trust_days` của company đang active). 0 = không nhớ
// (luôn hỏi lại). Chưa có hồ sơ / lỗi đọc ⇒ mặc định 30 — y hệt thân cũ (JOIN ra NULL ⇒ 30).
func (h *Handler) totpTrustDays() int {
	return brand.Profile(h.DB).TotpTrustDays
}

// Login2FA POST /auth/login/2fa — bước 2: verify mã TOTP/backup rồi cấp token.
func (h *Handler) Login2FA(c *gin.Context) {
	var req struct {
		ChallengeToken string `json:"challenge_token" binding:"required"`
		Code           string `json:"code" binding:"required"`
		RememberDevice bool   `json:"remember_device"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": i18n.T(c, "required")})
		return
	}
	userID, fingerprint, err := h.TokenSvc.VerifyChallengeToken(req.ChallengeToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "token_invalid")})
		return
	}
	var status int
	h.DB.Table("user").Select("status").Where("id = ?", userID).Scan(&status)
	if status != define.UserStatusActive {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "login_failed")})
		return
	}
	if h.TwoFASvc == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "twofa_invalid_code")})
		return
	}
	// Check rate-limit RIÊNG trước VerifyCode — phân biệt "đang bị khóa tạm" (429 +
	// retry_after cho FE đếm ngược) với "nhập sai mã" (401, vẫn còn lượt thử).
	if limited, retryAfter := h.TwoFASvc.VerifyRateLimited(userID); limited {
		c.Set("userID", userID)
		seclog.Event(c, "2fa_failed", zap.String("reason", "rate_limited"))
		c.JSON(http.StatusTooManyRequests, gin.H{"message": i18n.T(c, "twofa_rate_limited"), "retry_after": retryAfter})
		return
	}
	if !h.TwoFASvc.VerifyCode(userID, req.Code) {
		c.Set("userID", userID)
		seclog.Event(c, "2fa_failed")
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "twofa_invalid_code")})
		return
	}

	// Remember device: tính hạn trust theo hồ sơ thương hiệu (0 = không nhớ, luôn hỏi lần sau).
	// issueLoginSuccess set totp_trusted_until SAU khi upsert tạo row (tránh Update-trước-Insert).
	var trustUntil int64
	if req.RememberDevice && fingerprint != "" {
		if days := h.totpTrustDays(); days > 0 {
			trustUntil = time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix()
		}
	}

	h.issueLoginSuccess(c, userID, fingerprint, c.ClientIP(), trustUntil)
}

// --- Refresh Token ---

type refreshRequest struct {
	DeviceFingerprint string `json:"device_fingerprint"`
	// RefreshToken CHỈ đọc khi mobile transport — web dùng HttpOnly cookie,
	// không nhận từ body để chống XSS leak.
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) RefreshToken(c *gin.Context) {
	var body refreshRequest
	_ = c.ShouldBindJSON(&body)

	// Web: refresh token từ HttpOnly cookie. Mobile: từ body (secure storage phía app).
	var refreshToken string
	if isMobileTransport(c) {
		refreshToken = body.RefreshToken
	} else {
		refreshToken, _ = c.Cookie(refreshCookieName)
	}
	if refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": i18n.T(c, "token_invalid")})
		return
	}

	// Fingerprint từ body (device binding) — optional; sai/thiếu xử lý theo mode.
	fingerprint := sanitizeFingerprint(body.DeviceFingerprint)

	// Mode log: không chặn, chỉ ghi seclog khi mismatch để đo tỷ lệ trước khi enforce.
	if h.TokenSvc.DeviceBindingMode() == "log" {
		if matched, hadFP := h.TokenSvc.RefreshFingerprintMatches(refreshToken, fingerprint); hadFP && !matched {
			seclog.Event(c, "refresh_fp_mismatch", seclog.FP(fingerprint))
		}
	}

	userID, newRefresh, err := h.TokenSvc.VerifyAndRotateRefresh(refreshToken, fingerprint)
	if errors.Is(err, token.ErrRefreshRaced) {
		// Request khác (tab khác) vừa xoay vòng chính token này: KHÔNG xoá cookie — cookie đang
		// giữ là token mới của request thắng. 409 ⇒ FE coi là lỗi thoáng qua, không đăng xuất.
		c.JSON(http.StatusConflict, gin.H{"ok": false, "message": i18n.T(c, "token_invalid")})
		return
	}
	if err != nil {
		// Token invalid/expired hoặc fingerprint mismatch (enforce) → xoá cookie cũ.
		if !isMobileTransport(c) {
			clearRefreshCookie(c)
		}
		if err == token.ErrFingerprintMismatch {
			seclog.Event(c, "refresh_fp_mismatch", seclog.FP(fingerprint), zap.Bool("enforced", true))
		}
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": i18n.T(c, "token_invalid")})
		return
	}

	// Check user still active
	var status int
	h.DB.Table("user").Select("status").Where("id = ?", userID).Scan(&status)
	if status != define.UserStatusActive {
		if !isMobileTransport(c) {
			clearRefreshCookie(c)
		}
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": i18n.T(c, "token_invalid")})
		return
	}

	// Cập nhật last_seen/last_ip cho thiết bị (best-effort, không block refresh).
	// Throttle 1h: refresh ~13 phút/lần không cần ghi DB mỗi lần.
	if fingerprint != "" {
		if _, _, devErr := h.deviceDao().Upsert(userID, fingerprint, helpers.DeviceNameFromUA(c.GetHeader("User-Agent")), c.ClientIP(), time.Hour); devErr != nil {
			middleware.GetLogger(c).Error("user_device upsert on refresh failed", zap.Error(devErr))
		}
	}

	result, err := h.TokenSvc.IssueAccessOnly(userID, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": i18n.T(c, "server_error")})
		return
	}

	resp := gin.H{
		"ok":               true,
		"message":          "Token refreshed",
		"accessToken":      result.AccessToken,
		"expiresIn":        result.ExpiresIn,
		"expiresAt":        result.ExpiresAt,
		"refreshExpiresAt": result.RefreshExpiresAt,
		"tokenType":        result.TokenType,
	}
	// Rotate: mobile nhận refresh mới trong body, web qua cookie.
	if isMobileTransport(c) {
		resp["refreshToken"] = newRefresh
	} else {
		setRefreshCookie(c, newRefresh, result.RefreshExpiresAt)
	}
	c.JSON(http.StatusOK, resp)
}

// --- Logout ---

// Logout là public route (không qua JWTAuth): user với access token đã hết hạn
// vẫn phải consume được refresh cookie — nếu bắt buộc Bearer hợp lệ, logout lúc
// token hết hạn sẽ 401 trước handler và refresh token sống tới hết TTL trong Redis.
func (h *Handler) Logout(c *gin.Context) {
	// Public + cookie-consuming → chặn CSRF forced-logout từ site khác (chỉ prod:
	// prod FE same-origin `/api`; dev FE/BE có thể khác site → skip guard).
	// Sec-Fetch-Site do browser tự set (không giả mạo được từ JS); request
	// non-browser (không có header) và same-origin/same-site đều đi qua.
	if h.Prod && strings.EqualFold(c.GetHeader("Sec-Fetch-Site"), "cross-site") {
		c.Status(http.StatusForbidden)
		return
	}

	// Revoke access token (best-effort — bearer có thể vắng mặt hoặc đã hết hạn)
	var logoutUserID uint
	auth := c.GetHeader("Authorization")
	if auth != "" && strings.HasPrefix(auth, "Bearer ") {
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := h.TokenSvc.DecodeAndValidate(tokenStr, c.ClientIP())
		if err == nil && claims.ID != "" {
			logoutUserID = claims.UID
			ttl := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
			h.TokenSvc.Revoke(claims.ID, ttl)
		}
	}

	// Consume refresh token — invalidate KHÔNG re-issue (rotate ở đây tạo orphan
	// token chiếm 1 slot device limit đến hết RefreshTTL).
	// Best-effort: nếu token đã expired/invalid, Logout vẫn thành công ở phía client.
	// Mobile: token từ body (app không có cookie); web: từ HttpOnly cookie.
	if isMobileTransport(c) {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.RefreshToken != "" {
			_ = h.TokenSvc.ConsumeRefresh(body.RefreshToken)
		}
	} else if refreshToken, err := c.Cookie(refreshCookieName); err == nil && refreshToken != "" {
		_ = h.TokenSvc.ConsumeRefresh(refreshToken)
	}

	// Audit logout — chỉ ghi khi decode được bearer (biết user). Story-58: không gắn company.
	if logoutUserID > 0 {
		seclog.Event(c, "logout", seclog.UserID(logoutUserID))
	}

	// Clear cookie ở browser (mobile không có cookie — skip)
	if !isMobileTransport(c) {
		clearRefreshCookie(c)
	}

	c.Status(http.StatusNoContent)
}

// --- Me ---

func (h *Handler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": i18n.T(c, "token_invalid")})
		return
	}

	userData := h.loadUserData(userID)
	c.JSON(http.StatusOK, userData)
}

// --- Permission ---

func (h *Handler) Permission(c *gin.Context) {
	userID := middleware.GetUserID(c)
	permissions := h.getUserPermissions(userID)
	c.JSON(http.StatusOK, permissions)
}

// --- Helpers ---

// checkLoginRateLimit trả false nếu IP đã vượt loginFailMax lần thất bại trong loginFailWindow.
func (h *Handler) checkLoginRateLimit(ip string) bool {
	if h.Cache == nil {
		return true
	}
	ctx := context.Background()
	key := h.Cache.Prefix() + "auth:login_fail:" + ip
	count, err := h.Cache.Client().Get(ctx, key).Int64()
	if err != nil {
		return true // key chưa tồn tại hoặc lỗi Redis → cho qua
	}
	return count < loginFailMax
}

// recordLoginFailure tăng counter failed login cho IP, set TTL loginFailWindow.
func (h *Handler) recordLoginFailure(ip string) {
	if h.Cache == nil {
		return
	}
	ctx := context.Background()
	key := h.Cache.Prefix() + "auth:login_fail:" + ip
	pipe := h.Cache.Client().Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginFailWindow)
	// Rate-limit counter best-effort — Redis fail không nên block login attempt.
	_, _ = pipe.Exec(ctx)
}

// clearLoginFailures xoá counter sau khi login thành công.
func (h *Handler) clearLoginFailures(ip string) {
	if h.Cache == nil {
		return
	}
	h.Cache.Client().Del(context.Background(), h.Cache.Prefix()+"auth:login_fail:"+ip)
}

// --- Per-account rate limit (song song IP-limit) ---
// Key theo username lowercased. username rỗng (client không gửi) → không giới hạn
// tầng này (IP-limit vẫn chặn), tránh 1 key gộp mọi request thiếu username.

func (h *Handler) userFailKey(username string) string {
	return h.Cache.Prefix() + "auth:login_fail_user:" + username
}

// checkLoginRateLimitUser trả false nếu username vượt loginFailUserMax lần thất bại
// trong loginFailWindow. Fail-open khi Redis lỗi (nhất quán với IP-limit).
func (h *Handler) checkLoginRateLimitUser(username string) bool {
	if h.Cache == nil || username == "" {
		return true
	}
	count, err := h.Cache.Client().Get(context.Background(), h.userFailKey(username)).Int64()
	if err != nil {
		return true
	}
	return count < loginFailUserMax
}

// recordLoginFailureUser tăng counter failed login cho username, set TTL loginFailWindow.
func (h *Handler) recordLoginFailureUser(username string) {
	if h.Cache == nil || username == "" {
		return
	}
	ctx := context.Background()
	key := h.userFailKey(username)
	pipe := h.Cache.Client().Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, loginFailWindow)
	_, _ = pipe.Exec(ctx)
}

// clearLoginFailuresUser xoá counter per-account sau khi login thành công.
func (h *Handler) clearLoginFailuresUser(username string) {
	if h.Cache == nil || username == "" {
		return
	}
	h.Cache.Client().Del(context.Background(), h.userFailKey(username))
}

// loginRetryAfterSeconds: FE hiện đếm ngược khi bị rate-limit — lấy TTL còn lại
// của key IP-limit và user-limit (2 tầng độc lập, TTL khác nhau), trả max để
// đảm bảo user chờ đủ cho CẢ 2 tầng cùng hết hạn (không unblock sớm giả).
func (h *Handler) loginRetryAfterSeconds(ip, username string) int64 {
	if h.Cache == nil {
		return 0
	}
	ctx := context.Background()
	var maxTTL int64
	if ttl, err := h.Cache.Client().TTL(ctx, h.Cache.Prefix()+"auth:login_fail:"+ip).Result(); err == nil && ttl > 0 {
		maxTTL = int64(ttl.Seconds())
	}
	if username != "" {
		if ttl, err := h.Cache.Client().TTL(ctx, h.userFailKey(username)).Result(); err == nil && int64(ttl.Seconds()) > maxTTL {
			maxTTL = int64(ttl.Seconds())
		}
	}
	return maxTTL
}

func (h *Handler) loadUserData(userID uint) gin.H {
	// User + Profile
	var user struct {
		ID                 uint   `json:"id"`
		BranchId           string `json:"branchId"`
		MustChangePassword int    `json:"must_change_password"`
	}
	h.DB.Table("user").Select("id, branchId, must_change_password").Where("id = ?", userID).First(&user)

	var profile struct {
		UserID         uint    `json:"user_id"`
		Fullname       string  `json:"fullname"`
		Nickname       *string `json:"nickname"`
		Code           *string `json:"code"`
		Birthday       *string `json:"birthday"`
		About          string  `json:"about"`
		Address        string  `json:"address"`
		Phone          string  `json:"phone"`
		IdPancake      string  `json:"id_pancake"`
		Facebook       string  `json:"facebook"`
		Telegram       string  `json:"telegram"`
		Avatar         string  `json:"avatar"`
		Cover          *string `json:"cover"`
		Locale         string  `json:"locale"`
		Gender         *int    `json:"gender"`
		DepartmentList string  `json:"departmentList"`
		BranchOfficeId *int    `json:"branchOffice_id"`
		BranchActiveId *int    `json:"branchActive_id"`
		UpdatedAt      int64   `json:"updated_at"` // khoá lạc quan user_profile (D6)
	}
	h.DB.Table("user_profile").Where("user_id = ?", userID).First(&profile)

	// Parse JSON fields
	var branchIds []int
	json.Unmarshal([]byte(user.BranchId), &branchIds)

	var deptList []int
	json.Unmarshal([]byte(profile.DepartmentList), &deptList)

	// Role
	var role struct {
		Name        string `json:"name"`
		Type        int    `json:"type"`
		Description string `json:"description"`
	}
	h.DB.Table("rbac_auth_assignment a").
		Select("i.name, i.type, i.description").
		Joins("JOIN rbac_auth_item i ON a.item_name = i.name").
		Where("a.user_id = ? AND i.type = 1", userID).
		Limit(1).Scan(&role)

	// Permissions
	permissions := h.getUserPermissions(userID)

	profileMap := gin.H{
		"user_id":         profile.UserID,
		"fullname":        profile.Fullname,
		"nickname":        profile.Nickname,
		"code":            profile.Code,
		"birthday":        profile.Birthday,
		"about":           profile.About,
		"address":         profile.Address,
		"phone":           profile.Phone,
		"id_pancake":      profile.IdPancake,
		"facebook":        profile.Facebook,
		"telegram":        profile.Telegram,
		"avatar":          storage.ResolveURL(profile.Avatar), // cột lưu KEY kho public ⇒ ghép URL
		"cover":           profile.Cover,
		"locale":          profile.Locale,
		"gender":          profile.Gender,
		"departmentList":  deptList,
		"branchOffice_id": profile.BranchOfficeId,
		"branchActive_id": profile.BranchActiveId,
		"updated_at":      profile.UpdatedAt,
	}

	var roleMap interface{} = nil
	if role.Name != "" {
		roleMap = gin.H{
			"name":        role.Name,
			"type":        role.Type,
			"description": role.Description,
		}
	}

	return gin.H{
		"user": gin.H{
			"id":          userID,
			"userProfile": profileMap,
			"userRole":    roleMap,
			"branchId":    branchIds,
			// S-4/D-1: true ⇒ FE điều hướng vào màn đổi mật khẩu bắt buộc; BE (AuthResponseHeaders)
			// đồng thời chặn mọi API nghiệp vụ với 403 code `password_change_required`.
			"mustChangePassword": user.MustChangePassword == define.UserMustChangePasswordYes,
		},
		"permission": permissions,
	}
}

// getUserPermissions delegate sang h.RbacSvc (cache + super-role bypass).
func (h *Handler) getUserPermissions(userID uint) []string {
	if h.RbacSvc == nil {
		return []string{}
	}
	perms, err := h.RbacSvc.GetUserPermissions(userID)
	if err != nil {
		return []string{}
	}
	return perms
}

// --- ClearCache ---

// clearCacheKeepPrefixes — key bảo mật nằm CHUNG namespace cache app, clear-cache KHÔNG được xoá:
// bộ đếm sai mật khẩu theo IP/username (`auth:login_fail*`), bộ đếm sai mã 2FA (`2fa:verify_fail:`),
// throttle log IP bị chặn (`ip_deny_log:`). Xoá chúng = reset rate-limit ⇒ brute-force vô hạn (S-11).
var clearCacheKeepPrefixes = []string{"auth:", "2fa:", "ip_deny_log:"}

// ClearCache POST /auth/clear-cache — chỉ super-user (S-11): xoá cache app (scope/quyền/dữ liệu
// cache), GIỮ key rate-limit/2FA. User thường ⇒ 403 bất kể PERM_ENFORCE.
func (h *Handler) ClearCache(c *gin.Context) {
	uid := middleware.GetUserID(c)
	if h.RbacSvc == nil || uid == 0 || !h.RbacSvc.IsSuperUser(uid) {
		response.Forbidden(c)
		return
	}
	if h.Cache == nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": i18n.T(c, "cache_not_configured")})
		return
	}
	h.Cache.DeletePatternExcept("*", clearCacheKeepPrefixes...)
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": i18n.T(c, "cache_cleared")})
}

// --- LogoutAll ---

func (h *Handler) LogoutAll(c *gin.Context) {
	userID := middleware.GetUserID(c)
	h.TokenSvc.RevokeAllSessions(userID)
	// Thiếu dòng này thì thiết bị khác chỉ bị 401 thoáng qua rồi silent-refresh
	// lấy access token mới — "logout tất cả" không có tác dụng.
	h.TokenSvc.RevokeAllRefreshTokens(userID)
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": i18n.T(c, "all_sessions_revoked"),
	})
}

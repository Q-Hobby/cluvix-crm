package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/seclog"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Config struct {
	Secret      string
	AccessTTL   time.Duration // default 900s (15 min)
	RefreshTTL  time.Duration // default 86400s (1 day)
	MaxSessions int           // default 3
	IPBinding   bool
	Issuer      string // "cluvix.develop.issuer"
	Audience    string // "cluvix.develop.aud"
	AuthPrefix  string // "Cluvix:dev:auth:"
	CfgVer      string // "v1"
	// DeviceBindingMode: off | log | enforce — so device_fingerprint khi refresh.
	DeviceBindingMode string
}

// ErrFingerprintMismatch: refresh token dùng từ thiết bị khác lúc login (mode enforce).
// Handler map về 401 như token invalid; sentinel để phân biệt cho seclog.
var ErrFingerprintMismatch = fmt.Errorf("device fingerprint mismatch")

// rtFPSep ngăn cách hash và fingerprint trong value rt:{uid}:{rid}.
// Value không có separator = token legacy (trước khi có device binding).
const rtFPSep = ":fp:"

type Service struct {
	cfg   Config
	redis *redis.Client
}

type Claims struct {
	jwt.RegisteredClaims
	UID    uint   `json:"uid"`
	CfgVer string `json:"cfgVer,omitempty"`
	IPHash string `json:"ipHash,omitempty"`
}

type IssueResult struct {
	AccessToken      string `json:"accessToken"`
	RefreshToken     string `json:"refreshToken"`
	ExpiresIn        int    `json:"expiresIn"`
	ExpiresAt        int64  `json:"expiresAt"`
	RefreshExpiresAt int64  `json:"refreshExpiresAt"`
	TokenType        string `json:"tokenType"`
}

func New(cfg Config, redis *redis.Client) *Service {
	if cfg.AuthPrefix == "" {
		cfg.AuthPrefix = "auth:"
	}
	if cfg.AuthPrefix[len(cfg.AuthPrefix)-1] != ':' {
		cfg.AuthPrefix += ":"
	}
	return &Service{cfg: cfg, redis: redis}
}

// --- Issue ---

func (s *Service) IssueAccess(userID uint, ip string, fingerprint string) (*IssueResult, error) {
	tokenStr, jti, expTs, err := s.buildAccessJWT(userID, ip)
	if err != nil {
		return nil, err
	}

	s.registerSession(userID, jti, expTs)
	if s.cfg.MaxSessions > 0 {
		s.enforceSessionLimit(userID)
	}

	refreshToken, err := s.generateRefreshToken(userID, fingerprint)
	if err != nil {
		return nil, err
	}

	return &IssueResult{
		AccessToken:      tokenStr,
		RefreshToken:     refreshToken,
		ExpiresIn:        int(s.cfg.AccessTTL.Seconds()),
		ExpiresAt:        expTs,
		RefreshExpiresAt: time.Now().Add(s.cfg.RefreshTTL).Unix(),
		TokenType:        "Bearer",
	}, nil
}

func (s *Service) IssueAccessOnly(userID uint, ip string) (*IssueResult, error) {
	tokenStr, jti, expTs, err := s.buildAccessJWT(userID, ip)
	if err != nil {
		return nil, err
	}

	s.registerSession(userID, jti, expTs)
	if s.cfg.MaxSessions > 0 {
		s.enforceSessionLimit(userID)
	}

	return &IssueResult{
		AccessToken:      tokenStr,
		ExpiresIn:        int(s.cfg.AccessTTL.Seconds()),
		ExpiresAt:        expTs,
		RefreshExpiresAt: time.Now().Add(s.cfg.RefreshTTL).Unix(),
		TokenType:        "Bearer",
	}, nil
}

// --- 2FA challenge token ---

// challengeClaims: proof đã qua password, chờ nhập TOTP. Ngắn hạn 5 phút, aud riêng.
type challengeClaims struct {
	jwt.RegisteredClaims
	UID uint   `json:"uid"`
	FP  string `json:"fp,omitempty"`
}

const challengeAudience = "2fa-challenge"

// challengeKey — key TÁCH BIỆT với access token (derive từ Secret). Challenge token
// ký bằng key này → JWTAuth (dùng jwtSecret = Secret) sẽ REJECT vì sai chữ ký.
// CRITICAL: nếu ký bằng Secret, challenge token dùng được như Bearer access token
// (JWTAuth không check audience) → bypass 2FA hoàn toàn.
func (s *Service) challengeKey() []byte {
	h := sha256.Sum256([]byte(s.cfg.Secret + ":2fa-challenge-signing-key"))
	return h[:]
}

// IssueChallengeToken cấp token ngắn hạn (5 phút) sau khi verify password, trước 2FA.
func (s *Service) IssueChallengeToken(userID uint, fingerprint string) (string, error) {
	now := time.Now()
	claims := challengeClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			// Issuer định danh khách (chứa DB_NAME) — challenge của khách A không dùng được ở khách B
			// khi chung TOKEN_SECRET (R-1).
			Issuer:    s.cfg.Issuer,
			Audience:  jwt.ClaimStrings{challengeAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
		UID: userID,
		FP:  fingerprint,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.challengeKey())
}

// VerifyChallengeToken verify token 2FA-challenge → (userID, fingerprint).
func (s *Service) VerifyChallengeToken(tokenStr string) (uint, string, error) {
	claims := &challengeClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.challengeKey(), nil
	})
	if err != nil || !parsed.Valid {
		return 0, "", fmt.Errorf("invalid challenge token")
	}
	okAud := false
	for _, a := range claims.Audience {
		if a == challengeAudience {
			okAud = true
		}
	}
	if !okAud {
		return 0, "", fmt.Errorf("wrong audience")
	}
	if claims.Issuer != s.cfg.Issuer {
		return 0, "", fmt.Errorf("wrong issuer")
	}
	return claims.UID, claims.FP, nil
}

// --- Verify Access ---

// ClaimsMatchTenant — access token phải mang ĐÚNG issuer + audience của khách này (chứa DB_NAME,
// xem config.TokenIssuer). Fail-closed: issuer/audience mong đợi rỗng ⇒ false (cấu hình thiếu không
// được biến thành "bỏ qua kiểm"). Dùng chung cho middleware.JWTAuth và DecodeAndValidate để 2 đường
// verify không trôi lệch (R-1, 2026-10-18).
func ClaimsMatchTenant(rc *jwt.RegisteredClaims, issuer, audience string) bool {
	if issuer == "" || audience == "" || rc.Issuer != issuer {
		return false
	}
	for _, a := range rc.Audience {
		if a == audience {
			return true
		}
	}
	return false
}

func (s *Service) DecodeAndValidate(tokenStr string, ip string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Defend against alg=none / RS256→HS256 confusion: enforce HMAC family
		// since s.cfg.Secret is a shared secret. Match pattern in middleware/auth.go.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if !ClaimsMatchTenant(&claims.RegisteredClaims, s.cfg.Issuer, s.cfg.Audience) {
		return nil, fmt.Errorf("invalid token")
	}

	// Check revoked
	if claims.ID != "" && s.isRevoked(claims.ID) {
		return nil, fmt.Errorf("token revoked")
	}

	// IP binding
	if s.cfg.IPBinding && claims.IPHash != "" {
		if claims.IPHash != sha256Hex(ip) {
			return nil, fmt.Errorf("IP mismatch")
		}
	}

	return claims, nil
}

// --- Refresh Token ---

// VerifyAndRotateRefresh verify refresh token + kiểm device fingerprint theo
// DeviceBindingMode, rồi rotate (invalidate cũ, phát hành mới giữ nguyên fp).
// fingerprint = giá trị client gửi lúc refresh; "" khi client không gửi.
func (s *Service) VerifyAndRotateRefresh(refreshToken string, fingerprint string) (uint, string, error) {
	userID, rid, storedFP, err := s.verifyRefreshToken(refreshToken)
	if err != nil {
		return 0, "", err
	}

	// Device binding: chỉ check khi token có storedFP (token legacy bỏ qua, tự
	// nâng cấp ở rotation này vì token mới sẽ mang fingerprint client vừa gửi).
	if s.cfg.DeviceBindingMode == "enforce" && storedFP != "" && storedFP != fingerprint {
		return 0, "", ErrFingerprintMismatch
	}

	// Invalidate old
	s.invalidateRefresh(userID, rid)

	// Generate new — giữ fingerprint hiện diện: ưu tiên client gửi (nâng cấp token
	// legacy), fallback storedFP nếu client refresh không gửi (mode log/off).
	fp := fingerprint
	if fp == "" {
		fp = storedFP
	}
	newToken, err := s.generateRefreshToken(userID, fp)
	if err != nil {
		return 0, "", err
	}

	return userID, newToken, nil
}

// RefreshFingerprintMatches báo storedFP có khớp fingerprint client gửi không —
// dùng cho mode "log": handler gọi để quyết định có seclog mismatch, KHÔNG chặn.
// Trả (matched, hadStoredFP). token legacy (không fp) → (true, false).
func (s *Service) RefreshFingerprintMatches(refreshToken, fingerprint string) (bool, bool) {
	_, _, storedFP, err := s.verifyRefreshToken(refreshToken)
	if err != nil || storedFP == "" {
		return true, false
	}
	return storedFP == fingerprint, true
}

// ConsumeRefresh verify + invalidate refresh token mà KHÔNG phát hành token mới.
// Dùng cho logout — nếu dùng VerifyAndRotateRefresh, token mới sinh ra không ai
// nhận nhưng vẫn sống hết RefreshTTL trong Redis và chiếm 1 slot device limit.
func (s *Service) ConsumeRefresh(refreshToken string) error {
	userID, rid, _, err := s.verifyRefreshToken(refreshToken)
	if err != nil {
		return err
	}
	s.invalidateRefresh(userID, rid)
	return nil
}

// verifyRefreshToken trả (userID, rid, storedFingerprint, error).
// Value rt:{uid}:{rid} = "<hash>" (legacy) hoặc "<hash>:fp:<fingerprint>".
func (s *Service) verifyRefreshToken(refreshToken string) (uint, string, string, error) {
	if len(refreshToken) < 18 {
		return 0, "", "", fmt.Errorf("malformed refresh token")
	}

	dotIdx := 16 // rid = 16 hex chars
	rid := refreshToken[:dotIdx]
	raw := refreshToken[dotIdx+1:]

	ctx := context.Background()

	// Lookup userID from index
	uidStr, err := s.redis.Get(ctx, s.k("rtidx:"+rid)).Result()
	if err != nil {
		return 0, "", "", fmt.Errorf("refresh token not found/expired")
	}
	userID, _ := strconv.ParseUint(uidStr, 10, 64)

	// Verify hash (tách fingerprint suffix nếu có)
	stored, err := s.redis.Get(ctx, s.k(fmt.Sprintf("rt:%d:%s", userID, rid))).Result()
	if err != nil {
		return 0, "", "", fmt.Errorf("refresh token not found/expired")
	}
	hash, storedFP := stored, ""
	if i := strings.Index(stored, rtFPSep); i >= 0 {
		hash, storedFP = stored[:i], stored[i+len(rtFPSep):]
	}
	if hash != sha256Hex(raw) {
		return 0, "", "", fmt.Errorf("refresh token mismatch")
	}

	return uint(userID), rid, storedFP, nil
}

func (s *Service) generateRefreshToken(userID uint, fingerprint string) (string, error) {
	raw := randomHex(32) // 64 hex chars
	rid := randomHex(8)  // 16 hex chars
	hash := sha256Hex(raw)

	// Đóng fingerprint vào value để refresh sau so được — cookie bị trộm sang máy
	// khác sẽ không refresh được (mode enforce). Fingerprint đã sanitize ở handler.
	val := hash
	if fingerprint != "" {
		val = hash + rtFPSep + fingerprint
	}

	ctx := context.Background()
	rtsKey := s.k(fmt.Sprintf("rts:%d", userID))
	pipe := s.redis.Pipeline()
	pipe.Set(ctx, s.k(fmt.Sprintf("rt:%d:%s", userID, rid)), val, s.cfg.RefreshTTL)
	pipe.Set(ctx, s.k("rtidx:"+rid), strconv.FormatUint(uint64(userID), 10), s.cfg.RefreshTTL)
	// Index refresh token theo user (rid → expiry) — nền tảng của device limit.
	// Score = expiry để cleanup by score; thứ tự score == thứ tự phát hành/rotation
	// nên "oldest" = thiết bị ít refresh nhất (LRU).
	pipe.ZAdd(ctx, rtsKey, redis.Z{Score: float64(time.Now().Add(s.cfg.RefreshTTL).Unix()), Member: rid})
	pipe.Expire(ctx, rtsKey, s.cfg.RefreshTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return "", err
	}

	if s.cfg.MaxSessions > 0 {
		s.enforceRefreshLimit(userID)
	}

	return rid + "." + raw, nil
}

// enforceRefreshLimit giữ tối đa MaxSessions refresh token / user — device limit
// thật. Access-session limit một mình vô nghĩa: thiết bị bị revoke access chỉ cần
// silent-refresh là sống lại. Rotation không trigger evict (old bị invalidate
// trước khi generate new → count không đổi); chỉ login mới vượt limit.
func (s *Service) enforceRefreshLimit(userID uint) {
	ctx := context.Background()
	key := s.k(fmt.Sprintf("rts:%d", userID))

	// Cleanup expired
	s.redis.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(time.Now().Unix(), 10))

	count, _ := s.redis.ZCard(ctx, key).Result()
	if int(count) <= s.cfg.MaxSessions {
		return
	}

	// Evict thiết bị ít refresh nhất — refresh của nó chết ngay, access token
	// còn lại tự hết hạn trong vòng AccessTTL.
	excess := int(count) - s.cfg.MaxSessions
	rids, _ := s.redis.ZRange(ctx, key, 0, int64(excess-1)).Result()
	for _, rid := range rids {
		s.invalidateRefresh(userID, rid)
	}
	if len(rids) > 0 {
		seclog.Event(nil, "device_evicted", seclog.UserID(userID), zap.Int("count", len(rids)))
	}
}

// ActiveFingerprints trả set fingerprint của các refresh token đang sống của user
// — FE đánh dấu thiết bị nào trong user_device còn phiên. Token legacy (không fp)
// không xuất hiện. Rẻ vì tối đa MaxSessions key.
func (s *Service) ActiveFingerprints(userID uint) map[string]bool {
	ctx := context.Background()
	out := map[string]bool{}
	pattern := s.k(fmt.Sprintf("rt:%d:*", userID))
	iter := s.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		val, err := s.redis.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}
		if i := strings.Index(val, rtFPSep); i >= 0 {
			out[val[i+len(rtFPSep):]] = true
		}
	}
	return out
}

// RevokeRefreshByFingerprint xoá mọi refresh token khớp fingerprint của user —
// "đăng xuất thiết bị này". Trả số token đã xoá.
func (s *Service) RevokeRefreshByFingerprint(userID uint, fingerprint string) int {
	if fingerprint == "" {
		return 0
	}
	ctx := context.Background()
	n := 0
	pattern := s.k(fmt.Sprintf("rt:%d:*", userID))
	iter := s.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		rtKey := iter.Val()
		val, err := s.redis.Get(ctx, rtKey).Result()
		if err != nil {
			continue
		}
		i := strings.Index(val, rtFPSep)
		if i < 0 || val[i+len(rtFPSep):] != fingerprint {
			continue
		}
		rid := rtKey[strings.LastIndex(rtKey, ":")+1:]
		s.invalidateRefresh(userID, rid)
		n++
	}
	return n
}

func (s *Service) invalidateRefresh(userID uint, rid string) {
	ctx := context.Background()
	s.redis.Del(ctx, s.k(fmt.Sprintf("rt:%d:%s", userID, rid)))
	s.redis.Del(ctx, s.k("rtidx:"+rid))
	// Giữ zset đồng bộ — member stale có score tương lai sẽ chiếm oan slot device.
	s.redis.ZRem(ctx, s.k(fmt.Sprintf("rts:%d", userID)), rid)
}

// --- Revoke ---

func (s *Service) Revoke(jti string, ttl time.Duration) {
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	s.redis.Set(context.Background(), s.k("revoked:"+jti), "1", ttl)
}

func (s *Service) RevokeAllSessions(userID uint) {
	ctx := context.Background()
	key := s.k(fmt.Sprintf("sess:%d", userID))

	// Cleanup expired
	s.redis.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(time.Now().Unix(), 10))

	// Revoke all remaining — TTL blacklist lấy từ score (= expTs của token).
	// TTL cố định ngắn hơn AccessTTL sẽ mở "resurrection window": entry blacklist
	// hết hạn trước token → token đã revoke dùng lại được phần đời còn lại.
	entries, _ := s.redis.ZRangeWithScores(ctx, key, 0, -1).Result()
	for _, e := range entries {
		jti, _ := e.Member.(string)
		if jti == "" {
			continue
		}
		if ttl := time.Until(time.Unix(int64(e.Score), 0)); ttl > 0 {
			s.Revoke(jti, ttl)
		}
	}
	s.redis.Del(ctx, key)
}

// RevokeAllRefreshTokens xoá toàn bộ refresh token của user bằng SCAN rt:{userID}:*.
// Gọi kèm RevokeAllSessions khi đổi mật khẩu — nếu chỉ revoke access token, FE vẫn
// có thể silent-refresh để lấy access token mới và bypass revocation.
func (s *Service) RevokeAllRefreshTokens(userID uint) {
	ctx := context.Background()
	// SCAN thay vì đọc rts zset: cover cả token cũ phát hành trước khi có zset index.
	pattern := s.k(fmt.Sprintf("rt:%d:*", userID))
	iter := s.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		rtKey := iter.Val()
		rid := rtKey[strings.LastIndex(rtKey, ":")+1:]
		s.redis.Del(ctx, rtKey)
		s.redis.Del(ctx, s.k("rtidx:"+rid))
	}
	s.redis.Del(ctx, s.k(fmt.Sprintf("rts:%d", userID)))
}

func (s *Service) isRevoked(jti string) bool {
	val, _ := s.redis.Get(context.Background(), s.k("revoked:"+jti)).Result()
	return val != ""
}

// IsRevoked exposes the jti blacklist check for the auth middleware.
// Fail-open on Redis error (consistent with login rate-limit behaviour).
func (s *Service) IsRevoked(jti string) bool {
	return s.isRevoked(jti)
}

// --- Sessions ---

func (s *Service) buildAccessJWT(userID uint, ip string) (string, string, int64, error) {
	now := time.Now()
	exp := now.Add(s.cfg.AccessTTL)
	jti := randomHex(16)

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Audience:  jwt.ClaimStrings{s.cfg.Audience},
			Subject:   strconv.FormatUint(uint64(userID), 10),
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		UID:    userID,
		CfgVer: s.cfg.CfgVer,
	}

	if s.cfg.IPBinding && ip != "" {
		claims.IPHash = sha256Hex(ip)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", "", 0, err
	}

	return tokenStr, jti, exp.Unix(), nil
}

func (s *Service) registerSession(userID uint, jti string, expTs int64) {
	ctx := context.Background()
	key := s.k(fmt.Sprintf("sess:%d", userID))
	s.redis.ZAdd(ctx, key, redis.Z{Score: float64(expTs), Member: jti})
	s.redis.Expire(ctx, key, s.cfg.RefreshTTL)
}

func (s *Service) enforceSessionLimit(userID uint) {
	ctx := context.Background()
	key := s.k(fmt.Sprintf("sess:%d", userID))

	// Cleanup expired
	s.redis.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(time.Now().Unix(), 10))

	count, _ := s.redis.ZCard(ctx, key).Result()
	if int(count) <= s.cfg.MaxSessions {
		return
	}

	// Remove oldest — TTL blacklist theo score (xem RevokeAllSessions): session
	// "oldest" vẫn có thể còn gần đủ AccessTTL nếu user login nhiều lần liên tiếp.
	excess := int(count) - s.cfg.MaxSessions
	entries, _ := s.redis.ZRangeWithScores(ctx, key, 0, int64(excess-1)).Result()
	for _, e := range entries {
		jti, _ := e.Member.(string)
		if jti == "" {
			continue
		}
		if ttl := time.Until(time.Unix(int64(e.Score), 0)); ttl > 0 {
			s.Revoke(jti, ttl)
		}
		s.redis.ZRem(ctx, key, e.Member)
	}
}

// --- Helpers ---

// DeviceBindingMode expose config cho handler quyết định nhánh log/enforce.
func (s *Service) DeviceBindingMode() string { return s.cfg.DeviceBindingMode }

func (s *Service) k(suffix string) string {
	return s.cfg.AuthPrefix + suffix
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RandomHex — n byte crypto/rand → 2n hex char. Exported wrapper dùng cho các nơi ngoài
// package token cần token ngẫu nhiên (site_key, identity_secret...) và cần biết crypto/rand
// có lỗi hay không, khác `randomHex` nội bộ (bỏ qua lỗi Read, dùng cho case không critical).
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

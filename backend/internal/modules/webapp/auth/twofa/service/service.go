// Package service — TOTP 2FA mechanics: enroll, verify, backup codes.
// Secret mã hóa AES-GCM bằng TOTP_ENC_KEY (tách khỏi key config chung).
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/crypto"

	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

const (
	totpEncKeyEnv   = "TOTP_ENC_KEY"
	backupCodeCount = 10
	verifyMaxFails  = 5
	verifyWindow    = 5 * time.Minute
)

type Service struct {
	DB     *gorm.DB
	Cache  *cache.RedisCache
	Issuer string // "Cluvix" — hiển thị trong app authenticator
}

// Setup sinh TOTP secret mới (pending, chưa enable), trả otpauth URI (QR) + secret (nhập tay).
// Từ chối nếu đã bật — ghi đè secret đang dùng sẽ làm authenticator của user vô hiệu
// (brick 2FA). Muốn đổi phải Disable trước.
func (s *Service) Setup(userID uint, username string) (uri, secret string, err error) {
	if s.IsEnabled(userID) {
		return "", "", fmt.Errorf("2fa already enabled")
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: s.Issuer, AccountName: username})
	if err != nil {
		return "", "", err
	}
	enc, err := crypto.EncryptAESGCMKey(key.Secret(), totpEncKeyEnv)
	if err != nil {
		return "", "", err
	}
	// Lưu secret pending, KHÔNG bật (totp_enabled giữ nguyên) — chỉ bật sau VerifySetup.
	if err := s.DB.Table("user").Where("id = ?", userID).
		Update("totp_secret", enc).Error; err != nil {
		return "", "", err
	}
	return key.URL(), key.Secret(), nil
}

// VerifySetup xác nhận mã đầu tiên → bật 2FA + sinh backup codes (trả plaintext 1 lần).
func (s *Service) VerifySetup(userID uint, code string) ([]string, error) {
	secret, err := s.loadSecret(userID)
	if err != nil {
		return nil, err
	}
	if !totp.Validate(code, secret) {
		return nil, fmt.Errorf("invalid code")
	}
	now := time.Now().Unix()
	if err := s.DB.Table("user").Where("id = ?", userID).Updates(map[string]any{
		"totp_enabled":     1,
		"totp_enrolled_at": now,
	}).Error; err != nil {
		return nil, err
	}
	return s.regenBackupCodes(userID)
}

// VerifyCode dùng khi login: khớp TOTP HOẶC 1 backup code chưa dùng (đánh dấu đã dùng).
// Rate-limit verifyMaxFails/verifyWindow theo user chống brute 6 số.
func (s *Service) VerifyCode(userID uint, code string) bool {
	if !s.checkVerifyRate(userID) {
		return false
	}
	secret, err := s.loadSecret(userID)
	if err == nil && secret != "" && totp.Validate(code, secret) {
		s.clearVerifyFails(userID)
		return true
	}
	if s.consumeBackupCode(userID, code) {
		s.clearVerifyFails(userID)
		return true
	}
	s.recordVerifyFail(userID)
	return false
}

// Disable tắt 2FA (yêu cầu mã hợp lệ) — xóa secret + backup codes.
func (s *Service) Disable(userID uint, code string) error {
	if !s.VerifyCode(userID, code) {
		return fmt.Errorf("invalid code")
	}
	return s.clear(userID)
}

// Reset (super-user) xóa 2FA của user khi mất thiết bị.
func (s *Service) Reset(userID uint) error {
	return s.clear(userID)
}

func (s *Service) clear(userID uint) error {
	if err := s.DB.Table("user").Where("id = ?", userID).Updates(map[string]any{
		"totp_secret":      nil,
		"totp_enabled":     0,
		"totp_enrolled_at": nil,
	}).Error; err != nil {
		return err
	}
	s.DB.Table("user_2fa_backup_code").Where("user_id = ?", userID).Delete(nil)
	return nil
}

// IsEnabled — user đã bật 2FA chưa.
func (s *Service) IsEnabled(userID uint) bool {
	var enabled int
	s.DB.Table("user").Select("totp_enabled").Where("id = ?", userID).Scan(&enabled)
	return enabled == 1
}

// --- Helpers ---

func (s *Service) loadSecret(userID uint) (string, error) {
	var enc *string
	if err := s.DB.Table("user").Select("totp_secret").Where("id = ?", userID).Scan(&enc).Error; err != nil {
		return "", err
	}
	if enc == nil || *enc == "" {
		return "", fmt.Errorf("no secret")
	}
	return crypto.DecryptAESGCMKey(*enc, totpEncKeyEnv)
}

func (s *Service) regenBackupCodes(userID uint) ([]string, error) {
	s.DB.Table("user_2fa_backup_code").Where("user_id = ?", userID).Delete(nil)
	now := time.Now().Unix()
	plain := make([]string, 0, backupCodeCount)
	rows := make([]map[string]any, 0, backupCodeCount)
	for i := 0; i < backupCodeCount; i++ {
		c := randomCode()
		plain = append(plain, c)
		rows = append(rows, map[string]any{
			"user_id": userID, "code_hash": sha256Hex(c), "created_at": now,
		})
	}
	if err := s.DB.Table("user_2fa_backup_code").Create(rows).Error; err != nil {
		return nil, err
	}
	return plain, nil
}

// consumeBackupCode: nếu code khớp 1 backup chưa dùng → đánh dấu used, trả true.
func (s *Service) consumeBackupCode(userID uint, code string) bool {
	h := sha256Hex(strings.TrimSpace(code))
	res := s.DB.Table("user_2fa_backup_code").
		Where("user_id = ? AND code_hash = ? AND used_at IS NULL", userID, h).
		Update("used_at", time.Now().Unix())
	return res.RowsAffected > 0
}

// --- Rate limit verify (Redis) ---

func (s *Service) verifyKey(userID uint) string {
	return s.Cache.Prefix() + fmt.Sprintf("2fa:verify_fail:%d", userID)
}
func (s *Service) checkVerifyRate(userID uint) bool {
	if s.Cache == nil {
		return true
	}
	n, err := s.Cache.Client().Get(context.Background(), s.verifyKey(userID)).Int64()
	if err != nil {
		return true
	}
	return n < verifyMaxFails
}

// VerifyRateLimited: handler gọi TRƯỚC VerifyCode để phân biệt "đang bị khóa tạm"
// với "nhập sai mã" (VerifyCode không phân biệt 2 case, luôn trả false như nhau).
// Trả (limited, retryAfterSeconds) — retryAfterSeconds lấy từ TTL key Redis hiện có,
// không phải counter mới, nên không ảnh hưởng ngưỡng verifyMaxFails/verifyWindow.
func (s *Service) VerifyRateLimited(userID uint) (bool, int64) {
	if s.Cache == nil || s.checkVerifyRate(userID) {
		return false, 0
	}
	ttl, err := s.Cache.Client().TTL(context.Background(), s.verifyKey(userID)).Result()
	if err != nil || ttl <= 0 {
		return true, 0
	}
	return true, int64(ttl.Seconds())
}
func (s *Service) recordVerifyFail(userID uint) {
	if s.Cache == nil {
		return
	}
	ctx := context.Background()
	pipe := s.Cache.Client().Pipeline()
	pipe.Incr(ctx, s.verifyKey(userID))
	pipe.Expire(ctx, s.verifyKey(userID), verifyWindow)
	_, _ = pipe.Exec(ctx)
}
func (s *Service) clearVerifyFails(userID uint) {
	if s.Cache != nil {
		s.Cache.Client().Del(context.Background(), s.verifyKey(userID))
	}
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// randomCode sinh backup code dạng "a1b2c3d4" (8 hex, dễ gõ).
func randomCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

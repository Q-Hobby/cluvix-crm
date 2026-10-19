package token

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// TestChallengeTokenNotAcceptedAsAccess — CRITICAL: challenge token KHÔNG được verify
// bằng access-token secret (cfg.Secret) → JWTAuth (dùng cùng secret) sẽ reject → không
// thể dùng challenge token làm Bearer để bypass 2FA.
func TestChallengeTokenNotAcceptedAsAccess(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	s := New(Config{Secret: "sec", AccessTTL: time.Minute, RefreshTTL: time.Hour, AuthPrefix: "t:", Audience: "aud"}, rc)

	chal, err := s.IssueChallengeToken(7, "fp")
	if err != nil {
		t.Fatal(err)
	}

	// 1) Round-trip challenge vẫn đúng.
	uid, fp, err := s.VerifyChallengeToken(chal)
	if err != nil || uid != 7 || fp != "fp" {
		t.Fatalf("challenge round-trip: uid=%d fp=%s err=%v", uid, fp, err)
	}

	// 2) DecodeAndValidate (dùng cfg.Secret — GIỐNG JWTAuth) PHẢI reject challenge token.
	if _, err := s.DecodeAndValidate(chal, ""); err == nil {
		t.Fatal("SECURITY: challenge token verify được bằng access secret → có thể làm Bearer bypass 2FA")
	}

	// 3) Xác nhận cụ thể: parse challenge bằng đúng cfg.Secret → sai chữ ký.
	_, perr := jwt.Parse(chal, func(*jwt.Token) (interface{}, error) { return []byte("sec"), nil })
	if perr == nil {
		t.Fatal("SECURITY: challenge token có chữ ký hợp lệ với access secret")
	}
}

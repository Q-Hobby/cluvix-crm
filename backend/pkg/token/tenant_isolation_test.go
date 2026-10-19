package token

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTenantService — 2 khách dùng CHUNG TOKEN_SECRET + Redis, chỉ khác issuer/audience (chứa DB_NAME).
func newTenantService(t *testing.T, rc *redis.Client, db string) *Service {
	t.Helper()
	return New(Config{
		Secret: "shared-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour,
		AuthPrefix: "C:prod:" + db + ":auth:",
		Issuer:     "C.prod." + db + ".issuer", Audience: "C.prod." + db + ".aud",
	}, rc)
}

// TestAccessToken_KhacKhach_BiTuChoi — R-1: token ký đúng secret nhưng issuer của khách khác ⇒ reject.
func TestAccessToken_KhacKhach_BiTuChoi(t *testing.T) {
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	a := newTenantService(t, rc, "khach_a")
	b := newTenantService(t, rc, "khach_b")

	res, err := a.IssueAccessOnly(1, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.DecodeAndValidate(res.AccessToken, ""); err != nil {
		t.Fatalf("token hợp lệ ở chính khách A phải qua: %v", err)
	}
	if _, err := b.DecodeAndValidate(res.AccessToken, ""); err == nil {
		t.Fatal("SECURITY: access token của khách A được khách B chấp nhận (chung TOKEN_SECRET)")
	}
}

// TestChallengeToken_KhacKhach_BiTuChoi — challenge 2FA cũng mang issuer khách.
func TestChallengeToken_KhacKhach_BiTuChoi(t *testing.T) {
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	a := newTenantService(t, rc, "khach_a")
	b := newTenantService(t, rc, "khach_b")

	chal, err := a.IssueChallengeToken(1, "fp")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.VerifyChallengeToken(chal); err != nil {
		t.Fatalf("challenge ở chính khách A phải qua: %v", err)
	}
	if _, _, err := b.VerifyChallengeToken(chal); err == nil {
		t.Fatal("SECURITY: challenge token của khách A được khách B chấp nhận")
	}
}

// TestClaimsMatchTenant_FailClosed — cấu hình thiếu issuer/audience ⇒ KHÔNG coi là "bỏ qua kiểm".
func TestClaimsMatchTenant_FailClosed(t *testing.T) {
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	s := New(Config{Secret: "s", AccessTTL: time.Minute, RefreshTTL: time.Hour}, rc)
	res, err := s.IssueAccessOnly(1, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecodeAndValidate(res.AccessToken, ""); err == nil {
		t.Fatal("issuer/audience rỗng phải fail-closed")
	}
}

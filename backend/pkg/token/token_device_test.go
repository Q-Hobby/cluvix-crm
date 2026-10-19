package token

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestService dựng token.Service trên miniredis (in-memory, không cần Redis thật).
func newTestService(t *testing.T, mode string) *Service {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return New(Config{
		Secret:            "test-secret",
		AccessTTL:         15 * time.Minute,
		RefreshTTL:        24 * time.Hour,
		MaxSessions:       3,
		AuthPrefix:        "test:auth:",
		Issuer:            "test.dev.db.issuer",
		Audience:          "test.dev.db.aud",
		DeviceBindingMode: mode,
	}, rc)
}

// TestFingerprintBoundRotation — refresh với đúng fingerprint (enforce) rotate thành công.
func TestFingerprintBoundRotation(t *testing.T) {
	s := newTestService(t, "enforce")
	res, err := s.IssueAccess(1, "1.1.1.1", "fpA")
	if err != nil {
		t.Fatalf("IssueAccess: %v", err)
	}

	uid, newRT, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA")
	if err != nil {
		t.Fatalf("rotate with matching fp should succeed: %v", err)
	}
	if uid != 1 {
		t.Fatalf("uid = %d, want 1", uid)
	}
	// Token mới vẫn refresh được với cùng fingerprint.
	if _, _, err := s.VerifyAndRotateRefresh(newRT, "fpA"); err != nil {
		t.Fatalf("second rotate: %v", err)
	}
	// Token cũ đã bị invalidate.
	if _, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA"); err == nil {
		t.Fatal("old refresh token must be invalid after rotation")
	}
}

// TestEnforceRejectsMismatch — enforce mode: fingerprint lệch → ErrFingerprintMismatch.
func TestEnforceRejectsMismatch(t *testing.T) {
	s := newTestService(t, "enforce")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")

	_, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpB")
	if err != ErrFingerprintMismatch {
		t.Fatalf("err = %v, want ErrFingerprintMismatch", err)
	}
	// Token KHÔNG bị rotate khi mismatch → vẫn còn (dùng đúng fp vẫn refresh được).
	if _, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA"); err != nil {
		t.Fatalf("token should survive a rejected mismatch: %v", err)
	}
}

// TestLogModeAllowsMismatch — log mode: mismatch KHÔNG chặn (chỉ để handler seclog).
func TestLogModeAllowsMismatch(t *testing.T) {
	s := newTestService(t, "log")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")

	if _, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpB"); err != nil {
		t.Fatalf("log mode must not block mismatch: %v", err)
	}

	matched, hadFP := s.RefreshFingerprintMatches("dummy", "x")
	_ = matched
	_ = hadFP
}

// TestLegacyTokenUpgrades — token không fingerprint (legacy) bỏ qua enforce, tự
// nâng cấp ở rotation kế: token mới mang fingerprint client và enforce từ đó.
func TestLegacyTokenUpgrades(t *testing.T) {
	s := newTestService(t, "enforce")
	// fp="" mô phỏng token phát hành trước khi có device binding.
	res, _ := s.IssueAccess(1, "1.1.1.1", "")

	uid, upgraded, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpNew")
	if err != nil {
		t.Fatalf("legacy token must pass enforce: %v", err)
	}
	if uid != 1 {
		t.Fatalf("uid = %d", uid)
	}
	// Sau nâng cấp, token mới đã bound "fpNew" → fp khác bị chặn.
	if _, _, err := s.VerifyAndRotateRefresh(upgraded, "fpOther"); err != ErrFingerprintMismatch {
		t.Fatalf("upgraded token must enforce: err = %v", err)
	}
}

// TestConsumeRefresh — logout: invalidate không re-issue.
func TestConsumeRefresh(t *testing.T) {
	s := newTestService(t, "off")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")

	if err := s.ConsumeRefresh(res.RefreshToken); err != nil {
		t.Fatalf("ConsumeRefresh: %v", err)
	}
	if _, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA"); err == nil {
		t.Fatal("token must be dead after ConsumeRefresh")
	}
	if fps := s.ActiveFingerprints(1); len(fps) != 0 {
		t.Fatalf("no active fingerprints expected, got %v", fps)
	}
}

// TestDeviceLimitCap — login vượt MaxSessions giữ đúng cap (device limit thật).
// Không assert victim cụ thể: nhiều login trong cùng giây có score bằng nhau nên
// tie-break theo rid (không deterministic) — cap mới là bất biến bảo mật.
func TestDeviceLimitCap(t *testing.T) {
	s := newTestService(t, "off")
	for i, fp := range []string{"fp1", "fp2", "fp3", "fp4", "fp5"} {
		if _, err := s.IssueAccess(1, "1.1.1.1", fp); err != nil {
			t.Fatalf("IssueAccess #%d: %v", i, err)
		}
	}
	fps := s.ActiveFingerprints(1)
	if len(fps) != 3 {
		t.Fatalf("active fingerprints = %d, want 3 (MaxSessions)", len(fps))
	}
}

// TestRevokeByFingerprint — "đăng xuất thiết bị này" xoá đúng refresh token khớp fp.
func TestRevokeByFingerprint(t *testing.T) {
	s := newTestService(t, "off")
	rtA, _ := s.IssueAccess(1, "1.1.1.1", "fpA")
	s.IssueAccess(1, "1.1.1.1", "fpB")

	n := s.RevokeRefreshByFingerprint(1, "fpA")
	if n != 1 {
		t.Fatalf("revoked = %d, want 1", n)
	}
	if _, _, err := s.VerifyAndRotateRefresh(rtA.RefreshToken, "fpA"); err == nil {
		t.Fatal("fpA refresh token must be dead after revoke")
	}
	fps := s.ActiveFingerprints(1)
	if len(fps) != 1 || !fps["fpB"] {
		t.Fatalf("active = %v, want only fpB", fps)
	}
	// Revoke fingerprint không tồn tại → 0, không panic.
	if got := s.RevokeRefreshByFingerprint(1, "nope"); got != 0 {
		t.Fatalf("revoke nonexistent = %d, want 0", got)
	}
}

// TestActiveFingerprintsIgnoresLegacy — token không fp không xuất hiện trong active set.
func TestActiveFingerprintsIgnoresLegacy(t *testing.T) {
	s := newTestService(t, "off")
	s.IssueAccess(1, "1.1.1.1", "")    // legacy — no fp
	s.IssueAccess(1, "1.1.1.1", "fpB") // has fp

	fps := s.ActiveFingerprints(1)
	if len(fps) != 1 || !fps["fpB"] {
		t.Fatalf("active = %v, want only fpB (legacy ignored)", fps)
	}
}

// TestRevocationBlacklist — access token bị Revoke thì IsRevoked = true.
func TestRevocationBlacklist(t *testing.T) {
	s := newTestService(t, "off")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")
	claims, err := s.DecodeAndValidate(res.AccessToken, "1.1.1.1")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.IsRevoked(claims.ID) {
		t.Fatal("fresh token must not be revoked")
	}
	s.Revoke(claims.ID, time.Minute)
	if !s.IsRevoked(claims.ID) {
		t.Fatal("token must be revoked after Revoke")
	}
}

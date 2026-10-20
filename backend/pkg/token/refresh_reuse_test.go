package token

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// backdateRotation lùi mốc xoay vòng của token về `ago` trước — giả lập trình lại sau grace.
func backdateRotation(t *testing.T, s *Service, refreshToken string, ago time.Duration) {
	t.Helper()
	ctx := context.Background()
	key := s.k("rtused:" + refreshToken[:16])
	val, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("rtused chưa được ghi: %v", err)
	}
	parts := strings.Split(val, ":")
	parts[2] = fmt.Sprint(time.Now().Add(-ago).Unix())
	if err := s.redis.Set(ctx, key, strings.Join(parts, ":"), time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
}

// TestRefreshRotation_ChiMotRequestThang — N request cùng token chạy song song: đúng 1 cái nhận token
// mới, còn lại ErrRefreshRaced (trước đây cả N cùng qua verify ⇒ N phiên song song).
func TestRefreshRotation_ChiMotRequestThang(t *testing.T) {
	s := newTestService(t, "off")
	res, err := s.IssueAccess(1, "1.1.1.1", "fpA")
	if err != nil {
		t.Fatal(err)
	}
	const n = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, raced := 0, 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrRefreshRaced):
				raced++
			default:
				t.Errorf("lỗi ngoài dự kiến: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || raced != n-1 {
		t.Fatalf("ok=%d raced=%d, muốn ok=1 raced=%d", ok, raced, n-1)
	}
}

// TestRefreshReuse_TrongGrace — trình lại token cũ ngay sau khi xoay vòng (2 tab) ⇒ ErrRefreshRaced,
// token mới vẫn dùng được (không thu hồi phiên).
func TestRefreshReuse_TrongGrace(t *testing.T) {
	s := newTestService(t, "off")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")
	_, newRT, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA"); !errors.Is(err, ErrRefreshRaced) {
		t.Fatalf("err = %v, muốn ErrRefreshRaced", err)
	}
	if _, _, err := s.VerifyAndRotateRefresh(newRT, "fpA"); err != nil {
		t.Fatalf("token mới phải còn dùng được: %v", err)
	}
}

// TestRefreshReuse_QuaGrace — token cũ bị trình lại sau grace ⇒ ErrRefreshReused + thu hồi MỌI
// refresh token của user (kể cả token mới và phiên ở thiết bị khác). User khác không bị ảnh hưởng.
func TestRefreshReuse_QuaGrace(t *testing.T) {
	s := newTestService(t, "off")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")
	other, _ := s.IssueAccess(1, "2.2.2.2", "fpB")
	stranger, _ := s.IssueAccess(2, "3.3.3.3", "fpC")
	_, newRT, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA")
	if err != nil {
		t.Fatal(err)
	}
	backdateRotation(t, s, res.RefreshToken, refreshReuseGrace+time.Second)

	if _, _, err := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA"); !errors.Is(err, ErrRefreshReused) {
		t.Fatalf("err = %v, muốn ErrRefreshReused", err)
	}
	for name, rt := range map[string]string{"token mới": newRT, "thiết bị khác": other.RefreshToken} {
		if _, _, err := s.VerifyAndRotateRefresh(rt, ""); err == nil {
			t.Fatalf("%s phải bị thu hồi sau khi phát hiện dùng lại", name)
		}
	}
	if _, _, err := s.VerifyAndRotateRefresh(stranger.RefreshToken, "fpC"); err != nil {
		t.Fatalf("user khác không được bị thu hồi: %v", err)
	}
}

// TestRefreshReuse_TokenSaiHash — rid đúng nhưng phần bí mật sai ⇒ lỗi thường, KHÔNG thu hồi phiên
// (không cho ai biết rid mà không có token gây đăng xuất hàng loạt).
func TestRefreshReuse_TokenSaiHash(t *testing.T) {
	s := newTestService(t, "off")
	res, _ := s.IssueAccess(1, "1.1.1.1", "fpA")
	_, newRT, _ := s.VerifyAndRotateRefresh(res.RefreshToken, "fpA")
	backdateRotation(t, s, res.RefreshToken, refreshReuseGrace+time.Second)

	forged := res.RefreshToken[:17] + strings.Repeat("0", 64)
	_, _, err := s.VerifyAndRotateRefresh(forged, "fpA")
	if err == nil || errors.Is(err, ErrRefreshReused) || errors.Is(err, ErrRefreshRaced) {
		t.Fatalf("err = %v, muốn lỗi token thường", err)
	}
	if _, _, err := s.VerifyAndRotateRefresh(newRT, "fpA"); err != nil {
		t.Fatalf("token mới không được bị thu hồi: %v", err)
	}
}

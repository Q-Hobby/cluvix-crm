package cache

import (
	"context"
	"testing"
	"time"
)

// newTestCache — kết nối Redis local thật (127.0.0.1:6379, khớp .env dev), prefix riêng test:
// để không đụng namespace cache thật. Redis KHÔNG phải "DB thật" theo nghĩa Hard Rules CẤM
// test/migrate chạm DB thật (rule đó nói về MySQL DB_NAME) — nhưng vẫn dọn key sau mỗi test để
// không rác cache instance dùng chung.
func newTestCache(t *testing.T) *RedisCache {
	t.Helper()
	c := New("127.0.0.1:6379", "", "test:cache_redis_test:", 0)
	if err := c.Ping(); err != nil {
		t.Skipf("Redis local không sẵn sàng (%v) — skip test cần Redis thật", err)
	}
	return c
}

func TestSetNX_OnlyOneOwner(t *testing.T) {
	c := newTestCache(t)
	key := "lock:demo"
	defer c.Delete(key)

	ok1, err := c.SetNX(key, "owner-A", 5*time.Second)
	if err != nil {
		t.Fatalf("SetNX owner-A: %v", err)
	}
	if !ok1 {
		t.Fatal("owner-A phải chiếm được lock (key chưa tồn tại)")
	}

	ok2, err := c.SetNX(key, "owner-B", 5*time.Second)
	if err != nil {
		t.Fatalf("SetNX owner-B: %v", err)
	}
	if ok2 {
		t.Fatal("owner-B KHÔNG được chiếm lock khi owner-A đang giữ")
	}
}

func TestExtendLock_FailsWhenNotOwner(t *testing.T) {
	c := newTestCache(t)
	key := "lock:extend"
	defer c.Delete(key)

	if ok, err := c.SetNX(key, "owner-A", 2*time.Second); err != nil || !ok {
		t.Fatalf("setup SetNX owner-A thất bại: ok=%v err=%v", ok, err)
	}

	// owner-B thử renew lock của owner-A — PHẢI thất bại (không phải chủ).
	extended, err := c.ExtendLock(key, "owner-B", 10*time.Second)
	if err != nil {
		t.Fatalf("ExtendLock owner-B: %v", err)
	}
	if extended {
		t.Fatal("owner-B KHÔNG được renew lock của owner-A")
	}

	// owner-A renew chính lock của mình — PHẢI thành công.
	extended, err = c.ExtendLock(key, "owner-A", 10*time.Second)
	if err != nil {
		t.Fatalf("ExtendLock owner-A: %v", err)
	}
	if !extended {
		t.Fatal("owner-A phải renew được lock của chính mình")
	}
}

func TestExtendLock_FailsAfterExpiry(t *testing.T) {
	c := newTestCache(t)
	key := "lock:expiry"
	defer c.Delete(key)

	if ok, err := c.SetNX(key, "owner-A", 500*time.Millisecond); err != nil || !ok {
		t.Fatalf("setup SetNX owner-A thất bại: ok=%v err=%v", ok, err)
	}
	time.Sleep(700 * time.Millisecond) // chờ TTL hết hạn thật

	extended, err := c.ExtendLock(key, "owner-A", 10*time.Second)
	if err != nil {
		t.Fatalf("ExtendLock sau khi hết hạn: %v", err)
	}
	if extended {
		t.Fatal("KHÔNG được renew lock đã hết hạn (owner-A không còn là chủ thật)")
	}

	// Xác nhận owner khác giờ chiếm được (đúng hành vi failover — story-02 đã đo thật ở mức
	// integration; test này chỉ verify đơn vị ExtendLock không tự tạo lại key).
	ok, err := c.SetNX(key, "owner-B", 5*time.Second)
	if err != nil {
		t.Fatalf("SetNX owner-B sau expiry: %v", err)
	}
	if !ok {
		t.Fatal("owner-B phải chiếm được lock sau khi TTL owner-A hết hạn")
	}
}

func TestReleaseLock_OnlyOwnerCanRelease(t *testing.T) {
	c := newTestCache(t)
	key := "lock:release"
	defer c.Delete(key)

	if ok, err := c.SetNX(key, "owner-A", 5*time.Second); err != nil || !ok {
		t.Fatalf("setup SetNX owner-A thất bại: ok=%v err=%v", ok, err)
	}

	// owner-B thử release lock của owner-A — PHẢI thất bại, key vẫn còn.
	released, err := c.ReleaseLock(key, "owner-B")
	if err != nil {
		t.Fatalf("ReleaseLock owner-B: %v", err)
	}
	if released {
		t.Fatal("owner-B KHÔNG được release lock của owner-A")
	}
	var dest string
	if err := c.Get(key, &dest); err == nil {
		t.Fatal("Get qua Get() sẽ lỗi vì value không phải JSON — dùng Client() để verify raw thay vì Get")
	}
	raw, err := c.Client().Get(context.Background(), c.k(key)).Result()
	if err != nil || raw != "owner-A" {
		t.Fatalf("key phải vẫn còn value owner-A sau khi owner-B release thất bại, got raw=%q err=%v", raw, err)
	}

	// owner-A tự release — PHẢI thành công, key biến mất.
	released, err = c.ReleaseLock(key, "owner-A")
	if err != nil {
		t.Fatalf("ReleaseLock owner-A: %v", err)
	}
	if !released {
		t.Fatal("owner-A phải release được lock của chính mình")
	}
	if _, err := c.Client().Get(context.Background(), c.k(key)).Result(); err == nil {
		t.Fatal("key phải không còn tồn tại sau khi owner-A release thành công")
	}
}

// DeletePatternExcept (S-11): xoá cache app nhưng GIỮ key thuộc keepPrefixes (rate-limit/2FA).
// Đối chứng 2 chiều: key thường PHẢI mất, key bảo vệ PHẢI còn.
func TestDeletePatternExcept_KeepsProtectedPrefixes(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()
	keys := map[string]bool{ // key → phải còn sau khi xoá?
		"user_scope:7":            false,
		"rbac:user:7":             false,
		"auth:login_fail:1.2.3.4": true,
		"2fa:verify_fail:7":       true,
	}
	for k := range keys {
		if err := c.client.Set(ctx, c.k(k), "1", time.Minute).Err(); err != nil {
			t.Fatalf("seed %s: %v", k, err)
		}
		defer c.Delete(k)
	}
	if n := c.DeletePatternExcept("*", "auth:", "2fa:"); n < 2 {
		t.Fatalf("phải xoá ≥2 key thường, xoá %d", n)
	}
	for k, keep := range keys {
		exists := c.client.Exists(ctx, c.k(k)).Val() == 1
		if exists != keep {
			t.Fatalf("key %s: còn=%v, mong đợi còn=%v", k, exists, keep)
		}
	}
}

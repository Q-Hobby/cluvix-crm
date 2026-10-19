package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client    *redis.Client
	keyPrefix string
}

func New(addr, password, prefix string, db int) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
		keyPrefix: prefix,
	}
}

// Get lấy value từ cache, unmarshal vào dest.
func (c *RedisCache) Get(key string, dest interface{}) error {
	ctx := context.Background()
	val, err := c.client.Get(ctx, c.k(key)).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(val, dest)
}

// Set lưu value vào cache với TTL.
func (c *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.k(key), data, ttl).Err()
}

// GetOrSet — tương đương Yii::$app->cache->getOrSet()
func (c *RedisCache) GetOrSet(key string, ttl time.Duration, dest interface{}, fn func() (interface{}, error)) error {
	if err := c.Get(key, dest); err == nil {
		return nil
	}

	result, err := fn()
	if err != nil {
		return err
	}

	c.Set(key, result, ttl)

	// Marshal + Unmarshal to populate dest
	data, _ := json.Marshal(result)
	return json.Unmarshal(data, dest)
}

// Delete xoá 1 key.
func (c *RedisCache) Delete(key string) {
	c.client.Del(context.Background(), c.k(key))
}

// DeletePattern xoá theo pattern (SCAN + DEL).
func (c *RedisCache) DeletePattern(pattern string) {
	ctx := context.Background()
	iter := c.client.Scan(ctx, 0, c.k(pattern), 100).Iterator()
	for iter.Next(ctx) {
		c.client.Del(ctx, iter.Val())
	}
}

// DeletePatternExcept như DeletePattern nhưng GIỮ các key có prefix (đã bỏ keyPrefix) thuộc
// keepPrefixes — dùng cho "xoá cache app" mà không được đụng bộ đếm bảo mật nằm chung namespace
// (rate-limit đăng nhập `auth:`, 2FA `2fa:`…). Trả số key đã xoá.
func (c *RedisCache) DeletePatternExcept(pattern string, keepPrefixes ...string) int {
	ctx := context.Background()
	n := 0
	iter := c.client.Scan(ctx, 0, c.k(pattern), 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		keep := false
		for _, p := range keepPrefixes {
			if strings.HasPrefix(key, c.k(p)) {
				keep = true
				break
			}
		}
		if keep {
			continue
		}
		if c.client.Del(ctx, key).Val() > 0 {
			n++
		}
	}
	return n
}

// InvalidateTag — tương đương TagDependency::invalidate()
// Xoá tất cả keys có prefix tag
func (c *RedisCache) InvalidateTag(tags ...string) {
	for _, tag := range tags {
		c.DeletePattern(tag + ":*")
		c.Delete(tag)
	}
}

func (c *RedisCache) k(key string) string {
	return c.keyPrefix + key
}

// Client trả về raw redis client nếu cần.
func (c *RedisCache) Client() *redis.Client {
	return c.client
}

// releaseLockScript / extendLockScript — Lua so value TRƯỚC khi DEL/PEXPIRE, tránh 1 owner cũ
// (đã mất lock do TTL hết hạn, key đã bị owner mới SetNX chiếm) vô tình xoá/gia hạn nhầm lock
// đang thuộc owner khác. Mẫu đã verify chạy thật (omnichat-telegram-userbot story-02 spike,
// backend/spike/redis-lock/main.go) trước khi đưa vào đây làm code production.
var (
	releaseLockScript = redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)
	extendLockScript  = redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`)
)

// SetNX chiếm lock nếu key chưa tồn tại (ownership lock per-account — omnichat-telegram-userbot
// story-06). value NÊN là định danh instance/process (vd instance ID) để ExtendLock/ReleaseLock
// xác thực đúng chủ trước khi gia hạn/xoá.
func (c *RedisCache) SetNX(key, value string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(context.Background(), c.k(key), value, ttl).Result()
}

// ExtendLock gia hạn TTL lock CHỈ KHI value hiện tại khớp (vẫn đúng chủ) — trả false nếu lock đã
// hết hạn/bị owner khác SetNX chiếm trong lúc đó (KHÔNG được tự tạo lại, gọi lại SetNX nếu cần
// chiếm mới). false, nil = lock đã đổi chủ (không phải lỗi kỹ thuật).
func (c *RedisCache) ExtendLock(key, value string, ttl time.Duration) (bool, error) {
	res, err := extendLockScript.Run(context.Background(), c.client, []string{c.k(key)}, value, ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	return toInt64(res) != 0, nil
}

// ReleaseLock xoá lock CHỈ KHI value hiện tại khớp (graceful shutdown chủ động nhả lock — omnichat-
// telegram-userbot story-06 AC6). false, nil = lock không còn thuộc mình (không có gì để nhả, không
// phải lỗi).
func (c *RedisCache) ReleaseLock(key, value string) (bool, error) {
	res, err := releaseLockScript.Run(context.Background(), c.client, []string{c.k(key)}, value).Result()
	if err != nil {
		return false, err
	}
	return toInt64(res) != 0, nil
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

// Ping kiểm tra kết nối.
func (c *RedisCache) Ping() error {
	return c.client.Ping(context.Background()).Err()
}

func (c *RedisCache) Prefix() string {
	return c.keyPrefix
}

// FormatKey tạo key với format.
func (c *RedisCache) FormatKey(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

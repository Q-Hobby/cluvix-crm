package dexie

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"erpCluvix/pkg/cache"
)

const (
	OptionDexieVersions = "dexieVersions" // map[tableName]unixTimestamp
)

var cacheInstance *cache.RedisCache

// SetCache injects the cache used by dexie helpers.
// Call once from main.go after redis init.
func SetCache(c *cache.RedisCache) { cacheInstance = c }

// Cache returns the injected cache.
func Cache() *cache.RedisCache { return cacheInstance }

// ConfigSystemsGetOption đọc 1 option từ JSON column.
func ConfigSystemsGetOption(db *gorm.DB, option string) string {
	var value *string
	db.Raw("SELECT JSON_UNQUOTE(JSON_EXTRACT(content, ?)) FROM config_systems", "$."+option).Scan(&value)
	if value == nil {
		return ""
	}
	return *value
}

// ConfigSystemsSetOption ghi 1 option + xoá cache.
func ConfigSystemsSetOption(db *gorm.DB, option string, value interface{}) {
	db.Exec("UPDATE config_systems SET content = JSON_SET(COALESCE(content, '{}'), ?, ?)", "$."+option, value)

	if cacheInstance != nil {
		cacheInstance.Delete(option)
		cacheInstance.Delete("config:versions")
	}
}

// GetDexieVersions đọc map per-table versions từ cache → fallback DB.
func GetDexieVersions(db *gorm.DB) map[string]int64 {
	cacheKey := "config:dexie_versions"
	if cacheInstance != nil {
		var v map[string]int64
		if err := cacheInstance.Get(cacheKey, &v); err == nil && v != nil {
			return v
		}
	}

	raw := ConfigSystemsGetOption(db, OptionDexieVersions)
	versions := make(map[string]int64)
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &versions)
	}

	if cacheInstance != nil {
		cacheInstance.Set(cacheKey, versions, 5*time.Minute)
	}
	return versions
}

// BumpDexieTable bump per-table version cho 1 hoặc nhiều table.
// tableName phải khớp FE TABLE_REGISTRY.
// Sentinel "__all__" → FE full sync.
func BumpDexieTable(db *gorm.DB, tables ...string) {
	if len(tables) == 0 {
		return
	}
	now := time.Now().Unix()

	parts := make([]string, 0, len(tables))
	args := make([]interface{}, 0, len(tables)*2)
	for _, t := range tables {
		if t == "" {
			continue
		}
		parts = append(parts, "?, ?")
		args = append(args, t, now)
	}
	if len(parts) == 0 {
		return
	}

	sql := fmt.Sprintf(`
		UPDATE config_systems SET content = JSON_MERGE_PATCH(
			COALESCE(content, '{}'),
			JSON_OBJECT('%s', JSON_OBJECT(%s))
		)`,
		OptionDexieVersions, strings.Join(parts, ", "),
	)
	db.Exec(sql, args...)

	if cacheInstance != nil {
		cacheInstance.Delete("config:dexie_versions")
		cacheInstance.Delete("config:versions")
	}
}

// SerializeDexieVersionsHeader convert map → header value (sorted, comma-separated).
// Format: "companies=1745,branches=1746,units=1750"
func SerializeDexieVersionsHeader(versions map[string]int64) string {
	if len(versions) == 0 {
		return ""
	}
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+strconv.FormatInt(versions[k], 10))
	}
	return strings.Join(parts, ",")
}

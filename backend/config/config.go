package config

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/storage"
)

type Config struct {
	Env  string
	Code string
	Host string
	Port string

	DBHost string
	DBPort string
	DBUser string
	DBPass string
	DBName string

	TokenSecret      string
	TokenAccessTTL   time.Duration
	TokenRefreshTTL  time.Duration
	TokenMaxSessions int
	TokenIPBinding   bool
	SeedStrict       bool
	SeedPrune        bool
	TokenIssuer      string
	TokenAudience    string
	TokenCfgVer      string

	// TrustedProxies: CIDR/IP list (phẩy) mà gin tin X-Forwarded-For.
	// Rỗng = không tin proxy nào (ClientIP = IP kết nối trực tiếp).
	TrustedProxies []string
	// DeviceBindingMode: off | log | enforce — so device_fingerprint khi refresh token.
	DeviceBindingMode string
	// IPAllowlistMode: off | log | enforce — chặn nhân viên theo allowed_ips (company/user).
	IPAllowlistMode string
	// PermEnforce: false = audit-mode (log would-deny, KHÔNG chặn), true = enforce 403.
	// Đọc 1 lần lúc boot (PERM_ENFORCE env), KHÔNG đọc mỗi request.
	PermEnforce bool

	// CORSAllowedOrigins — whitelist Origin prod (CORS_ALLOWED_ORIGINS, CSV). Rỗng ở prod = fail-closed
	// (KHÔNG origin nào được cấp Allow-Origin); dev/develop không dùng field này
	// (CORS dev phản chiếu mọi Origin, xem middleware.CORS).
	CORSAllowedOrigins []string

	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDBCache  int
	RedisDBAuth   int

	// Storage — chọn driver bằng STORAGE_DRIVER=local|s3|r2 (pkg/storage.New). Trống ⇒ suy ra s3 nếu
	// có S3_KEY (tương thích cấu hình cũ), không thì local. Sai/thiếu cấu hình ⇒ DỪNG khởi động.
	StorageDriver       string
	StoragePath         string // kho public local (phục vụ static tại StoragePublicPrefix)
	StoragePrivatePath  string // kho private local — KHÔNG BAO GIỜ mount static
	StoragePublicPrefix string
	StoragePublicBase   string // gốc URL tuyệt đối cho ảnh local; rỗng ⇒ URL tương đối
	// StorageSigningKey — STORAGE_SIGNING_KEY: khoá HMAC ký URL ngắn hạn file đính kèm (driver local).
	// Bắt buộc khi driver local ở prod (pkg/storage.New kiểm, dừng khởi động nếu thiếu).
	StorageSigningKey string
	// StorageSignedURLTTL — STORAGE_SIGNED_URL_TTL: hạn URL ký (mặc định 5 phút, trần 15 phút).
	StorageSignedURLTTL time.Duration

	S3Key           string
	S3Secret        string
	S3Region        string
	S3Bucket        string
	S3PrivateBucket string
	S3Endpoint      string
	S3CDN           string
	S3Folder        string
	S3PublicACL     string

	TLSCert string
	TLSKey  string

	DownloadURL string
	// ExportFileTTL — file export (storage/export) cũ hơn mốc này bị dọn lúc boot + mỗi giờ
	// (EXPORT_FILE_TTL_HOURS, mặc định 24; giá trị ≤ 0 ⇒ dùng mặc định).
	ExportFileTTL time.Duration

	TGBotToken string
	TGChatID   string
}

func Load() (*Config, error) {
	cfg := &Config{
		// Thiếu APP_ENV ⇒ prod (fail-closed): quên set biến ở server thật không được âm thầm chạy chế
		// độ dev (CORS phản chiếu mọi Origin, bỏ check secret). Dev phải ghi APP_ENV=dev tường minh.
		Env:                 env("APP_ENV", "prod"),
		Code:                env("APP_CODE", "cluvix"),
		Host:                env("SERVER_HOST", "127.0.0.1"),
		Port:                env("SERVER_PORT", "8080"),
		DBHost:              env("DB_HOST", "localhost"),
		DBPort:              env("DB_PORT", "3306"),
		DBUser:              env("DB_USER", "root"),
		DBPass:              env("DB_PASS", ""),
		DBName:              env("DB_NAME", "cluvix"),
		TokenSecret:         env("TOKEN_SECRET", ""),
		TokenAccessTTL:      time.Duration(envInt("TOKEN_ACCESSTTL", 900)) * time.Second,
		TokenRefreshTTL:     time.Duration(envInt("TOKEN_REFRESHTTL", 86400)) * time.Second,
		TokenMaxSessions:    envInt("TOKEN_MAX_SESSIONS", 3),
		TokenIPBinding:      env("TOKEN_IP_BINDING", "false") == "true",
		SeedStrict:          env("SEED_STRICT", "false") == "true",
		SeedPrune:           env("SEED_PRUNE", "false") == "true",
		TokenCfgVer:         env("TOKEN_CFGVER", "v1"),
		TrustedProxies:      envList("TRUSTED_PROXIES"),
		DeviceBindingMode:   env("DEVICE_BINDING_MODE", "off"),
		IPAllowlistMode:     env("IP_ALLOWLIST_MODE", "off"),
		PermEnforce:         env("PERM_ENFORCE", "true") != "false",
		CORSAllowedOrigins:  envList("CORS_ALLOWED_ORIGINS"),
		RedisHost:           env("REDIS_HOST", "127.0.0.1"),
		RedisPort:           env("REDIS_PORT", "6379"),
		RedisPassword:       env("REDIS_PASSWORD", ""),
		RedisDBCache:        envInt("REDIS_DB_CACHE", 0),
		RedisDBAuth:         envInt("REDIS_DB_AUTH", 1),
		StorageDriver:       env("STORAGE_DRIVER", ""),
		StoragePath:         env("STORAGE_PATH", "./storage/uploads"),
		StoragePrivatePath:  env("STORAGE_PRIVATE_PATH", "./storage/private"),
		StoragePublicPrefix: env("STORAGE_PUBLIC_PREFIX", "/uploads"),
		StoragePublicBase:   env("STORAGE_PUBLIC_BASE", ""),
		StorageSigningKey:   env("STORAGE_SIGNING_KEY", ""),
		S3Key:               env("S3_KEY", ""),
		S3Secret:            env("S3_SECRET", ""),
		S3Region:            env("S3_REGION", "hcm"),
		S3Bucket:            env("S3_BUCKET", ""),
		S3PrivateBucket:     env("S3_PRIVATE_BUCKET", ""),
		S3Endpoint:          env("S3_ENDPOINT", ""),
		S3CDN:               env("S3_CDN", ""),
		S3Folder:            env("S3_FOLDER", ""),
		S3PublicACL:         env("S3_PUBLIC_ACL", ""),
		TLSCert:             env("TLS_CERT", ""),
		TLSKey:              env("TLS_KEY", ""),
		DownloadURL:         env("DOWNLOAD_URL", ""),
		ExportFileTTL:       time.Duration(envInt("EXPORT_FILE_TTL_HOURS", defaultExportFileTTLHours)) * time.Hour,
		TGBotToken:          env("TG_BOT_TOKEN", ""),
		TGChatID:            env("TG_CHAT_ID", ""),
	}

	if cfg.TokenSecret == "" {
		return nil, fmt.Errorf("TOKEN_SECRET is required")
	}
	// Ở prod TOKEN_SECRET phải đủ dài và không phải giá trị mẫu: secret yếu/công khai ⇒ ai cũng ký
	// được JWT hợp lệ (giả mạo mọi user, kể cả super). Dev giữ nới lỏng để chạy nhanh.
	if cfg.IsProduction() {
		if err := validateTokenSecret(cfg.TokenSecret); err != nil {
			return nil, err
		}
	}
	if cfg.ExportFileTTL <= 0 {
		cfg.ExportFileTTL = defaultExportFileTTLHours * time.Hour
	}

	ttl, clamped, err := storage.ParseSignedURLTTL(os.Getenv("STORAGE_SIGNED_URL_TTL"))
	if err != nil {
		return nil, err
	}
	if clamped {
		log.Printf("config: STORAGE_SIGNED_URL_TTL vượt trần ⇒ dùng %s", storage.MaxSignedURLTTL)
	}
	cfg.StorageSignedURLTTL = ttl

	// O-7: ở prod (gồm mọi APP_ENV không phải dev tường minh — xem IsProduction) secret 2FA PHẢI được
	// mã hoá. Thiếu TOTP_ENC_KEY thì pkg/crypto lưu secret PLAINTEXT và chỉ log WARN; set key về sau
	// thì secret cũ giải mã lỗi. Fail-closed lúc boot thay vì phát hiện khi đã có dữ liệu.
	if cfg.IsProduction() {
		if err := validateTOTPKey(os.Getenv("TOTP_ENC_KEY")); err != nil {
			return nil, err
		}
	}

	// JWT_LIVECHAT_KEY / JWT_PARTNER_KEY / EMR_CONFIG_ENCRYPTION_KEY từng bắt buộc ở production
	// (fail-closed lúc boot). Module dùng chúng (livechat, partner/client, ký số/e-invoice/Zalo OA)
	// đã gỡ khỏi CRM standalone — không còn code sản phẩm nào đọc ⇒ bỏ check (retro F10, user chốt
	// 2026-10-17). Thêm lại tính năng cần secret riêng ⇒ thêm check fail-closed mới tại đây.

	// Issuer/audience JWT: "{APP_CODE}.{ENV}.{DB_NAME}.issuer|aud". DB_NAME là định danh khách
	// (DB-per-customer) — JWTAuth/DecodeAndValidate kiểm 2 claim này ⇒ access token của khách A KHÔNG
	// dùng được ở khách B kể cả khi 2 khách lỡ chung TOKEN_SECRET (R-1, 2026-10-18). Cùng lý do với
	// redisNamespace.
	cfg.TokenIssuer = fmt.Sprintf("%s.%s.%s.issuer", cfg.Code, cfg.Env, cfg.DBName)
	cfg.TokenAudience = fmt.Sprintf("%s.%s.%s.aud", cfg.Code, cfg.Env, cfg.DBName)

	return cfg, nil
}

func (c *Config) DSN() string {
	// collation (không phải charset): khai tường minh để connection không phụ thuộc default của
	// server/DB. `charset=utf8mb4` chỉ phát `SET NAMES utf8mb4` -> lấy collation mặc định của charset
	// (MySQL 9.7: utf8mb4_0900_ai_ci), lệch chuẩn dự án utf8mb4_unicode_ci.
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?collation=utf8mb4_unicode_ci&parseTime=True&loc=Local",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName)
}

func (c *Config) ListenAddr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// devEnvs — các giá trị APP_ENV được coi là môi trường phát triển. CHỈ các giá trị này mới bật
// hành vi nới lỏng (CORS phản chiếu mọi Origin, gin debug, bỏ check secret prod).
var devEnvs = map[string]bool{"dev": true, "develop": true, "local": true}

const defaultExportFileTTLHours = 24

// IsDevelopment — APP_ENV là dev/develop/local TƯỜNG MINH.
func (c *Config) IsDevelopment() bool {
	return devEnvs[c.Env]
}

// IsProduction — fail-closed (S-12/O-8, 2026-10-18): MỌI APP_ENV không phải dev tường minh đều chạy
// chế độ prod (vd "production", "staging", "PROD", gõ nhầm). Trước đây chỉ đúng chuỗi "prod" mới
// là prod ⇒ staging/production chạy với CORS phản chiếu mọi Origin kèm Allow-Credentials.
func (c *Config) IsProduction() bool {
	return !c.IsDevelopment()
}

// minTokenSecretLen — độ dài tối thiểu TOKEN_SECRET ở prod (khuyến nghị `openssl rand -hex 32`).
const minTokenSecretLen = 32

// knownTokenSecrets — giá trị mẫu nằm trong repo public (.env.docker.example, docker-compose.yml):
// ai cũng biết ⇒ cấm ở prod.
var knownTokenSecrets = map[string]bool{"dev-only-token-secret-change-me": true}

// validateTokenSecret — TOKEN_SECRET ở prod: không phải giá trị mẫu và ≥ minTokenSecretLen ký tự.
func validateTokenSecret(secret string) error {
	if knownTokenSecrets[secret] {
		return fmt.Errorf("TOKEN_SECRET is a public sample value — set your own (openssl rand -hex 32)")
	}
	if len(secret) < minTokenSecretLen {
		return fmt.Errorf("TOKEN_SECRET too short for production (%d chars, need >= %d; openssl rand -hex 32)", len(secret), minTokenSecretLen)
	}
	return nil
}

// validateTOTPKey —TOTP_ENC_KEY phải đúng 64 ký tự hex (32 byte AES-256).
func validateTOTPKey(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("TOTP_ENC_KEY is required in production (64 hex chars, got %d chars)", len(key))
	}
	if _, err := hex.DecodeString(key); err != nil {
		return fmt.Errorf("TOTP_ENC_KEY must be 64 hex chars: %v", err)
	}
	return nil
}

// redisNamespace — `APP_CODE:APP_ENV:DB_NAME`. DB_NAME tách khoá Redis giữa các khách SaaS dùng chung
// 1 Redis (DB-per-customer ⇒ DB_NAME là định danh khách duy nhất trong config). Thiếu nó, refresh
// token opaque của khách A (tra `rtidx:<rid>` → uid) dùng được ở khách B (S-3, 2026-10-18).
func (c *Config) redisNamespace() string {
	return fmt.Sprintf("%s:%s:%s", c.Code, c.Env, c.DBName)
}

func (c *Config) CachePrefix() string {
	return c.redisNamespace() + ":app:"
}

func (c *Config) AuthPrefix() string {
	return c.redisNamespace() + ":auth:"
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envList parse env dạng "a,b,c" → []string, trim space, bỏ phần tử rỗng.
func envList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

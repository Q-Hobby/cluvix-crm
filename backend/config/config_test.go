package config

import (
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

var validTOTPKey = strings.Repeat("ab", 32)

// TestLoad_ProdBootGuards — retro F10 (2026-10-17): production chỉ còn bắt buộc TOKEN_SECRET.
// JWT_LIVECHAT_KEY / JWT_PARTNER_KEY / EMR_CONFIG_ENCRYPTION_KEY thuộc module đã gỡ — prod boot
// được khi KHÔNG set chúng (trước đây boot fail). Case "thiếu TOKEN_SECRET" giữ để chứng minh
// guard fail-closed còn lại vẫn chạy.
func TestLoad_ProdBootGuards(t *testing.T) {
	cases := []struct {
		name        string
		env         string
		tokenSecret string
		totpKey     string
		wantErr     bool
	}{
		{name: "prod thiếu TOKEN_SECRET → error", env: "prod", tokenSecret: "", totpKey: validTOTPKey, wantErr: true},
		{name: "dev thiếu TOKEN_SECRET → error", env: "dev", tokenSecret: "", wantErr: true},
		{name: "prod có TOKEN_SECRET + TOTP_ENC_KEY (không key di sản) → ok", env: "prod", tokenSecret: "s", totpKey: validTOTPKey, wantErr: false},
		{name: "dev chỉ có TOKEN_SECRET → ok", env: "dev", tokenSecret: "s", wantErr: false},
		// O-7: TOTP_ENC_KEY bắt buộc ở prod — thiếu/sai định dạng ⇒ từ chối boot.
		{name: "prod thiếu TOTP_ENC_KEY → error", env: "prod", tokenSecret: "s", totpKey: "", wantErr: true},
		{name: "prod TOTP_ENC_KEY không phải hex → error", env: "prod", tokenSecret: "s", totpKey: strings.Repeat("z", 64), wantErr: true},
		{name: "prod TOTP_ENC_KEY sai độ dài → error", env: "prod", tokenSecret: "s", totpKey: "abcd", wantErr: true},
		// O-8: APP_ENV lạ (staging/production) = prod ⇒ cũng bắt buộc TOTP_ENC_KEY.
		{name: "staging thiếu TOTP_ENC_KEY → error", env: "staging", tokenSecret: "s", totpKey: "", wantErr: true},
		{name: "production thiếu TOTP_ENC_KEY → error", env: "production", tokenSecret: "s", totpKey: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("TOKEN_SECRET", tc.tokenSecret)
			t.Setenv("TOTP_ENC_KEY", tc.totpKey)
			t.Setenv("JWT_LIVECHAT_KEY", "")
			t.Setenv("JWT_PARTNER_KEY", "")
			t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", "")

			_, err := Load()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

// TestIsProduction_FailClosed — S-12/O-8: chỉ dev/develop/local TƯỜNG MINH là dev; mọi giá trị khác
// (production, staging, PROD, gõ nhầm) chạy chế độ prod.
func TestIsProduction_FailClosed(t *testing.T) {
	for env, wantProd := range map[string]bool{
		"dev": false, "develop": false, "local": false,
		"prod": true, "production": true, "staging": true, "PROD": true, "Dev": true, "demo": true,
	} {
		c := &Config{Env: env}
		if got := c.IsProduction(); got != wantProd {
			t.Errorf("APP_ENV=%q: IsProduction()=%v, muốn %v", env, got, wantProd)
		}
		if c.IsDevelopment() == wantProd {
			t.Errorf("APP_ENV=%q: IsDevelopment() phải là phủ định của IsProduction()", env)
		}
	}
}

// TestRedisPrefix_ChuaDBName — S-3: 2 khách cùng APP_CODE/APP_ENV dùng chung Redis phải có namespace
// khác nhau (DB_NAME là định danh khách — DB-per-customer).
func TestRedisPrefix_ChuaDBName(t *testing.T) {
	a := &Config{Code: "CluvixCRM", Env: "prod", DBName: "khach_a"}
	b := &Config{Code: "CluvixCRM", Env: "prod", DBName: "khach_b"}
	if got, want := a.AuthPrefix(), "CluvixCRM:prod:khach_a:auth:"; got != want {
		t.Fatalf("AuthPrefix = %q, muốn %q", got, want)
	}
	if got, want := a.CachePrefix(), "CluvixCRM:prod:khach_a:app:"; got != want {
		t.Fatalf("CachePrefix = %q, muốn %q", got, want)
	}
	if a.AuthPrefix() == b.AuthPrefix() || a.CachePrefix() == b.CachePrefix() {
		t.Fatal("2 khách khác DB_NAME phải có prefix Redis khác nhau")
	}
}

// TestTokenIssuer_ChuaDBName — R-1: issuer/audience JWT chứa DB_NAME ⇒ 2 khách khác DB không nhận
// access token của nhau dù chung TOKEN_SECRET.
func TestTokenIssuer_ChuaDBName(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("APP_CODE", "CluvixCRM")
	t.Setenv("TOKEN_SECRET", "s")
	t.Setenv("DB_NAME", "khach_a")
	a, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DB_NAME", "khach_b")
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := a.TokenIssuer, "CluvixCRM.dev.khach_a.issuer"; got != want {
		t.Fatalf("TokenIssuer = %q, muốn %q", got, want)
	}
	if got, want := a.TokenAudience, "CluvixCRM.dev.khach_a.aud"; got != want {
		t.Fatalf("TokenAudience = %q, muốn %q", got, want)
	}
	if a.TokenIssuer == b.TokenIssuer || a.TokenAudience == b.TokenAudience {
		t.Fatal("2 khách khác DB_NAME phải có issuer/audience khác nhau")
	}
}

// TestLoad_ExportFileTTL — mặc định 24h; env ghi đè; giá trị ≤ 0 quay về mặc định.
func TestLoad_ExportFileTTL(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("TOKEN_SECRET", "s")
	for raw, want := range map[string]time.Duration{"": 24 * time.Hour, "6": 6 * time.Hour, "0": 24 * time.Hour, "-3": 24 * time.Hour, "abc": 24 * time.Hour} {
		t.Setenv("EXPORT_FILE_TTL_HOURS", raw)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.ExportFileTTL != want {
			t.Errorf("EXPORT_FILE_TTL_HOURS=%q: %v, muốn %v", raw, cfg.ExportFileTTL, want)
		}
	}
}

// TestEnvExample_KhongCoCommentLotVaoGiaTri — O-1: godotenv chỉ coi `#` là comment khi có khoảng
// trắng đứng trước; `APP_CODE=CluvixCRM#Quan trọng` từng cho giá trị "CluvixCRM#Quan trọng" (lọt vào
// JWT issuer, prefix Redis, issuer TOTP). Đọc .env.example ĐÚNG như app đọc và chặn mọi giá trị có `#`.
func TestEnvExample_KhongCoCommentLotVaoGiaTri(t *testing.T) {
	vals, err := godotenv.Read("../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	if vals["APP_CODE"] != "CluvixCRM" { // đối chứng: file thật sự được đọc
		t.Fatalf("APP_CODE = %q, muốn %q", vals["APP_CODE"], "CluvixCRM")
	}
	for k, v := range vals {
		if strings.Contains(v, "#") {
			t.Errorf("%s=%q — comment lọt vào giá trị; đưa comment lên dòng riêng", k, v)
		}
	}
}

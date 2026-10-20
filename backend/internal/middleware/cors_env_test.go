package middleware

import (
	"strings"
	"testing"

	"erpCluvix/config"
)

// TestCORS_AppEnvLaKhongPhanChieuOrigin — S-12/O-8: APP_ENV khác dev tường minh (staging, production,
// PROD…) KHÔNG được phản chiếu Origin lạ kèm Allow-Credentials. Đi đúng đường router dùng:
// config.Load() → cfg.IsProduction() → CORS(...).
func TestCORS_AppEnvLaKhongPhanChieuOrigin(t *testing.T) {
	const evil = "https://evil.example.net"
	for _, env := range []string{"staging", "production", "PROD", "demo", "prod"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("TOKEN_SECRET", strings.Repeat("k", 32))
			t.Setenv("TOTP_ENC_KEY", strings.Repeat("ab", 32))
			t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			w := preflightWithAllowlist(evil, cfg.IsProduction(), cfg.CORSAllowedOrigins)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("APP_ENV=%s phản chiếu origin lạ: Allow-Origin=%q", env, got)
			}
			// Origin trong whitelist vẫn được cấp.
			w = preflightWithAllowlist("https://app.example.com", cfg.IsProduction(), cfg.CORSAllowedOrigins)
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
				t.Fatalf("APP_ENV=%s: origin whitelist không được cấp: %q", env, got)
			}
		})
	}
}

// TestCORS_DevTuongMinhVanPhanChieu — dev/develop/local giữ tiện lợi dev (phản chiếu mọi Origin).
func TestCORS_DevTuongMinhVanPhanChieu(t *testing.T) {
	const origin = "http://localhost:4211"
	for _, env := range []string{"dev", "develop", "local"} {
		t.Setenv("APP_ENV", env)
		t.Setenv("TOKEN_SECRET", strings.Repeat("k", 32))
		cfg, err := config.Load()
		if err != nil {
			t.Fatal(err)
		}
		w := preflightWithAllowlist(origin, cfg.IsProduction(), cfg.CORSAllowedOrigins)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("APP_ENV=%s: muốn phản chiếu %q, nhận %q", env, origin, got)
		}
	}
}

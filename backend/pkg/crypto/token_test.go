package crypto

import (
	"strings"
	"testing"

	"erpCluvix/pkg/define"
)

// key test: hex 64 chars = 32 bytes AES-256
const testKeyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestEncryptTokenRoundtrip(t *testing.T) {
	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", testKeyHex)

	plain := "DS976-7-secret-refresh-token-value"
	enc, err := EncryptToken(plain)
	if err != nil {
		t.Fatalf("EncryptToken: %v", err)
	}
	if !strings.HasPrefix(enc, define.EncMarkerV1) {
		t.Fatalf("encrypted value missing marker %q", define.EncMarkerV1)
	}
	if enc == plain || strings.Contains(enc, plain) {
		t.Fatal("encrypted value must not contain plaintext")
	}

	got, err := DecryptTokenAware(enc)
	if err != nil {
		t.Fatalf("DecryptTokenAware: %v", err)
	}
	if got != plain {
		t.Fatalf("roundtrip mismatch: got %d chars, want %d chars", len(got), len(plain))
	}
}

// AC2 — nhánh không marker: trả nguyên văn (plaintext legacy transition).
func TestDecryptTokenAwarePlaintextPassthrough(t *testing.T) {
	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", testKeyHex)

	plain := "legacy-plaintext-token"
	got, err := DecryptTokenAware(plain)
	if err != nil {
		t.Fatalf("DecryptTokenAware plaintext: %v", err)
	}
	if got != plain {
		t.Fatal("plaintext passthrough must return input unchanged")
	}
}

// Double-encrypt guard — input đã có marker → trả nguyên văn (backfill idempotent).
func TestEncryptTokenDoubleEncryptGuard(t *testing.T) {
	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", testKeyHex)

	enc, err := EncryptToken("some-token")
	if err != nil {
		t.Fatalf("EncryptToken: %v", err)
	}
	again, err := EncryptToken(enc)
	if err != nil {
		t.Fatalf("EncryptToken second pass: %v", err)
	}
	if again != enc {
		t.Fatal("re-encrypting an already-marked value must return it unchanged")
	}
}

// Chuỗi rỗng giữ nguyên rỗng (Revoke set '' — không được biến thành ciphertext).
func TestEncryptTokenEmpty(t *testing.T) {
	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", testKeyHex)

	enc, err := EncryptToken("")
	if err != nil {
		t.Fatalf("EncryptToken empty: %v", err)
	}
	if enc != "" {
		t.Fatal("empty input must stay empty")
	}
	got, err := DecryptTokenAware("")
	if err != nil || got != "" {
		t.Fatalf("DecryptTokenAware empty: got %q err %v", got, err)
	}
}

// Dev-mode (key chưa set): encrypt trả plaintext KHÔNG marker.
func TestEncryptTokenNoKeyDevMode(t *testing.T) {
	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", "")

	plain := "dev-mode-token"
	enc, err := EncryptToken(plain)
	if err != nil {
		t.Fatalf("EncryptToken dev-mode: %v", err)
	}
	if enc != plain {
		t.Fatal("dev-mode (no key) must return plaintext without marker")
	}
}

// Có marker nhưng key thiếu → ErrEncKeyMissing, KHÔNG trả ciphertext làm token.
func TestDecryptTokenAwareMarkedButNoKey(t *testing.T) {
	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", testKeyHex)
	enc, err := EncryptToken("token-x")
	if err != nil {
		t.Fatalf("EncryptToken: %v", err)
	}

	t.Setenv("EMR_CONFIG_ENCRYPTION_KEY", "")
	if _, err := DecryptTokenAware(enc); err == nil {
		t.Fatal("expected ErrEncKeyMissing when value has marker but key env is empty")
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("plain") {
		t.Fatal("plain value must not be detected as encrypted")
	}
	if !IsEncrypted(define.EncMarkerV1 + "abcd") {
		t.Fatal("marked value must be detected as encrypted")
	}
}

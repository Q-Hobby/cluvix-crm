// Package crypto — AES-GCM encrypt/decrypt dùng cho sensitive config (api_key, credentials).
// Key đọc từ env EMR_CONFIG_ENCRYPTION_KEY (hex 64 chars = 32 bytes) hoặc fallback
// hash SHA-256 của bất kỳ chuỗi nào. Nếu key trống → trả plaintext (dev-mode, log warn).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
)

// EncryptAESGCM mã hoá plaintext bằng AES-256-GCM, trả hex string (nonce+ciphertext).
func EncryptAESGCM(plaintext string) (string, error) {
	return EncryptAESGCMKey(plaintext, "EMR_CONFIG_ENCRYPTION_KEY")
}

// DecryptAESGCM giải mã bằng key mặc định (EMR_CONFIG_ENCRYPTION_KEY).
func DecryptAESGCM(cipherhex string) (string, error) {
	return DecryptAESGCMKey(cipherhex, "EMR_CONFIG_ENCRYPTION_KEY")
}

// EncryptAESGCMKey — như EncryptAESGCM nhưng key đọc từ env `keyEnv` bất kỳ
// (vd TOTP_ENC_KEY cho 2FA — tách riêng để đổi 1 key không ảnh hưởng cái kia).
func EncryptAESGCMKey(plaintext, keyEnv string) (string, error) {
	key := resolveKeyFrom(keyEnv)
	if key == nil {
		log.Printf("[WARN] EncryptAESGCMKey: %s chưa set — lưu plaintext (dev-mode)", keyEnv)
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// DecryptAESGCM giải mã hex string (nonce+ciphertext) → plaintext.
// DecryptAESGCMKey — như DecryptAESGCM nhưng key từ env `keyEnv` bất kỳ.
func DecryptAESGCMKey(cipherhex, keyEnv string) (string, error) {
	key := resolveKeyFrom(keyEnv)
	if key == nil {
		// Dev-mode: không có key → coi đây là plaintext
		return cipherhex, nil
	}
	data, err := hex.DecodeString(cipherhex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// resolveKeyFrom trả 32-byte AES key từ env `name` (hex 64 chars) hoặc SHA-256
// hash của env value. Trả nil nếu env trống (dev-mode).
func resolveKeyFrom(name string) []byte {
	raw := os.Getenv(name)
	if raw == "" {
		return nil
	}
	// Nếu đúng 64 hex chars = 32 bytes → dùng trực tiếp
	if len(raw) == 64 {
		if b, err := hex.DecodeString(raw); err == nil {
			return b
		}
	}
	// Fallback: SHA-256 của raw string → 32 bytes
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

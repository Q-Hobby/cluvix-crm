// Token encryption enc-aware — dùng cho secret/token lưu DB (zalo_oa_config
// giai đoạn chuyển tiếp plaintext → encrypted). Giá trị đã mã hóa mang prefix
// define.EncMarkerV1 ("enc:v1:"); giá trị không prefix = plaintext legacy.
//
// Dùng CHUNG cặp AES-256-GCM sẵn có trong package này (EncryptAESGCM /
// DecryptAESGCM — key env EMR_CONFIG_ENCRYPTION_KEY) — KHÔNG tạo helper crypto
// thứ 2 (AC1 story omnichannel-config/story-02).
package crypto

import (
	"errors"
	"strings"

	"erpCluvix/pkg/define"
)

// tokenKeyEnv — env var chứa AES key, dùng chung với digital_signature/einvoice
// (EncryptAESGCM mặc định). Đổi key = re-encrypt toàn bộ giá trị đang lưu.
const tokenKeyEnv = "EMR_CONFIG_ENCRYPTION_KEY"

// ErrEncKeyMissing — giá trị có marker enc:v1: nhưng EMR_CONFIG_ENCRYPTION_KEY
// chưa set → không thể decrypt. Đây là lỗi cấu hình, KHÔNG được âm thầm trả
// ciphertext làm token (call site sẽ gửi rác sang Zalo).
var ErrEncKeyMissing = errors.New("encrypted value found but EMR_CONFIG_ENCRYPTION_KEY is not set")

// EncryptToken mã hóa plain bằng AES-256-GCM và gắn marker define.EncMarkerV1.
//   - plain == ""            → trả "" (Revoke lưu chuỗi rỗng, giữ nguyên semantics).
//   - plain đã có marker     → trả nguyên văn (double-encrypt guard — backfill
//     chạy lại idempotent, không bọc enc 2 lớp).
//   - key env chưa set (dev) → trả plain KHÔNG marker (marker chỉ gắn khi thật
//     sự encrypt; gắn marker lên plaintext sẽ vỡ khi key được set sau này).
//
// KHÔNG log plain/ciphertext trong bất kỳ nhánh nào.
func EncryptToken(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if strings.HasPrefix(plain, define.EncMarkerV1) {
		return plain, nil
	}
	if resolveKeyFrom(tokenKeyEnv) == nil {
		// Dev-mode: EncryptAESGCM sẽ trả plaintext — không gắn marker giả.
		return plain, nil
	}
	enc, err := EncryptAESGCM(plain)
	if err != nil {
		return "", err
	}
	return define.EncMarkerV1 + enc, nil
}

// DecryptTokenAware giải mã enc-aware:
//   - có marker define.EncMarkerV1 → strip marker + decrypt (key thiếu → ErrEncKeyMissing).
//   - không marker → trả nguyên văn (plaintext legacy, giai đoạn chuyển tiếp).
func DecryptTokenAware(v string) (string, error) {
	if !strings.HasPrefix(v, define.EncMarkerV1) {
		return v, nil
	}
	if resolveKeyFrom(tokenKeyEnv) == nil {
		return "", ErrEncKeyMissing
	}
	return DecryptAESGCM(strings.TrimPrefix(v, define.EncMarkerV1))
}

// IsEncrypted — true nếu giá trị đã mang marker enc:v1: (backfill dùng để skip).
func IsEncrypted(v string) bool {
	return strings.HasPrefix(v, define.EncMarkerV1)
}

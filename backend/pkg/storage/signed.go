package storage

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"strconv"
	"strings"
	"time"
)

// ── URL ký ngắn hạn (signed URL) cho kho PRIVATE ─────────────────────────────
//
// Vì sao: <video>/<audio>/<img>/<a href> KHÔNG mang header Authorization ⇒ trước đây FE phải tải trọn
// blob qua XHR (có JWT) rồi mới phát — không tua được, video lớn ngốn RAM. URL ký ngắn hạn cho trình
// duyệt đọc THẲNG (hỗ trợ HTTP Range ⇒ tua, phát dần) mà không lộ JWT vào URL:
//   - s3/r2: presigned GET của chính object storage (S3.SignedURL).
//   - local: URL tới GET /api/files/signed/:token, token = HMAC-SHA256 (LocalSigned).
//
// Quyền được kiểm TRƯỚC khi ký (handler có JWT + perm + scope); URL ký chỉ là "vé" có hạn ngắn cho
// đúng 1 object với đúng Content-Type/Content-Disposition đã chốt lúc ký.

const (
	// DefaultSignedURLTTL — hạn mặc định (STORAGE_SIGNED_URL_TTL trống).
	DefaultSignedURLTTL = 5 * time.Minute
	// MaxSignedURLTTL — trần cứng: URL lọt ra ngoài (log proxy, lịch sử trình duyệt, copy-paste)
	// chỉ dùng được tối đa chừng này. Cấu hình lớn hơn ⇒ bị kẹp về trần.
	MaxSignedURLTTL = 15 * time.Minute

	// LocalSignedPath — đường công khai (KHÔNG qua JWT) phục vụ URL ký của driver local, tính từ gốc
	// site (router mount dưới /api). Đổi ở đây phải đổi cả router + route_perm_map.json.
	LocalSignedPath = "/api/files/signed/"

	// DispositionInline / DispositionAttachment — giá trị hợp lệ của SignOptions.Disposition.
	DispositionInline     = "inline"
	DispositionAttachment = "attachment"
)

var (
	// ErrSignatureInvalid — token sai định dạng / sai chữ ký / bị sửa payload.
	ErrSignatureInvalid = errors.New("storage: signature invalid")
	// ErrSignatureExpired — chữ ký đúng nhưng đã quá hạn.
	ErrSignatureExpired = errors.New("storage: signature expired")
	// ErrSigningUnsupported — kho không hỗ trợ URL ký.
	ErrSigningUnsupported = errors.New("storage: signed url unsupported")
)

// SignOptions — header phản hồi chốt lúc ký. Người cầm URL KHÔNG đổi được (nằm trong phần đã ký).
type SignOptions struct {
	// ContentType — MIME trả về. Caller PHẢI đưa MIME đã qua whitelist (ngoài whitelist ⇒
	// application/octet-stream), không lấy nguyên giá trị client khai.
	ContentType string
	// Disposition — DispositionInline | DispositionAttachment (giá trị khác ⇒ attachment).
	Disposition string
	// FileName — tên gợi ý khi tải, caller đã làm sạch (bỏ ký tự điều khiển/đường dẫn).
	FileName string
}

// ContentDisposition dựng header Content-Disposition từ opts (filename theo RFC 2231/5987 khi có
// ký tự ngoài ASCII — mime.FormatMediaType tự lo).
func (o SignOptions) ContentDisposition() string {
	d := DispositionAttachment
	if o.Disposition == DispositionInline {
		d = DispositionInline
	}
	if o.FileName == "" {
		return d
	}
	if cd := mime.FormatMediaType(d, map[string]string{"filename": o.FileName}); cd != "" {
		return cd
	}
	return d
}

// Signer — interface TUỲ CHỌN, tách khỏi Store: chỉ driver nào ký được mới implement (caller kiểm
// bằng type assertion `st.(storage.Signer)`). Không bắt mọi Store (fake trong test, kho public…)
// phải có method này.
type Signer interface {
	// SignedURL trả URL đọc trực tiếp key trong ttl (đã kẹp vào (0, MaxSignedURLTTL]).
	// URL của driver local là đường dẫn tuyệt đối trên site (/api/files/signed/…) khi
	// STORAGE_PUBLIC_BASE trống ⇒ FE tự ghép origin của API.
	SignedURL(ctx context.Context, key string, ttl time.Duration, opts SignOptions) (string, error)
}

// ClampSignedURLTTL đưa ttl về (0, MaxSignedURLTTL]; ≤0 ⇒ DefaultSignedURLTTL.
func ClampSignedURLTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultSignedURLTTL
	}
	if ttl > MaxSignedURLTTL {
		return MaxSignedURLTTL
	}
	return ttl
}

// ParseSignedURLTTL đọc STORAGE_SIGNED_URL_TTL: rỗng ⇒ mặc định; nhận duration Go ("5m", "90s")
// hoặc số giây trần ("300"). Sai định dạng / ≤ 0 ⇒ lỗi (dừng khởi động, không âm thầm dùng mặc định).
// Lớn hơn trần ⇒ kẹp về MaxSignedURLTTL, clamped=true để caller log cảnh báo.
func ParseSignedURLTTL(v string) (ttl time.Duration, clamped bool, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return DefaultSignedURLTTL, false, nil
	}
	d, perr := time.ParseDuration(v)
	if perr != nil {
		n, aerr := strconv.Atoi(v)
		if aerr != nil {
			return 0, false, fmt.Errorf("STORAGE_SIGNED_URL_TTL=%q không hợp lệ — dùng số giây (300) hoặc duration (5m)", v)
		}
		d = time.Duration(n) * time.Second
	}
	if d <= 0 {
		return 0, false, fmt.Errorf("STORAGE_SIGNED_URL_TTL=%q phải > 0", v)
	}
	if d > MaxSignedURLTTL {
		return MaxSignedURLTTL, true, nil
	}
	return d, false, nil
}

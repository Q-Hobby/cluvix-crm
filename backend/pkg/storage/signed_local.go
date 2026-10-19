package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"
)

// LocalSigned — kho PRIVATE driver local có khả năng ký URL ngắn hạn (implement Signer).
//
// Bọc *Local (embed ⇒ Save/Open/Delete/URL/Public giữ nguyên hành vi) + khoá HMAC. URL ký trỏ tới
// GET /api/files/signed/:token — route công khai, xác thực bằng CHỮ KÝ thay cho JWT.
//
// Token = base64url(payload JSON) + "." + base64url(HMAC-SHA256(khoá, nhãn-phiên-bản + payload)).
// Payload chứa key + hạn + Content-Type + Content-Disposition: sửa bất kỳ trường nào (đổi key sang
// file khác, kéo dài hạn, đổi sang text/html để render) ⇒ chữ ký sai ⇒ 403. Key chỉ được resolve
// trong thư mục gốc của CHÍNH kho private này (Local.resolve chặn traversal) — token ký cho key ngoài
// kho vẫn không đọc được gì ngoài kho.
type LocalSigned struct {
	*Local
	key     []byte
	urlBase string // STORAGE_PUBLIC_BASE (gốc API) không có "/" cuối; rỗng ⇒ URL tương đối gốc site
	now     func() time.Time
}

var (
	_ Store  = (*LocalSigned)(nil)
	_ Signer = (*LocalSigned)(nil)
)

// signedTokenLabel — nhãn tách miền khoá: cùng khoá dùng cho mục đích khác về sau cũng không tạo
// được token hợp lệ ở đây (và ngược lại). Đổi định dạng payload ⇒ tăng v.
const signedTokenLabel = "cluvix.storage.local-signed.v1."

// DevSigningKey — khoá mặc định CHỈ cho môi trường dev (STORAGE_SIGNING_KEY trống). Prod thiếu khoá
// ⇒ storage.New trả lỗi, server dừng khởi động. Công khai trong mã nguồn ⇒ ai cũng ký được URL.
const DevSigningKey = "cluvix-dev-only-storage-signing-key-DO-NOT-USE-IN-PROD"

// MinSigningKeyLen — độ dài tối thiểu STORAGE_SIGNING_KEY ở prod (byte).
const MinSigningKeyLen = 32

// NewLocalSigned bọc kho local private với khoá ký. key rỗng ⇒ lỗi (caller quyết định mặc định dev).
func NewLocalSigned(l *Local, key []byte, urlBase string) (*LocalSigned, error) {
	if l == nil {
		return nil, errors.New("storage: NewLocalSigned thiếu kho local")
	}
	if len(key) == 0 {
		return nil, errors.New("storage: NewLocalSigned thiếu khoá ký")
	}
	return &LocalSigned{
		Local:   l,
		key:     append([]byte(nil), key...),
		urlBase: strings.TrimRight(strings.TrimSpace(urlBase), "/"),
		now:     time.Now,
	}, nil
}

// SignedObject — nội dung đã xác thực của 1 token.
type SignedObject struct {
	Key                string `json:"k"`
	Expires            int64  `json:"e"`
	ContentType        string `json:"t"`
	ContentDisposition string `json:"d"`
}

// SignedURL — implement Signer. Key được kiểm SafeKey ngay lúc ký (lỗi lập trình lộ ra sớm).
func (s *LocalSigned) SignedURL(ctx context.Context, key string, ttl time.Duration, opts SignOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := s.resolve(key); err != nil {
		return "", err
	}
	ct := strings.TrimSpace(opts.ContentType)
	if ct == "" {
		ct = "application/octet-stream"
	}
	obj := SignedObject{
		Key:                key,
		Expires:            s.now().Add(ClampSignedURLTTL(ttl)).Unix(),
		ContentType:        ct,
		ContentDisposition: opts.ContentDisposition(),
	}
	tok, err := s.sign(obj)
	if err != nil {
		return "", err
	}
	return s.urlBase + LocalSignedPath + tok, nil
}

func (s *LocalSigned) sign(obj SignedObject) (string, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.mac(payload)), nil
}

func (s *LocalSigned) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(signedTokenLabel))
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// Verify kiểm chữ ký (so sánh constant-time) TRƯỚC, rồi mới giải mã payload và kiểm hạn — payload
// chưa xác thực không bao giờ được parse. Sai ⇒ ErrSignatureInvalid; quá hạn ⇒ ErrSignatureExpired.
func (s *LocalSigned) Verify(token string) (SignedObject, error) {
	payload, sigB64, ok := strings.Cut(token, ".")
	if !ok || payload == "" || sigB64 == "" || len(token) > 4096 {
		return SignedObject{}, ErrSignatureInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return SignedObject{}, ErrSignatureInvalid
	}
	if !hmac.Equal(sig, s.mac(payload)) {
		return SignedObject{}, ErrSignatureInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return SignedObject{}, ErrSignatureInvalid
	}
	var obj SignedObject
	if err := json.Unmarshal(raw, &obj); err != nil || obj.Key == "" || obj.Expires == 0 {
		return SignedObject{}, ErrSignatureInvalid
	}
	if s.now().Unix() >= obj.Expires {
		return obj, ErrSignatureExpired
	}
	return obj, nil
}

// OpenFile mở file của key (trong kho này) dưới dạng *os.File — có Seek để phục vụ HTTP Range
// (http.ServeContent). Không tồn tại / là thư mục ⇒ ErrNotFound; key nguy hiểm ⇒ ErrUnsafePath.
func (s *LocalSigned) OpenFile(key string) (*os.File, fs.FileInfo, error) {
	full, err := s.resolve(key)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(full) //nolint:gosec // full đã qua resolve (chặn traversal)
	if os.IsNotExist(err) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if st.IsDir() {
		_ = f.Close()
		return nil, nil, ErrNotFound
	}
	return f, st, nil
}

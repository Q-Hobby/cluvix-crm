package storage

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestLocalSigned(t *testing.T) (*LocalSigned, string) {
	t.Helper()
	dir := t.TempDir()
	l, err := NewLocal(filepath.Join(dir, "private"), "", "", false)
	require.NoError(t, err)
	ls, err := NewLocalSigned(l, []byte("k-0123456789abcdef0123456789abcdef"), "https://api.example.com/")
	require.NoError(t, err)
	return ls, dir
}

func tokenOf(t *testing.T, u string) string {
	t.Helper()
	i := strings.Index(u, LocalSignedPath)
	require.GreaterOrEqual(t, i, 0, u)
	return u[i+len(LocalSignedPath):]
}

func TestLocalSigned_SignVerifyRoundtrip(t *testing.T) {
	ls, _ := newTestLocalSigned(t)
	u, err := ls.SignedURL(context.Background(), "crm/attachments/1/a.mp4", 5*time.Minute,
		SignOptions{ContentType: "video/mp4", Disposition: DispositionInline, FileName: "Hồ sơ.mp4"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u, "https://api.example.com"+LocalSignedPath), u)
	require.NotContains(t, u, "crm/attachments", "key thô không được nằm trần trong URL")

	obj, err := ls.Verify(tokenOf(t, u))
	require.NoError(t, err)
	require.Equal(t, "crm/attachments/1/a.mp4", obj.Key)
	require.Equal(t, "video/mp4", obj.ContentType)
	require.True(t, strings.HasPrefix(obj.ContentDisposition, "inline"), obj.ContentDisposition)
	require.Contains(t, obj.ContentDisposition, "filename*=utf-8''")
	require.InDelta(t, time.Now().Add(5*time.Minute).Unix(), obj.Expires, 2)
}

// Sửa payload (đổi key/hạn/Content-Type) ⇒ chữ ký sai. Khoá khác ⇒ sai. Rác ⇒ sai.
func TestLocalSigned_TamperedRejected(t *testing.T) {
	ls, _ := newTestLocalSigned(t)
	u, err := ls.SignedURL(context.Background(), "a/b.pdf", time.Minute, SignOptions{ContentType: "application/pdf"})
	require.NoError(t, err)
	tok := tokenOf(t, u)
	payload, sig, _ := strings.Cut(tok, ".")

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	require.NoError(t, err)
	forged := strings.Replace(string(raw), `"a/b.pdf"`, `"a/c.pdf"`, 1)
	require.NotEqual(t, string(raw), forged)
	_, err = ls.Verify(base64.RawURLEncoding.EncodeToString([]byte(forged)) + "." + sig)
	require.ErrorIs(t, err, ErrSignatureInvalid)

	other, err := NewLocalSigned(ls.Local, []byte("khoa-khac-0123456789abcdef0123456789"), "")
	require.NoError(t, err)
	_, err = other.Verify(tok)
	require.ErrorIs(t, err, ErrSignatureInvalid)

	for _, bad := range []string{"", ".", payload, payload + ".", "." + sig, payload + ".!!!", "x.y"} {
		_, err = ls.Verify(bad)
		require.ErrorIs(t, err, ErrSignatureInvalid, bad)
	}
	// Đối chứng: token gốc vẫn hợp lệ.
	_, err = ls.Verify(tok)
	require.NoError(t, err)
}

func TestLocalSigned_Expired(t *testing.T) {
	ls, _ := newTestLocalSigned(t)
	now := time.Unix(1_800_000_000, 0)
	ls.now = func() time.Time { return now }
	u, err := ls.SignedURL(context.Background(), "a.pdf", 5*time.Minute, SignOptions{})
	require.NoError(t, err)
	tok := tokenOf(t, u)

	ls.now = func() time.Time { return now.Add(5*time.Minute - time.Second) }
	_, err = ls.Verify(tok)
	require.NoError(t, err, "còn hạn")

	ls.now = func() time.Time { return now.Add(5 * time.Minute) }
	_, err = ls.Verify(tok)
	require.ErrorIs(t, err, ErrSignatureExpired)
}

// TTL vượt trần bị kẹp về 15 phút ngay lúc ký.
func TestLocalSigned_TTLClamped(t *testing.T) {
	ls, _ := newTestLocalSigned(t)
	now := time.Unix(1_800_000_000, 0)
	ls.now = func() time.Time { return now }
	u, err := ls.SignedURL(context.Background(), "a.pdf", 24*time.Hour, SignOptions{})
	require.NoError(t, err)
	obj, err := ls.Verify(tokenOf(t, u))
	require.NoError(t, err)
	require.Equal(t, now.Add(MaxSignedURLTTL).Unix(), obj.Expires)
}

// Key ngoài kho private: không ký được, và kể cả token HỢP LỆ chứa key traversal (giả định khoá lộ /
// lỗi lập trình) cũng không mở được file ngoài thư mục gốc.
func TestLocalSigned_KeyOutsidePrivateNotServed(t *testing.T) {
	ls, dir := newTestLocalSigned(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "uploads"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "uploads", "secret.txt"), []byte("x"), 0o600))

	for _, k := range []string{"../uploads/secret.txt", "/etc/passwd", "", "a/../../uploads/secret.txt"} {
		_, err := ls.SignedURL(context.Background(), k, time.Minute, SignOptions{})
		require.ErrorIs(t, err, ErrUnsafePath, k)
	}

	tok, err := ls.sign(SignedObject{Key: "../uploads/secret.txt", Expires: time.Now().Add(time.Minute).Unix()})
	require.NoError(t, err)
	obj, err := ls.Verify(tok)
	require.NoError(t, err)
	_, _, err = ls.OpenFile(obj.Key)
	require.ErrorIs(t, err, ErrUnsafePath)

	_, _, err = ls.OpenFile("khong-co.pdf")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestParseSignedURLTTL(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		clamped bool
		err     bool
	}{
		{"", DefaultSignedURLTTL, false, false},
		{"300", 5 * time.Minute, false, false},
		{"90s", 90 * time.Second, false, false},
		{"10m", 10 * time.Minute, false, false},
		{"1h", MaxSignedURLTTL, true, false},
		{"0", 0, false, true},
		{"-5m", 0, false, true},
		{"abc", 0, false, true},
	}
	for _, c := range cases {
		got, clamped, err := ParseSignedURLTTL(c.in)
		if c.err {
			require.Error(t, err, c.in)
			continue
		}
		require.NoError(t, err, c.in)
		require.Equal(t, c.want, got, c.in)
		require.Equal(t, c.clamped, clamped, c.in)
	}
	require.Equal(t, DefaultSignedURLTTL, ClampSignedURLTTL(0))
	require.Equal(t, MaxSignedURLTTL, ClampSignedURLTTL(time.Hour))
}

func TestFactory_LocalSigningKey(t *testing.T) {
	dir := t.TempDir()
	base := Config{Driver: DriverLocal, LocalPath: filepath.Join(dir, "u"), LocalPrivatePath: filepath.Join(dir, "p")}

	// Dev: trống ⇒ khoá dev, kho private ký được.
	_, priv, err := New(base)
	require.NoError(t, err)
	_, ok := priv.(Signer)
	require.True(t, ok, "kho private local phải ký được URL")

	// Prod: thiếu / ngắn / dùng khoá dev ⇒ dừng khởi động.
	for _, k := range []string{"", "ngan", DevSigningKey} {
		c := base
		c.Prod, c.SigningKey = true, k
		_, _, err = New(c)
		require.Error(t, err, "prod key=%q", k)
	}
	c := base
	c.Prod, c.SigningKey = true, strings.Repeat("a", MinSigningKeyLen)
	_, _, err = New(c)
	require.NoError(t, err)
}

// Presign S3/R2: tính cục bộ, có X-Amz-Expires đúng TTL + response-content-type/disposition; URL trỏ
// đúng bucket private + folder; endpoint giả nhận được đúng query đã ký khi trình duyệt GET.
func TestS3_SignedURL_Presign(t *testing.T) {
	f, srv := newFakeS3(t)
	cfg := s3FactoryCfg(srv.URL)
	cfg.S3PrivateBucket = "priv"
	_, priv, err := New(cfg)
	require.NoError(t, err)
	signer, ok := priv.(Signer)
	require.True(t, ok)

	raw, err := signer.SignedURL(context.Background(), "crm/attachments/1/a.mp4", 5*time.Minute,
		SignOptions{ContentType: "video/mp4", Disposition: DispositionInline, FileName: "a.mp4"})
	require.NoError(t, err)
	require.Empty(t, f.reqs, "presign KHÔNG được gọi mạng")

	u, err := url.Parse(raw)
	require.NoError(t, err)
	require.Equal(t, "/priv/crm/private/crm/attachments/1/a.mp4", u.Path)
	q := u.Query()
	require.Equal(t, "300", q.Get("X-Amz-Expires"))
	require.Equal(t, `inline; filename=a.mp4`, q.Get("response-content-disposition"))
	require.Equal(t, "video/mp4", q.Get("response-content-type"))
	require.NotEmpty(t, q.Get("X-Amz-Signature"))

	// TTL vượt trần ⇒ 900s; download ⇒ attachment.
	raw, err = signer.SignedURL(context.Background(), "a.pdf", time.Hour, SignOptions{Disposition: DispositionAttachment, FileName: "a.pdf"})
	require.NoError(t, err)
	u, _ = url.Parse(raw)
	require.Equal(t, "900", u.Query().Get("X-Amz-Expires"))
	require.True(t, strings.HasPrefix(u.Query().Get("response-content-disposition"), "attachment"))

	// Trình duyệt GET thẳng URL ký ⇒ request tới endpoint với đúng path.
	f.objects["/priv/crm/private/a.pdf"] = "pdf"
	resp, err := http.Get(raw) //nolint:gosec // URL của httptest
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "pdf", string(body))
	require.Equal(t, "/priv/crm/private/a.pdf", f.last(t).Path)

	_, err = signer.SignedURL(context.Background(), "../x", time.Minute, SignOptions{})
	require.ErrorIs(t, err, ErrUnsafePath)
}

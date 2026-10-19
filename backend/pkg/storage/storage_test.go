package storage

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── factory ────────────────────────────────────────────────────────────────

func TestResolveDriver(t *testing.T) {
	cases := []struct {
		driver, key, want string
		inferred, wantErr bool
	}{
		{"", "", DriverLocal, false, false},
		{"", "AKIA", DriverS3, true, false}, // tương thích cấu hình cũ chỉ khai S3_*
		{" LOCAL ", "AKIA", DriverLocal, false, false},
		{"s3", "", DriverS3, false, false},
		{"R2", "", DriverR2, false, false},
		{"gcs", "", "", false, true},
	}
	for _, c := range cases {
		got, inferred, err := ResolveDriver(c.driver, c.key)
		if c.wantErr {
			require.Error(t, err, c.driver)
			require.Contains(t, err.Error(), "STORAGE_DRIVER")
			continue
		}
		require.NoError(t, err)
		require.Equal(t, c.want, got)
		require.Equal(t, c.inferred, inferred)
	}
}

func TestFactory_InvalidDriverStopsBoot(t *testing.T) {
	pub, priv, err := New(Config{Driver: "ftp"})
	require.Error(t, err)
	require.Nil(t, pub)
	require.Nil(t, priv)
}

func TestFactory_LocalDefault(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := New(Config{
		LocalPath: filepath.Join(dir, "uploads"), LocalPrivatePath: filepath.Join(dir, "private"),
	})
	require.NoError(t, err)
	require.True(t, pub.Public())
	require.False(t, priv.Public())
	require.Equal(t, "/uploads/a/b.png", pub.URL("a/b.png"), "mặc định prefix /uploads, URL tương đối")
	require.Empty(t, priv.URL("a/b.pdf"))
	_, ok := pub.(*Local)
	require.True(t, ok)
}

// Thư mục private trùng/lồng thư mục public ⇒ file riêng tư bị phục vụ static ⇒ DỪNG boot.
func TestFactory_LocalPrivateNestedInPublicIsError(t *testing.T) {
	dir := t.TempDir()
	for _, priv := range []string{filepath.Join(dir, "uploads"), filepath.Join(dir, "uploads", "private")} {
		_, _, err := New(Config{Driver: "local", LocalPath: filepath.Join(dir, "uploads"), LocalPrivatePath: priv})
		require.Error(t, err, priv)
		require.Contains(t, err.Error(), "STORAGE_PRIVATE_PATH")
	}
	// Ngược lại: public nằm trong private cũng chặn.
	_, _, err := New(Config{Driver: "local", LocalPath: filepath.Join(dir, "p", "uploads"), LocalPrivatePath: filepath.Join(dir, "p")})
	require.Error(t, err)
}

func TestFactory_S3MissingConfigListsAllVars(t *testing.T) {
	_, _, err := New(Config{Driver: "s3"})
	require.Error(t, err)
	for _, v := range []string{"S3_KEY", "S3_SECRET", "S3_BUCKET", "S3_ENDPOINT hoặc S3_REGION"} {
		require.Contains(t, err.Error(), v)
	}
	// Từng biến một.
	base := Config{Driver: "s3", S3Key: "k", S3Secret: "s", S3Bucket: "b", S3Endpoint: "https://x"}
	for name, mut := range map[string]func(*Config){
		"S3_KEY":    func(c *Config) { c.S3Key = "" },
		"S3_SECRET": func(c *Config) { c.S3Secret = "" },
		"S3_BUCKET": func(c *Config) { c.S3Bucket = "" },
	} {
		cfg := base
		mut(&cfg)
		_, _, err := New(cfg)
		require.ErrorContains(t, err, name)
	}
	_, _, err = New(base)
	require.NoError(t, err)
}

func TestFactory_S3InvalidACL(t *testing.T) {
	_, _, err := New(Config{Driver: "s3", S3Key: "k", S3Secret: "s", S3Bucket: "b", S3Endpoint: "https://x", S3PublicACL: "world-writable"})
	require.ErrorContains(t, err, "S3_PUBLIC_ACL")
}

func TestFactory_R2RequiresCDNEndpointAndSeparatePrivateBucket(t *testing.T) {
	full := Config{Driver: "r2", S3Key: "k", S3Secret: "s", S3Bucket: "pub",
		S3Endpoint: "https://acc.r2.cloudflarestorage.com", S3CDN: "https://pub-x.r2.dev", S3PrivateBucket: "priv"}
	_, _, err := New(full)
	require.NoError(t, err)

	for name, mut := range map[string]func(*Config){
		"S3_KEY":            func(c *Config) { c.S3Key = "" },
		"S3_SECRET":         func(c *Config) { c.S3Secret = "" },
		"S3_BUCKET":         func(c *Config) { c.S3Bucket = "" },
		"S3_ENDPOINT":       func(c *Config) { c.S3Endpoint = "" },
		"S3_CDN":            func(c *Config) { c.S3CDN = "" },
		"S3_PRIVATE_BUCKET": func(c *Config) { c.S3PrivateBucket = "" },
	} {
		cfg := full
		mut(&cfg)
		_, _, err := New(cfg)
		require.Error(t, err, name)
		require.Contains(t, err.Error(), name)
	}

	same := full
	same.S3PrivateBucket = "pub"
	_, _, err = New(same)
	require.ErrorContains(t, err, "S3_PRIVATE_BUCKET phải KHÁC S3_BUCKET")
}

// ── local ──────────────────────────────────────────────────────────────────

func TestLocal_SaveOpenDelete(t *testing.T) {
	root := t.TempDir()
	l, err := NewLocal(root, "/uploads", "https://api.example.com/", true)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, l.Save(ctx, strings.NewReader("noi dung"), "a/b/c.pdf", "application/pdf"))
	b, err := os.ReadFile(filepath.Join(root, "a", "b", "c.pdf")) //nolint:gosec // đường dẫn trong t.TempDir()
	require.NoError(t, err)
	require.Equal(t, "noi dung", string(b))

	rc, info, err := l.Open(ctx, "a/b/c.pdf")
	require.NoError(t, err)
	got, _ := io.ReadAll(rc)
	require.NoError(t, rc.Close())
	require.Equal(t, "noi dung", string(got))
	require.EqualValues(t, 8, info.Size)
	require.Equal(t, "application/pdf", info.ContentType)

	require.Equal(t, "https://api.example.com/uploads/a/b/c.pdf", l.URL("a/b/c.pdf"))

	require.NoError(t, l.Delete(ctx, "a/b/c.pdf"))
	_, _, err = l.Open(ctx, "a/b/c.pdf")
	require.ErrorIs(t, err, ErrNotFound)
	require.NoError(t, l.Delete(ctx, "a/b/c.pdf"), "xoá file không tồn tại không phải lỗi")

	_, _, err = l.Open(ctx, "a/b")
	require.ErrorIs(t, err, ErrNotFound, "thư mục không phải file")

	// Không để lại file tạm.
	entries, _ := os.ReadDir(filepath.Join(root, "a", "b"))
	require.Empty(t, entries)
}

func TestLocal_PathTraversalRejected(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	l, err := NewLocal(root, "/uploads", "", false)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("bi mat"), 0o600))

	for _, bad := range []string{"", "/etc/passwd", "../secret.txt", "a/../../secret.txt", "..", `..\secret.txt`, "a/\x00b"} {
		require.ErrorIs(t, l.Save(ctx, strings.NewReader("x"), bad, ""), ErrUnsafePath, bad)
		_, _, err := l.Open(ctx, bad)
		require.ErrorIs(t, err, ErrUnsafePath, bad)
		require.ErrorIs(t, l.Delete(ctx, bad), ErrUnsafePath, bad)
	}
	// File ngoài root còn nguyên.
	b, err := os.ReadFile(filepath.Join(parent, "secret.txt")) //nolint:gosec // đường dẫn trong t.TempDir()
	require.NoError(t, err)
	require.Equal(t, "bi mat", string(b))
	// "a/./b" vẫn hợp lệ sau Clean.
	require.NoError(t, l.Save(ctx, strings.NewReader("x"), "a/./b.txt", ""))
}

func TestLocal_PrivateHasNoURL(t *testing.T) {
	l, err := NewLocal(t.TempDir(), "/uploads", "https://api.example.com", false)
	require.NoError(t, err)
	require.Empty(t, l.URL("a.pdf"))
	require.False(t, l.Public())
}

// ── PublicURL / ToKey (tương thích ngược) ─────────────────────────────────

func TestPublicURL_BackwardCompatible(t *testing.T) {
	l, err := NewLocal(t.TempDir(), "/uploads", "https://api.example.com", true)
	require.NoError(t, err)

	require.Equal(t, "", PublicURL(l, ""))
	require.Equal(t, "https://api.example.com/uploads/customer/1/a.webp", PublicURL(l, "customer/1/a.webp"))
	// Dữ liệu cũ: URL tuyệt đối trả NGUYÊN.
	for _, legacy := range []string{"https://cdn.old.com/crm/a.png", "HTTP://x/y.png", "//cdn.x/y.png"} {
		require.Equal(t, legacy, PublicURL(l, legacy))
	}
	require.Equal(t, "/uploads/x.png", PublicURL(l, "/uploads/x.png"), "đường dẫn tuyệt đối trên site trả nguyên")
	require.Equal(t, "a.png", PublicURL(nil, "a.png"), "kho nil ⇒ trả nguyên")
}

func TestToKey(t *testing.T) {
	l, err := NewLocal(t.TempDir(), "/uploads", "https://api.example.com", true)
	require.NoError(t, err)
	require.Equal(t, "customer/1/a.webp", ToKey(l, "https://api.example.com/uploads/customer/1/a.webp"))
	require.Equal(t, "customer/1/a.webp", ToKey(l, "customer/1/a.webp"))
	require.Equal(t, "https://cdn.other.com/a.png", ToKey(l, "https://cdn.other.com/a.png"), "URL ngoài kho giữ nguyên")
	require.Equal(t, "", ToKey(l, ""))
	// round-trip
	require.Equal(t, "k/x.png", ToKey(l, PublicURL(l, "k/x.png")))
}

func TestResolveHelpersUseDefaultStore(t *testing.T) {
	l, err := NewLocal(t.TempDir(), "/uploads", "https://api.example.com", true)
	require.NoError(t, err)
	prev := DefaultPublic()
	SetDefaultPublic(l)
	t.Cleanup(func() { SetDefaultPublic(prev) })

	rows := []map[string]interface{}{
		{"avatar": "a.webp", "other": "b.webp"},
		{"avatar": "https://old/x.png"},
		{"avatar": nil},
		{"avatar": []byte("c.webp")},
		{"customer": map[string]interface{}{"avatar": "n.webp"}},
		{"customer": nil},
	}
	ResolveRows(rows, "avatar")
	ResolveNested(rows, "customer", "avatar")
	require.Equal(t, "https://api.example.com/uploads/a.webp", rows[0]["avatar"])
	require.Equal(t, "b.webp", rows[0]["other"], "chỉ cột được nêu")
	require.Equal(t, "https://old/x.png", rows[1]["avatar"])
	require.Nil(t, rows[2]["avatar"])
	require.Equal(t, "https://api.example.com/uploads/c.webp", rows[3]["avatar"])
	require.Equal(t, "https://api.example.com/uploads/n.webp", rows[4]["customer"].(map[string]interface{})["avatar"])

	tbl := []map[string]interface{}{{"logo_image": "s.webp", "avatar": "z"}}
	ResolveTableImages(tbl, "business_profile")
	require.Equal(t, "https://api.example.com/uploads/s.webp", tbl[0]["logo_image"])
	require.Equal(t, "z", tbl[0]["avatar"])
}

// ── SaveUpload: đuôi theo MIME đã sniff ────────────────────────────────────

func multipartFile(t *testing.T, name string, body []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, _ = fw.Write(body)
	require.NoError(t, w.Close())
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(1<<20))
	return req.MultipartForm.File["file"][0]
}

func TestSaveUpload_ExtensionFromSniffedMIME(t *testing.T) {
	l, err := NewLocal(t.TempDir(), "", "", false)
	require.NoError(t, err)
	pdf := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")

	// PDF giả tên .jpg + tên chứa họ tên khách ⇒ key đuôi .pdf, KHÔNG chứa tên gốc.
	saved, err := SaveUpload(context.Background(), l, multipartFile(t, "Nguyen Van A.jpg", pdf), "crm/attachments/7/", AttachmentMIMETypes)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(saved.Key, "crm/attachments/7/"))
	require.Equal(t, ".pdf", filepath.Ext(saved.Key))
	require.NotContains(t, strings.ToLower(saved.Key), "nguyen")
	require.Equal(t, "application/pdf", saved.MIME)

	rc, _, err := l.Open(context.Background(), saved.Key)
	require.NoError(t, err)
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	require.Equal(t, pdf, got)

	// Ngoài whitelist ⇒ từ chối, không ghi gì.
	_, err = SaveUpload(context.Background(), l, multipartFile(t, "x.pdf", []byte("<html><script>alert(1)</script></html>")), "a/", AttachmentMIMETypes)
	require.Error(t, err)
}

func TestSafeKey(t *testing.T) {
	for _, ok := range []string{"a.png", "a/b/c.png", "a/./b"} {
		require.NoError(t, SafeKey(ok), ok)
	}
	for _, bad := range []string{"", "/a", "../a", "a/../../b", `a\b`, "a\x00"} {
		require.ErrorIs(t, SafeKey(bad), ErrUnsafePath, bad)
	}
}

package storage

// Test driver S3/R2 bằng httptest giả lập endpoint S3 (path-style) — KHÔNG gọi cloud thật,
// không cần credentials thật.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ── deadline upload (thuần tính toán) ──────────────────────────────────────

func TestUploadTimeout_ScalesWithSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want time.Duration
	}{
		// size ≤ 0 = nguồn io.Reader không biết độ dài → dùng trần, KHÔNG deadline ngắn.
		{"không biết size", 0, uploadMaxTimeout},
		{"size âm", -1, uploadMaxTimeout},
		{"file nhỏ 1KB", 1024, uploadBaseTimeout},
		{"10MB", 10 << 20, uploadBaseTimeout + 40*time.Second},
		{"100MB", 100 << 20, uploadBaseTimeout + 400*time.Second},
		{"10GB", 10 << 30, uploadMaxTimeout},
		{"1TB", 1 << 40, uploadMaxTimeout},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, uploadTimeout(tc.size))
		})
	}
}

// Bẫy tràn số: size rất lớn nhân với time.Duration sẽ tràn nếu chặn trần SAU khi đổi kiểu.
func TestUploadTimeout_HugeSizeDoesNotOverflow(t *testing.T) {
	for _, size := range []int64{1 << 50, 1 << 60, 1<<63 - 1} {
		got := uploadTimeout(size)
		require.Positive(t, int64(got), "size=%d tràn số", size)
		require.LessOrEqual(t, got, uploadMaxTimeout)
	}
}

// ── fake S3 ────────────────────────────────────────────────────────────────

type fakeReq struct {
	Method, Path, ACL, ContentType, ContentEncoding string
}

type fakeS3 struct {
	mu      sync.Mutex
	reqs    []fakeReq
	objects map[string]string // path (/bucket/key) → nội dung trả về khi GET
}

func newFakeS3(t *testing.T) (*fakeS3, *httptest.Server) {
	f := &fakeS3{objects: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		f.mu.Lock()
		f.reqs = append(f.reqs, fakeReq{
			Method: r.Method, Path: r.URL.Path,
			ACL: r.Header.Get("x-amz-acl"), ContentType: r.Header.Get("Content-Type"),
			ContentEncoding: r.Header.Get("Content-Encoding"),
		})
		body, ok := f.objects[r.URL.Path]
		f.mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			if !ok {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`)
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = io.WriteString(w, body)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeS3) last(t *testing.T) fakeReq {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.reqs, "fake S3 không nhận request nào")
	return f.reqs[len(f.reqs)-1]
}

func s3FactoryCfg(endpoint string) Config {
	return Config{
		Driver: DriverS3, S3Key: "k", S3Secret: "s", S3Region: "hcm",
		S3Bucket: "pub", S3Endpoint: endpoint, S3CDN: "https://cdn.example.com/", S3Folder: "crm",
	}
}

// Kho public s3: mặc định gửi ACL public-read (tương thích S3 cũ), key có folder prefix,
// Content-Type do caller quyết (MIME đã sniff).
func TestS3_PublicStore_SendsPublicReadACLAndFolder(t *testing.T) {
	f, srv := newFakeS3(t)
	pub, _, err := New(s3FactoryCfg(srv.URL))
	require.NoError(t, err)

	require.NoError(t, pub.Save(context.Background(), strings.NewReader("png"), "branch/a.png", "image/png"))
	got := f.last(t)
	require.Equal(t, http.MethodPut, got.Method)
	require.Equal(t, "/pub/crm/branch/a.png", got.Path)
	require.Equal(t, "public-read", got.ACL)
	require.Equal(t, "image/png", got.ContentType)
	require.Equal(t, "https://cdn.example.com/crm/branch/a.png", pub.URL("branch/a.png"))
	require.True(t, pub.Public())
}

func TestS3_PublicACLNone_SendsNoACL(t *testing.T) {
	f, srv := newFakeS3(t)
	cfg := s3FactoryCfg(srv.URL)
	cfg.S3PublicACL = "none"
	pub, _, err := New(cfg)
	require.NoError(t, err)
	require.NoError(t, pub.Save(context.Background(), strings.NewReader("x"), "a.png", "image/png"))
	require.Empty(t, f.last(t).ACL, "S3_PUBLIC_ACL=none ⇒ không gửi ACL")
}

// Kho private s3: ACL private, folder <folder>/private, bucket = S3_PRIVATE_BUCKET (mặc định S3_BUCKET),
// URL() rỗng.
func TestS3_PrivateStore_PrivateACLAndFolder(t *testing.T) {
	f, srv := newFakeS3(t)
	_, priv, err := New(s3FactoryCfg(srv.URL))
	require.NoError(t, err)

	require.NoError(t, priv.Save(context.Background(), strings.NewReader("pdf"), "crm/attachments/1/x.pdf", "application/pdf"))
	got := f.last(t)
	require.Equal(t, "/pub/crm/private/crm/attachments/1/x.pdf", got.Path)
	require.Equal(t, "private", got.ACL)
	require.Empty(t, priv.URL("crm/attachments/1/x.pdf"), "kho private không có URL công khai")
	require.False(t, priv.Public())

	cfg := s3FactoryCfg(srv.URL)
	cfg.S3PrivateBucket = "secret"
	_, priv2, err := New(cfg)
	require.NoError(t, err)
	require.NoError(t, priv2.Save(context.Background(), strings.NewReader("pdf"), "a.pdf", "application/pdf"))
	require.Equal(t, "/secret/crm/private/a.pdf", f.last(t).Path)
}

func TestS3_OpenAndDelete(t *testing.T) {
	f, srv := newFakeS3(t)
	f.objects["/pub/crm/private/doc.pdf"] = "%PDF-noi-dung"
	_, priv, err := New(s3FactoryCfg(srv.URL))
	require.NoError(t, err)

	rc, info, err := priv.Open(context.Background(), "doc.pdf")
	require.NoError(t, err)
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, "%PDF-noi-dung", string(b))
	require.Equal(t, "application/pdf", info.ContentType)
	require.EqualValues(t, len("%PDF-noi-dung"), info.Size)

	_, _, err = priv.Open(context.Background(), "khong-co.pdf")
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, priv.Delete(context.Background(), "doc.pdf"))
	got := f.last(t)
	require.Equal(t, http.MethodDelete, got.Method)
	require.Equal(t, "/pub/crm/private/doc.pdf", got.Path)
}

func TestS3_RejectsUnsafeKeys(t *testing.T) {
	_, srv := newFakeS3(t)
	pub, priv, err := New(s3FactoryCfg(srv.URL))
	require.NoError(t, err)
	for _, st := range []Store{pub, priv} {
		for _, bad := range []string{"", "/etc/passwd", "../ngoai.jpg", "a/../../ngoai.jpg", `a\b`} {
			require.ErrorIs(t, st.Save(context.Background(), strings.NewReader("x"), bad, "image/png"), ErrUnsafePath, bad)
			require.ErrorIs(t, st.Delete(context.Background(), bad), ErrUnsafePath, bad)
			_, _, err := st.Open(context.Background(), bad)
			require.ErrorIs(t, err, ErrUnsafePath, bad)
		}
	}
}

// R2: KHÔNG gửi ACL ở cả 2 kho, private ở bucket riêng, không aws-chunked trailer.
func TestR2_NoACLAndPrivateBucket(t *testing.T) {
	f, srv := newFakeS3(t)
	cfg := s3FactoryCfg(srv.URL)
	cfg.Driver = DriverR2
	cfg.S3PrivateBucket = "pub-private"
	cfg.S3PublicACL = "public-read" // bị bỏ qua với R2
	pub, priv, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, pub.Save(context.Background(), strings.NewReader("png"), "a.png", "image/png"))
	got := f.last(t)
	require.Empty(t, got.ACL, "R2 không hỗ trợ ACL object — không được gửi")
	require.Equal(t, "/pub/crm/a.png", got.Path)
	require.NotContains(t, got.ContentEncoding, "aws-chunked")

	require.NoError(t, priv.Save(context.Background(), strings.NewReader("pdf"), "b.pdf", "application/pdf"))
	got = f.last(t)
	require.Empty(t, got.ACL)
	require.Equal(t, "/pub-private/crm/private/b.pdf", got.Path)
	require.Empty(t, priv.URL("b.pdf"))
	require.Equal(t, "https://cdn.example.com/crm/a.png", pub.URL("a.png"))
}

func TestS3_URLWithoutCDN(t *testing.T) {
	s, err := NewS3(S3Config{Key: "k", Secret: "s", Region: "hcm", Bucket: "b", Endpoint: "https://s3.example.com/", Folder: "f", Public: true})
	require.NoError(t, err)
	require.Equal(t, "https://s3.example.com/b/f/x.png", s.URL("x.png"))

	aws, err := NewS3(S3Config{Key: "k", Secret: "s", Region: "ap-southeast-1", Bucket: "b", Public: true})
	require.NoError(t, err)
	require.Equal(t, "https://b.s3.ap-southeast-1.amazonaws.com/x.png", aws.URL("x.png"))
}

func TestIsNotFound_GenericErrorIsNot(t *testing.T) {
	require.False(t, isNotFound(errors.New("boom")))
}

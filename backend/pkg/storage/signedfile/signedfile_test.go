package signedfile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const content = "0123456789abcdefghij"

type env struct {
	ls  *storage.LocalSigned
	dir string
	r   *gin.Engine
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	l, err := storage.NewLocal(filepath.Join(dir, "private"), "", "", false)
	require.NoError(t, err)
	ls, err := storage.NewLocalSigned(l, []byte("k-0123456789abcdef0123456789abcdef"), "")
	require.NoError(t, err)
	require.NoError(t, ls.Save(context.Background(), strings.NewReader(content), "crm/attachments/1/a.mp4", "video/mp4"))
	r := gin.New()
	Mount(r.Group("/api"), ls)
	return &env{ls: ls, dir: dir, r: r}
}

func (e *env) sign(t *testing.T, key string, ttl time.Duration) string {
	t.Helper()
	u, err := e.ls.SignedURL(context.Background(), key, ttl,
		storage.SignOptions{ContentType: "video/mp4", Disposition: storage.DispositionInline, FileName: "a.mp4"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(u, storage.LocalSignedPath), "STORAGE_PUBLIC_BASE trống ⇒ URL tương đối gốc site: %s", u)
	return u
}

func get(r http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSignedFile_ValidServesWithHeaders(t *testing.T) {
	e := newEnv(t)
	w := get(e.r, e.sign(t, "crm/attachments/1/a.mp4", 5*time.Minute), nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, content, w.Body.String())
	require.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
	require.Equal(t, `inline; filename=a.mp4`, w.Header().Get("Content-Disposition"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
	cc := w.Header().Get("Cache-Control")
	require.True(t, strings.HasPrefix(cc, "private, max-age="), cc)
	require.LessOrEqual(t, len(cc), len("private, max-age=300"), "max-age không vượt TTL: %s", cc)
}

// HTTP Range ⇒ 206 đúng byte + Content-Range (tua video).
func TestSignedFile_Range206(t *testing.T) {
	e := newEnv(t)
	u := e.sign(t, "crm/attachments/1/a.mp4", 5*time.Minute)
	w := get(e.r, u, map[string]string{"Range": "bytes=2-5"})
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, content[2:6], w.Body.String())
	require.Equal(t, "bytes 2-5/20", w.Header().Get("Content-Range"))

	w = get(e.r, u, map[string]string{"Range": "bytes=15-"})
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, content[15:], w.Body.String())
}

func TestSignedFile_TamperedForbidden(t *testing.T) {
	e := newEnv(t)
	u := e.sign(t, "crm/attachments/1/a.mp4", 5*time.Minute)
	tok := strings.TrimPrefix(u, storage.LocalSignedPath)
	payload, sig, _ := strings.Cut(tok, ".")
	// Đổi 1 ký tự payload / chữ ký ⇒ 403.
	flip := func(s string) string {
		b := []byte(s)
		if b[0] == 'A' {
			b[0] = 'B'
		} else {
			b[0] = 'A'
		}
		return string(b)
	}
	for _, bad := range []string{flip(payload) + "." + sig, payload + "." + flip(sig), "rac", payload} {
		w := get(e.r, storage.LocalSignedPath+bad, nil)
		require.Equal(t, http.StatusForbidden, w.Code, bad)
		require.NotContains(t, w.Body.String(), content)
	}
	// Đối chứng: URL gốc vẫn 200.
	require.Equal(t, http.StatusOK, get(e.r, u, nil).Code)
}

func TestSignedFile_ExpiredGone(t *testing.T) {
	e := newEnv(t)
	// ttl 1ns ⇒ hạn = giây hiện tại ⇒ hết hạn ngay khi verify (Unix giây, now >= exp).
	w := get(e.r, e.sign(t, "crm/attachments/1/a.mp4", time.Nanosecond), nil)
	require.Equal(t, http.StatusGone, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), content)
}

// File chỉ có ở NGOÀI kho private (thư mục public cạnh đó) ⇒ không phục vụ; file đã xoá ⇒ 404.
func TestSignedFile_KeyNotInPrivateStore404(t *testing.T) {
	e := newEnv(t)
	pub := filepath.Join(e.dir, "uploads")
	require.NoError(t, os.MkdirAll(pub, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pub, "only-public.png"), []byte("pub"), 0o600))
	require.Equal(t, http.StatusNotFound, get(e.r, e.sign(t, "only-public.png", time.Minute), nil).Code)

	u := e.sign(t, "crm/attachments/1/a.mp4", time.Minute)
	require.NoError(t, e.ls.Delete(context.Background(), "crm/attachments/1/a.mp4"))
	require.Equal(t, http.StatusNotFound, get(e.r, u, nil).Code)
}

// Kho không phải LocalSigned (driver s3/r2, hoặc chưa cấu hình) ⇒ route vẫn có nhưng luôn 404.
func TestSignedFile_NonLocalStore404(t *testing.T) {
	e := newEnv(t)
	u := e.sign(t, "crm/attachments/1/a.mp4", time.Minute)
	r := gin.New()
	Mount(r.Group("/api"), nil)
	require.Equal(t, http.StatusNotFound, get(r, u, nil).Code)
}

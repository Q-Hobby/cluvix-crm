package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func get(r *gin.Engine, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

// Kho public local ⇒ phục vụ static tại prefix, có Cache-Control, KHÔNG liệt kê thư mục.
func TestMountPublicUploads_LocalPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	pub, err := storage.NewLocal(root, "/uploads", "", true)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "branch"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "branch", "a.png"), []byte("png"), 0o600))

	r := gin.New()
	mountPublicUploads(r, pub)

	w := get(r, "/uploads/branch/a.png")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "png", w.Body.String())
	require.Equal(t, publicUploadsCacheControl, w.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))

	require.NotEqual(t, http.StatusOK, get(r, "/uploads/branch/").Code, "không liệt kê thư mục")
	require.NotEqual(t, http.StatusOK, get(r, "/uploads/").Code, "không liệt kê thư mục gốc")
}

// Kho private (hoặc driver không phải local) ⇒ KHÔNG mount gì.
func TestMountPublicUploads_NeverMountsPrivate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	priv, err := storage.NewLocal(root, "/uploads", "", false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "x.pdf"), []byte("pdf"), 0o600))

	r := gin.New()
	mountPublicUploads(r, priv)
	mountPublicUploads(r, nil)
	require.Equal(t, http.StatusNotFound, get(r, "/uploads/x.pdf").Code)
}

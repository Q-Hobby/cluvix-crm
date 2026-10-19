//go:build integration

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/storage"

	"github.com/stretchr/testify/require"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// Upload avatar khách ⇒ DB lưu KEY (đường dẫn tương đối), response + GET trả URL đầy đủ của kho public.
func TestCreateAvatar_StoresKeyReturnsFullURL(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	pub, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"), "/uploads", "https://api.example.test", true)
	require.NoError(t, err)
	prev := storage.DefaultPublic()
	storage.SetDefaultPublic(pub)
	t.Cleanup(func() { storage.SetDefaultPublic(prev) })

	id := seedCustomer(t, "Khach Avatar Storage")
	t.Cleanup(func() { testDB.Exec("DELETE FROM customer WHERE id = ?", id) })

	h := &Handler{DB: testDB, Store: pub}
	r := testhelpers.NewGin()
	g := r.Group("/api/crm", testhelpers.AuthMiddleware(0, 1))
	g.POST("/customer/:id/create-avatar", h.CreateAvatar)
	g.GET("/customer/:id", h.GetCustomer)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("fileImage", "Anh Khach.png")
	require.NoError(t, err)
	_, _ = fw.Write(pngBytes(t, 640, 480))
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/crm/customer/%d/create-avatar", id), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			Avatar string `json:"avatar"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	var key string
	require.NoError(t, testDB.Table("customer").Where("id = ?", id).Pluck("avatar", &key).Error)
	require.NotEmpty(t, key)
	require.False(t, storage.IsAbsoluteURL(key), "DB phải lưu KEY, nhận %q", key)
	require.True(t, strings.HasPrefix(key, "customer/"), key)
	require.Equal(t, "https://api.example.test/uploads/"+key, resp.Data.Avatar)
	_, err = os.Stat(filepath.Join(pub.Root(), filepath.FromSlash(key)))
	require.NoError(t, err, "file avatar phải nằm trong kho public")

	// GET trả URL đầy đủ (ghép lúc đọc).
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/crm/customer/%d", id), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "https://api.example.test/uploads/"+key, got.Data["avatar"])

	// Dữ liệu cũ: URL tuyệt đối trả nguyên.
	legacy := "https://cdn.old.example.com/crm-cluvix/customer/x/avatar/a.webp"
	require.NoError(t, testDB.Table("customer").Where("id = ?", id).Update("avatar", legacy).Error)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/crm/customer/%d", id), nil))
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, legacy, got.Data["avatar"])
}

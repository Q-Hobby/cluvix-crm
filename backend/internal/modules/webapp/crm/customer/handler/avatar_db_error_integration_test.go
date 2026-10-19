//go:build integration

package handler

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/storage"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// #2: UPDATE avatar lỗi ⇒ 500, ảnh MỚI vừa upload bị dọn, ảnh CŨ còn nguyên, DB vẫn trỏ ảnh cũ.
// Trước đây `.Error` bị bỏ qua: trả 201 + xoá ảnh cũ ⇒ DB trỏ tới file đã mất, avatar hỏng vĩnh viễn.
func TestCreateAvatar_DBErrorKeepsOldRemovesNew(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	if logger.L == nil {
		logger.L = zap.NewNop() // response.ServerError ghi log qua logger.L
	}
	pub, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"), "/uploads", "", true)
	require.NoError(t, err)

	id := seedCustomer(t, "Khach Avatar DB Loi")
	t.Cleanup(func() { testDB.Exec("DELETE FROM customer WHERE id = ?", id) })
	oldKey := fmt.Sprintf("customer/tmp_%d/avatar/old.webp", id)
	require.NoError(t, pub.Save(t.Context(), strings.NewReader("old"), oldKey, "image/webp"))
	require.NoError(t, testDB.Table("customer").Where("id = ?", id).Update("avatar", oldKey).Error)

	h := &Handler{DB: testhelpers.FailingWritesDB(t, testDB), Store: pub}
	r := testhelpers.NewGin()
	r.Group("/api/crm", testhelpers.AuthMiddleware(0, 1)).POST("/customer/:id/create-avatar", h.CreateAvatar)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("fileImage", "a.png")
	require.NoError(t, err)
	_, _ = fw.Write(pngBytes(t, 640, 480))
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/crm/customer/%d/create-avatar", id), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	require.Equal(t, []string{oldKey}, testhelpers.ListStoreFiles(t, pub.Root()), "chỉ còn ảnh cũ: ảnh mới phải bị dọn, ảnh cũ giữ nguyên")
	var cur string
	require.NoError(t, testDB.Table("customer").Where("id = ?", id).Pluck("avatar", &cur).Error)
	require.Equal(t, oldKey, cur)
}

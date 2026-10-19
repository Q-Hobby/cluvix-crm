//go:build integration

package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/storage"
)

// #3: upload avatar xong mà ghi DB lỗi (lỗi SQL HOẶC 409 khoá lạc quan) ⇒ ảnh MỚI bị dọn, ảnh CŨ
// còn nguyên, cột avatar không đổi.
func TestUserAvatar_DBErrorCleansNewKeepsOld(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	pub, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"), "/uploads", "", true)
	require.NoError(t, err)

	uid := testhelpers.CreateUser(t, testDB, "upav", nil)
	oldKey := fmt.Sprintf("user-profile/%d/avatar/old.webp", uid)
	require.NoError(t, pub.Save(t.Context(), strings.NewReader("old"), oldKey, "image/webp"))
	const ver int64 = 1_700_000_000
	require.NoError(t, testDB.Exec("INSERT INTO user_profile (user_id, fullname, avatar, updated_at) VALUES (?, ?, ?, ?)",
		uid, "Avatar User", oldKey, ver).Error)
	t.Cleanup(func() { testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid) })

	call := func(db *gorm.DB, path, fileField string, fields map[string]string) *httptest.ResponseRecorder {
		h := New(Deps{DB: db, Store: pub})
		r := testhelpers.NewGin()
		g := r.Group("", testhelpers.AuthMiddleware(0, uid))
		g.POST("/update-user-profile", h.UpdateUserProfile)
		g.POST("/create-avatar-user", h.CreateAvatarUser)
		body, ct := testhelpers.MultipartBody(t, fields, fileField, "a.png", testhelpers.PNGBytes(t, 320, 320))
		req := httptest.NewRequest(http.MethodPost, path, body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	assertOnlyOld := func(msg string) {
		t.Helper()
		require.Equal(t, []string{oldKey}, testhelpers.ListStoreFiles(t, pub.Root()), msg)
		var cur string
		require.NoError(t, testDB.Table("user_profile").Where("user_id = ?", uid).Pluck("avatar", &cur).Error)
		require.Equal(t, oldKey, cur, msg)
	}
	verField := map[string]string{"updated_at": fmt.Sprint(ver)}

	// ── UpdateUserProfile: lỗi SQL ⇒ 500.
	w := call(testhelpers.FailingWritesDB(t, testDB), "/update-user-profile", "avatarFile", verField)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assertOnlyOld("update lỗi SQL")

	// ── UpdateUserProfile: request khác ghi chen ngay trước UPDATE ⇒ 409, ảnh mới vẫn phải bị dọn.
	// Ghi chen qua CHÍNH connection/tx của lệnh đang chạy (suite giới hạn 1 connection — dùng testDB
	// ở đây sẽ chờ connection mà tx đang giữ ⇒ deadlock).
	racy := testhelpers.BeforeWriteHookDB(t, testDB, func(tx *gorm.DB) {
		tx.Session(&gorm.Session{NewDB: true}).
			Exec("UPDATE user_profile SET updated_at = ? WHERE user_id = ?", time.Now().Unix()+3600, uid)
	})
	w = call(racy, "/update-user-profile", "avatarFile", verField)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assertOnlyOld("update 409")

	// ── CreateAvatarUser: lỗi SQL ⇒ 500.
	w = call(testhelpers.FailingWritesDB(t, testDB), "/create-avatar-user", "image", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assertOnlyOld("create-avatar lỗi SQL")
}

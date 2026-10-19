//go:build integration

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/storage"
)

// #1 (2026-10-19): chuỗi `avatar` trong payload chỉ được là ảnh hiện tại hoặc rỗng — key khác ⇒ 422.
// Ảnh cũ chỉ bị xoá khi nằm dưới `user-profile/<uid>/avatar/`: dữ liệu bị nhét key lạ từ trước bản
// vá (logo thương hiệu, file đính kèm riêng tư `private/...`) không bao giờ bị xoá.
func TestUserAvatar_PayloadKeyPolicyAndDeletePrefix(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	pub, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"), "/uploads", "https://api.example.test", true)
	require.NoError(t, err)

	uid := testhelpers.CreateUser(t, testDB, "upkey", nil)
	own := fmt.Sprintf("user-profile/%d/avatar/me.webp", uid)
	const logo = "business_profile/logo.webp"
	const priv = "private/customer/1/att.pdf"
	for _, k := range []string{own, logo, priv} {
		require.NoError(t, pub.Save(t.Context(), strings.NewReader("x"), k, "image/webp"))
	}
	require.NoError(t, testDB.Exec("INSERT INTO user_profile (user_id, fullname, avatar, updated_at) VALUES (?, ?, ?, ?)",
		uid, "Key Policy", own, 1_700_000_000).Error)
	t.Cleanup(func() { testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid) })

	h := New(Deps{DB: testDB, Store: pub})
	r := testhelpers.NewGin()
	g := r.Group("", testhelpers.AuthMiddleware(0, uid))
	g.POST("/update-user-profile", h.UpdateUserProfile)
	g.POST("/create-avatar-user", h.CreateAvatarUser)

	state := func() (avatar string, ver int64) {
		t.Helper()
		var row struct {
			Avatar    string
			UpdatedAt int64
		}
		require.NoError(t, testDB.Table("user_profile").Select("avatar, updated_at").Where("user_id = ?", uid).Take(&row).Error)
		return row.Avatar, row.UpdatedAt
	}
	setAvatar := func(v string) {
		t.Helper()
		require.NoError(t, testDB.Exec("UPDATE user_profile SET avatar = ? WHERE user_id = ?", v, uid).Error)
	}
	exists := func(k string) bool {
		_, err := os.Stat(filepath.Join(pub.Root(), filepath.FromSlash(k)))
		return err == nil
	}
	sendJSON := func(avatar any) *httptest.ResponseRecorder {
		t.Helper()
		_, ver := state()
		raw, _ := json.Marshal(map[string]any{"avatar": avatar, "updated_at": fmt.Sprint(ver)})
		req := httptest.NewRequest(http.MethodPost, "/update-user-profile", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	sendFile := func(path, field string, fields map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		body, ct := testhelpers.MultipartBody(t, fields, field, "a.png", testhelpers.PNGBytes(t, 320, 320))
		req := httptest.NewRequest(http.MethodPost, path, body)
		req.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// ── Tấn công: gán key logo / file riêng tư ⇒ 422, DB không đổi.
	for _, bad := range []string{logo, priv, pub.URL(logo)} {
		w := sendJSON(bad)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "Ảnh không hợp lệ") // i18n image_key_not_allowed
		cur, _ := state()
		require.Equal(t, own, cur, "payload %q KHÔNG được ghi vào avatar", bad)
	}
	require.True(t, exists(logo) && exists(priv) && exists(own))

	// ── Gửi lại nguyên URL GET đã trả ⇒ 200, không đổi, file còn.
	w := sendJSON(pub.URL(own))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	cur, _ := state()
	require.Equal(t, own, cur)
	require.True(t, exists(own))

	// ── Xoá ảnh của chính mình ⇒ file của mình bị xoá.
	w = sendJSON("")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	cur, _ = state()
	require.Empty(t, cur)
	require.False(t, exists(own), "xoá avatar của chính mình ⇒ file phải bị xoá")

	// ── Dữ liệu cũ đã bị nhét key lạ (trước bản vá): gửi rỗng ⇒ DB về rỗng nhưng file KHÔNG bị xoá.
	for _, poisoned := range []string{logo, priv} {
		setAvatar(poisoned)
		w = sendJSON("")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.True(t, exists(poisoned), "%s nằm ngoài prefix avatar — KHÔNG được xoá", poisoned)
	}

	// ── Upload mới (update-user-profile + create-avatar-user) khi DB trỏ key lạ ⇒ key lạ còn nguyên.
	setAvatar(priv)
	_, ver := state()
	w = sendFile("/update-user-profile", "avatarFile", map[string]string{"updated_at": fmt.Sprint(ver)})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, exists(priv), "private/... KHÔNG bao giờ bị xoá")
	setAvatar(logo)
	w = sendFile("/create-avatar-user", "image", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, exists(logo), "logo KHÔNG được xoá khi đổi avatar")

	// ── Upload mới khi ảnh cũ nằm trong prefix ⇒ ảnh cũ bị xoá, ảnh mới nằm trong prefix.
	prev, _ := state()
	require.True(t, strings.HasPrefix(prev, avatarPrefix(uid)), prev)
	require.True(t, exists(prev))
	_, ver = state()
	w = sendFile("/update-user-profile", "avatarFile", map[string]string{"updated_at": fmt.Sprint(ver), "avatar": pub.URL(prev)})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	next, _ := state()
	require.NotEqual(t, prev, next)
	require.True(t, strings.HasPrefix(next, avatarPrefix(uid)), next)
	require.True(t, exists(next))
	require.False(t, exists(prev), "upload mới ⇒ ảnh cũ trong prefix phải bị xoá")
	require.True(t, exists(logo) && exists(priv))
}

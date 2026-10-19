//go:build integration

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	customerService "erpCluvix/internal/modules/webapp/crm/customer/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/storage"
)

// #1 (2026-10-19): chuỗi `avatar` trong payload — TẠO: bị bỏ qua; SỬA: chỉ ảnh hiện tại hoặc rỗng
// (khác ⇒ 422). CreateAvatar chỉ xoá ảnh cũ nằm dưới `customer/<mã|tmp_id>/avatar/` — trước đây
// gán key logo cho khách rồi upload avatar mới là xoá được logo (hoặc file riêng tư `private/...`).
func TestCustomerAvatar_PayloadKeyPolicyAndDeletePrefix(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	pub, err := storage.NewLocal(filepath.Join(t.TempDir(), "uploads"), "/uploads", "https://api.example.test", true)
	require.NoError(t, err)

	tn := testhelpers.FreshTenant(t, testDB)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM note WHERE customer_id IN (SELECT id FROM (SELECT id FROM customer WHERE fullname LIKE 'K1 AV %') x)")
		testDB.Exec("DELETE FROM customer WHERE fullname LIKE 'K1 AV %'")
	})
	uid := testhelpers.CreateScopedUser(t, testDB, "k1av", tn, tn.BranchID)

	id := seedCustomer(t, fmt.Sprintf("K1 AV seed %d", time.Now().UnixNano()))
	own := fmt.Sprintf("customer/tmp_%d/avatar/me.webp", id)
	const logo = "business_profile/logo.webp"
	const priv = "private/customer/1/att.pdf"
	for _, k := range []string{own, logo, priv} {
		require.NoError(t, pub.Save(t.Context(), strings.NewReader("x"), k, "image/webp"))
	}
	exists := func(k string) bool {
		_, err := os.Stat(filepath.Join(pub.Root(), filepath.FromSlash(k)))
		return err == nil
	}
	setAvatar := func(v string) {
		t.Helper()
		require.NoError(t, testDB.Exec("UPDATE customer SET avatar = ? WHERE id = ?", v, id).Error)
	}
	state := func() (avatar string, ver int64) {
		t.Helper()
		var row struct {
			Avatar    string
			UpdatedAt int64
		}
		require.NoError(t, testDB.Table("customer").Select("avatar, updated_at").Where("id = ?", id).Take(&row).Error)
		return row.Avatar, row.UpdatedAt
	}
	setAvatar(own)

	h := &Handler{DB: testDB, Store: pub, CustomerSvc: customerService.NewCustomerService(testDB)}
	ctx := func(w *httptest.ResponseRecorder, method, path string, body *bytes.Buffer, ct string) *gin.Context {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(w)
		c.Set("branchActiveId", tn.BranchID)
		c.Set("branchIds", []uint{tn.BranchID})
		c.Set("userID", uid)
		c.Request = httptest.NewRequest(method, path, body)
		c.Request.Header.Set("Content-Type", ct)
		c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
		return c
	}
	update := func(avatar any) *httptest.ResponseRecorder {
		t.Helper()
		_, ver := state()
		raw, _ := json.Marshal(map[string]any{"avatar": avatar, "updated_at": ver, "nickname": "k1"})
		w := httptest.NewRecorder()
		h.UpdateCustomer(ctx(w, http.MethodPut, "/crm/customer/x", bytes.NewBuffer(raw), "application/json"))
		return w
	}
	upload := func() *httptest.ResponseRecorder {
		t.Helper()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("fileImage", "a.png")
		require.NoError(t, err)
		_, _ = fw.Write(testhelpers.PNGBytes(t, 320, 320))
		require.NoError(t, mw.Close())
		w := httptest.NewRecorder()
		h.CreateAvatar(ctx(w, http.MethodPost, "/crm/customer/x/create-avatar", &buf, mw.FormDataContentType()))
		return w
	}

	// ── SỬA: gán key logo / file riêng tư ⇒ 422, không đổi.
	for _, bad := range []string{logo, priv, pub.URL(logo)} {
		w := update(bad)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
		cur, _ := state()
		require.Equal(t, own, cur, "payload %q KHÔNG được ghi", bad)
	}
	// Gửi lại URL hiện tại ⇒ 200, không đổi.
	w := update(pub.URL(own))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	cur, _ := state()
	require.Equal(t, own, cur)
	require.True(t, exists(own))

	// ── TẠO kèm avatar = key logo ⇒ bỏ qua.
	name := fmt.Sprintf("K1 AV create %d", time.Now().UnixNano())
	raw, _ := json.Marshal(map[string]any{"fullname": name, "branch_id": tn.BranchID, "payment_user_id": uid, "avatar": logo})
	w = httptest.NewRecorder()
	h.CreateCustomer(ctx(w, http.MethodPost, "/crm/customer", bytes.NewBuffer(raw), "application/json"))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct{ Avatar string }
	require.NoError(t, testDB.Table("customer").Select("COALESCE(avatar, '') AS avatar").Where("fullname = ?", name).Take(&created).Error)
	require.Empty(t, created.Avatar, "đường TẠO phải bỏ qua avatar client gửi")

	// ── Dữ liệu cũ bị nhét key lạ: upload avatar mới ⇒ key lạ KHÔNG bị xoá.
	for _, poisoned := range []string{logo, priv} {
		setAvatar(poisoned)
		w = upload()
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		require.True(t, exists(poisoned), "%s nằm ngoài prefix avatar khách — KHÔNG được xoá", poisoned)
	}

	// ── Upload mới khi ảnh cũ trong prefix ⇒ ảnh cũ bị xoá.
	setAvatar(own)
	w = upload()
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	next, _ := state()
	require.NotEqual(t, own, next)
	require.True(t, strings.HasPrefix(next, fmt.Sprintf("customer/tmp_%d/avatar/", id)), next)
	require.True(t, exists(next))
	require.False(t, exists(own), "upload mới ⇒ ảnh cũ trong prefix phải bị xoá")
	require.True(t, exists(logo) && exists(priv))
}

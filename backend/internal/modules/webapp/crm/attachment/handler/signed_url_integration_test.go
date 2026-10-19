//go:build integration

package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	customerEntity "erpCluvix/internal/modules/webapp/crm/customer/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/storage/signedfile"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// signedEnv — kho private local CÓ khoá ký + engine gồm route attachment (sau auth giả) và route
// công khai /api/files/signed/:token (KHÔNG auth) y như router thật.
type signedEnv struct {
	*attEnv
	ls *storage.LocalSigned
}

func newSignedEnv(t *testing.T) *signedEnv {
	t.Helper()
	e := newAttEnv(t)
	ls, err := storage.NewLocalSigned(e.priv, []byte("k6-test-signing-key-0123456789abcdef"), "")
	require.NoError(t, err)
	return &signedEnv{attEnv: e, ls: ls}
}

func (e *signedEnv) handler(ttl time.Duration) *Handler {
	return New(Deps{DB: testDB, PrivateStore: e.ls, PublicStore: e.pub, SignedURLTTL: ttl})
}

func (e *signedEnv) router(uid uint, h *Handler) *gin.Engine {
	r := testhelpers.NewGin()
	signedfile.Mount(r.Group("/api"), e.ls)
	g := r.Group("/api/crm", testhelpers.AuthMiddleware(0, uid))
	g.POST("/customer/:id/attachments", h.UploadForCustomer)
	g.POST("/attachment/:id/signed-url", h.SignedURL)
	return r
}

// publicOnly — engine CHỈ có route công khai, không middleware auth nào: chứng minh URL ký dùng được
// mà không cần JWT.
func (e *signedEnv) publicOnly() *gin.Engine {
	r := testhelpers.NewGin()
	signedfile.Mount(r.Group("/api"), e.ls)
	return r
}

func (e *signedEnv) upload(t *testing.T, r *gin.Engine, cid uint) uint {
	t.Helper()
	body, ct := multipartBody(t, "Ho So.pdf", []byte(testPDF))
	w := do(r, http.MethodPost, fmt.Sprintf("/api/crm/customer/%d/attachments", cid), body, ct)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return uint(decodeData(t, w)["id"].(float64))
}

func signedURLOf(t *testing.T, r *gin.Engine, id uint, body string) (int, map[string]any) {
	t.Helper()
	var rd io.Reader
	ct := ""
	if body != "" {
		rd, ct = strings.NewReader(body), "application/json"
	}
	w := do(r, http.MethodPost, fmt.Sprintf("/api/crm/attachment/%d/signed-url", id), rd, ct)
	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	return w.Code, decodeData(t, w)
}

func TestAttachmentSignedURL_ScopeAndPublicAccess(t *testing.T) {
	e := newSignedEnv(t)
	optionTeleOff(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	tele := seedTelesale(t)
	cid := seedCustomer(t, owner)
	h := e.handler(5 * time.Minute)
	r := e.router(owner, h)
	attID := e.upload(t, r, cid)

	// ── trong phạm vi ⇒ URL ký, dùng được KHÔNG cần JWT
	before := time.Now().Unix()
	code, data := signedURLOf(t, r, attID, "")
	require.Equal(t, http.StatusOK, code)
	url := data["url"].(string)
	require.True(t, strings.HasPrefix(url, storage.LocalSignedPath), url)
	require.Equal(t, false, data["legacy"])
	exp := int64(data["expires_at"].(float64))
	require.GreaterOrEqual(t, exp, before+300)
	require.LessOrEqual(t, exp, time.Now().Unix()+300)

	w := do(e.publicOnly(), http.MethodGet, url, nil, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, testPDF, w.Body.String())
	require.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	require.True(t, strings.HasPrefix(w.Header().Get("Content-Disposition"), "inline"), w.Header().Get("Content-Disposition"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "Ho So.pdf")

	// download=true ⇒ attachment.
	code, data = signedURLOf(t, r, attID, `{"download":true}`)
	require.Equal(t, http.StatusOK, code)
	w = do(e.publicOnly(), http.MethodGet, data["url"].(string), nil, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment"), w.Header().Get("Content-Disposition"))

	// Body rác ⇒ 422, không cấp URL.
	code, _ = signedURLOf(t, r, attID, `{"download":`)
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// ── ngoài phạm vi telesale ⇒ 404 (không lộ sự tồn tại), không cấp URL
	code, _ = signedURLOf(t, e.router(tele, h), attID, "")
	require.Equal(t, http.StatusNotFound, code)
	// Đối chứng: khách do chính telesale phụ trách ⇒ cấp được (404 ở trên là do scope).
	cidTele := seedCustomer(t, tele)
	rt := e.router(tele, h)
	code, _ = signedURLOf(t, rt, e.upload(t, rt, cidTele), "")
	require.Equal(t, http.StatusOK, code)

	// ── id rác / không tồn tại ⇒ 404
	for _, p := range []string{"abc", fmt.Sprintf("%dabc", attID), "999999999"} {
		w = do(r, http.MethodPost, "/api/crm/attachment/"+p+"/signed-url", nil, "")
		require.Equal(t, http.StatusNotFound, w.Code, p)
	}
}

// URL hết hạn ⇒ bị từ chối (410), không trả nội dung.
func TestAttachmentSignedURL_ExpiredRejected(t *testing.T) {
	e := newSignedEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	// ttl 1ns ⇒ hạn = giây hiện tại ⇒ hết hạn ngay khi dùng.
	r := e.router(owner, e.handler(time.Nanosecond))
	attID := e.upload(t, r, cid)
	code, data := signedURLOf(t, r, attID, "")
	require.Equal(t, http.StatusOK, code)
	w := do(e.publicOnly(), http.MethodGet, data["url"].(string), nil, "")
	require.Equal(t, http.StatusGone, w.Code)
	require.NotContains(t, w.Body.String(), "%PDF")
}

// Bản ghi cũ (URL tuyệt đối public) ⇒ trả nguyên URL + legacy=true (như download redirect).
func TestAttachmentSignedURL_Legacy(t *testing.T) {
	e := newSignedEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	legacy := customerEntity.Attachment{RefType: "customer", RefId: int(cid), CustomerID: cid,
		FileName: "cu.pdf", FileUrl: "https://cdn.old.example.com/crm/attachments/cu.pdf", FileType: "application/pdf"}
	require.NoError(t, testDB.Create(&legacy).Error)

	code, data := signedURLOf(t, e.router(owner, e.handler(0)), legacy.ID, "")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, legacy.FileUrl, data["url"])
	require.Equal(t, true, data["legacy"])
}

// Kho private không ký được (fake/thiếu cấu hình) ⇒ 500, không rơi về URL nào khác.
func TestAttachmentSignedURL_StoreWithoutSigner500(t *testing.T) {
	e := newSignedEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	upR := e.router(owner, e.handler(0))
	attID := e.upload(t, upR, cid)
	h := New(Deps{DB: testDB, PrivateStore: e.priv, PublicStore: e.pub}) // *storage.Local trần, không Signer
	code, _ := signedURLOf(t, e.router(owner, h), attID, "")
	require.Equal(t, http.StatusInternalServerError, code)
}

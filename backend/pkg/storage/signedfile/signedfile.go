// Package signedfile — route công khai phục vụ URL ký ngắn hạn của kho private driver local.
//
// Tách khỏi internal/router để test tích hợp của module (vd crm/attachment) dựng đúng route thật
// mà không vòng import router → module.
package signedfile

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
)

// Mount đăng ký GET /api/files/signed/:token — phục vụ URL ký ngắn hạn của kho PRIVATE
// driver local (storage.LocalSigned).
//
// CÔNG KHAI (không JWT, không PermissionEnforcer — đăng ký trên group `api`, KHÔNG phải `authed`):
// xác thực bằng CHỮ KÝ HMAC trong token, cấp bởi POST /api/crm/attachment/:id/signed-url sau khi đã
// kiểm JWT + perm + scope. Vì không qua AuthResponseHeaders nên gate must-change-password/2FA/IP
// allowlist cũng không chặn — đúng ý: các gate đó đã chạy lúc CẤP URL.
//
// Route luôn được đăng ký (bộ route ổn định cho route_perm_map/coverage test); kho không phải
// LocalSigned (driver s3/r2 — URL ký trỏ thẳng object storage) ⇒ 404.
func Mount(api gin.IRouter, private storage.Store) {
	ls, _ := private.(*storage.LocalSigned)
	api.GET(strings.TrimPrefix(storage.LocalSignedPath, "/api")+":token", Handler(ls))
}

// Handler — GET /api/files/signed/:token. ls nil (kho không phải LocalSigned) ⇒ luôn 404.
func Handler(ls *storage.LocalSigned) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ls == nil {
			response.NotFound(c)
			return
		}
		obj, err := ls.Verify(c.Param("token"))
		if errors.Is(err, storage.ErrSignatureExpired) {
			response.Gone(c, i18n.T(c, "signed_url_expired"))
			return
		}
		if err != nil {
			response.Forbidden(c)
			return
		}
		// Key chỉ resolve trong thư mục gốc kho private (chặn traversal) — key ngoài kho ⇒ 404.
		f, fi, err := ls.OpenFile(obj.Key)
		if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrUnsafePath) {
			response.NotFound(c)
			return
		}
		if err != nil {
			response.ServerError(c, err)
			return
		}
		defer f.Close()

		// Cache riêng tư, không dài hơn phần hạn còn lại của chữ ký (≤ MaxSignedURLTTL).
		maxAge := obj.Expires - time.Now().Unix()
		if maxAge < 0 {
			maxAge = 0
		}
		if limit := int64(storage.MaxSignedURLTTL / time.Second); maxAge > limit {
			maxAge = limit
		}
		h := c.Writer.Header()
		h.Set("Content-Type", obj.ContentType)
		h.Set("Content-Disposition", obj.ContentDisposition)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "private, max-age="+strconv.FormatInt(maxAge, 10))
		// Token nằm trong URL ⇒ không để rò qua Referer nếu nội dung (PDF) có link ra ngoài.
		h.Set("Referrer-Policy", "no-referrer")
		// ServeContent: Range/If-Range/HEAD ⇒ 206 + Content-Range đúng byte (tua video/audio). Content-Type
		// đã đặt sẵn nên ServeContent không tự sniff.
		http.ServeContent(c.Writer, c.Request, "", fi.ModTime(), f)
	}
}

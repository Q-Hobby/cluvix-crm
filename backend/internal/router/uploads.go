package router

import (
	"net/http"

	"erpCluvix/pkg/storage"

	"github.com/gin-gonic/gin"
)

// publicUploadsCacheControl — ảnh public có key ngẫu nhiên, đổi ảnh = key mới (không ghi đè), nên
// cache dài an toàn. Không "immutable" vì key cũ có thể bị xoá khi đổi ảnh.
const publicUploadsCacheControl = "public, max-age=604800"

// mountPublicUploads phục vụ kho public driver local tại prefix của nó (vd /uploads).
//
// gin.Dir(root, false) ⇒ KHÔNG liệt kê thư mục. Chỉ mount khi kho là *storage.Local công khai —
// kho s3/r2 hoặc kho private (Public()=false) bị bỏ qua, nên không có đường nào mount nhầm private.
func mountPublicUploads(r *gin.Engine, st storage.Store) {
	l, ok := st.(*storage.Local)
	if !ok || l == nil || !l.Public() {
		return
	}
	g := r.Group(l.PublicPrefix(), func(c *gin.Context) {
		c.Header("Cache-Control", publicUploadsCacheControl)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	g.StaticFS("/", http.FileSystem(gin.Dir(l.Root(), false)))
}

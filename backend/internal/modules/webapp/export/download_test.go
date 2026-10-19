package export

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"

	"erpCluvix/internal/middleware"
	pkgExport "erpCluvix/pkg/export"
	"erpCluvix/pkg/logger"
)

const (
	dlTestSecret   = "download-test-secret"
	dlTestIssuer   = "dltest.issuer"
	dlTestAudience = "dltest.aud"
)

// dlRouter dựng đúng chuỗi middleware thật của nhóm authed tối thiểu cần cho endpoint tải:
// JWTAuth (chữ ký + hạn) → RegisterDownload. Chạy trong thư mục tạm (DefaultDir là đường dẫn tương đối).
func dlRouter(t *testing.T) *gin.Engine {
	t.Helper()
	t.Chdir(t.TempDir())
	gin.SetMode(gin.TestMode)
	middleware.SetJWTVerifier(dlTestSecret, dlTestIssuer, dlTestAudience)
	if logger.L == nil {
		logger.L = zap.NewNop() // JWTAuth gắn logger theo user vào context
	}
	r := gin.New()
	RegisterDownload(r.Group("/api", middleware.JWTAuth(nil)))
	return r
}

func dlToken(t *testing.T, uid uint) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: dlTestIssuer, Audience: jwt.ClaimStrings{dlTestAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UID: uid,
	})
	s, err := tok.SignedString([]byte(dlTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func dlGet(r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// saveOwned ghi 1 workbook thật qua SaveWorkbook (đúng đường sản phẩm) cho owner, trả tên file từ URL.
func saveOwned(t *testing.T, owner uint) string {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	_ = f.SetCellValue("Sheet1", "A1", "pii")
	url, err := pkgExport.SaveWorkbook(f, pkgExport.Config{
		FileName: "Khach-Hang-export", ExportDir: pkgExport.DefaultDir, DownloadURL: "https://crm.test", OwnerID: owner,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "https://crm.test"+pkgExport.DownloadPath) {
		t.Fatalf("URL không trỏ endpoint có auth: %s", url)
	}
	return url[strings.LastIndex(url, "/")+1:]
}

// S-2/O-6: file export chỉ tải được qua JWT, chỉ bởi người tạo, tên phải là basename hợp lệ.
func TestDownloadExport_AuthOwnerVaTenFile(t *testing.T) {
	r := dlRouter(t)
	name := saveOwned(t, 7)
	path := "/api/download/export/" + name

	// Chủ file + token hợp lệ ⇒ 200, có Content-Disposition attachment, nội dung là file xlsx.
	w := dlGet(r, path, dlToken(t, 7))
	if w.Code != http.StatusOK {
		t.Fatalf("owner tải: code=%d body=%s", w.Code, w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, name) {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if w.Body.Len() == 0 || !strings.HasPrefix(w.Body.String(), "PK") {
		t.Fatalf("body không phải xlsx (len=%d)", w.Body.Len())
	}

	// Không token ⇒ 401.
	if w := dlGet(r, path, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("không token: code=%d, muốn 401", w.Code)
	}
	// Token sai chữ ký ⇒ 401.
	if w := dlGet(r, path, dlToken(t, 7)+"x"); w.Code != http.StatusUnauthorized {
		t.Fatalf("token hỏng: code=%d, muốn 401", w.Code)
	}
	// User khác, cùng tên file ⇒ 404 (không lộ là file tồn tại).
	if w := dlGet(r, path, dlToken(t, 8)); w.Code != http.StatusNotFound {
		t.Fatalf("user khác: code=%d, muốn 404", w.Code)
	}

	// Path traversal / tên lạ ⇒ 404, kể cả khi đích traversal là file có thật của user khác.
	other := saveOwned(t, 8)
	for _, bad := range []string{
		"/api/download/export/..%2F8%2F" + other,
		"/api/download/export/%2E%2E%2F8%2F" + other,
		"/api/download/export/..",
		"/api/download/export/.export-123.part",
		"/api/download/export/" + strings.TrimSuffix(name, ".xlsx") + ".txt",
		"/api/download/export/a%5Cb.xlsx",
	} {
		w := dlGet(r, bad, dlToken(t, 7))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: code=%d, muốn 404", bad, w.Code)
		}
	}

	// File đã bị dọn ⇒ 404.
	if err := os.Remove(filepath.Join(pkgExport.OwnerDir(pkgExport.DefaultDir, 7), name)); err != nil {
		t.Fatal(err)
	}
	if w := dlGet(r, path, dlToken(t, 7)); w.Code != http.StatusNotFound {
		t.Fatalf("file đã xoá: code=%d, muốn 404", w.Code)
	}
}

// Symlink trong thư mục owner (trỏ ra ngoài) KHÔNG được phục vụ.
func TestDownloadExport_KhongTheoSymlink(t *testing.T) {
	r := dlRouter(t)
	secret := filepath.Join(t.TempDir(), "secret.xlsx")
	if err := os.WriteFile(secret, []byte("PKsecret"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := pkgExport.OwnerDir(pkgExport.DefaultDir, 7)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "link.xlsx")); err != nil {
		t.Skip("không tạo được symlink:", err)
	}
	if w := dlGet(r, "/api/download/export/link.xlsx", dlToken(t, 7)); w.Code != http.StatusNotFound {
		t.Fatalf("symlink: code=%d, muốn 404", w.Code)
	}
}

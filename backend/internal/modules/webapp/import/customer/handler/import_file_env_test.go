package handler

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

// Đường dẫn file import lấy từ env IMPORT_CUSTOMER_FILE (trước đây gán cứng trong code, kèm tên 1
// khách). Hai nhánh dừng TRƯỚC mọi truy cập DB nên chạy được không cần DB: env rỗng ⇒ 422
// `import_file_not_configured`; env trỏ file không tồn tại ⇒ 422 `excel_read_failed` (chứng minh
// handler mở ĐÚNG đường dẫn trong env, không phải đường dẫn nào khác).
func TestImportCustomerFromExcel_FilePathFromEnv(t *testing.T) {
	gin.SetMode(gin.TestMode)
	missing := filepath.Join(t.TempDir(), "khong-ton-tai.xlsx")
	cases := []struct {
		name    string
		env     string
		wantMsg string
	}{
		{name: "env rỗng ⇒ chưa cấu hình", env: "", wantMsg: "IMPORT_CUSTOMER_FILE"},
		{name: "env chỉ khoảng trắng ⇒ chưa cấu hình", env: "   ", wantMsg: "IMPORT_CUSTOMER_FILE"},
		{name: "env trỏ file không có ⇒ lỗi đọc đúng file đó", env: missing, wantMsg: "khong-ton-tai.xlsx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(define.EnvImportCustomerFile, tc.env)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/import/customer/from-excel", nil)

			(&Handler{}).ImportCustomerFromExcel(c)

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, muốn 422; body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantMsg) {
				t.Fatalf("body thiếu %q: %s", tc.wantMsg, w.Body.String())
			}
		})
	}
}

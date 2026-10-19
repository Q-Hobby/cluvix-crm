package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

// chunkedMultipart — body multipart có 1 file `size` byte, đọc qua io.Reader KHÔNG biết độ dài ⇒
// Content-Length = -1 (giống client gửi Transfer-Encoding: chunked). Trả reader + Content-Type.
func chunkedMultipart(t *testing.T, size int64) (io.Reader, string) {
	t.Helper()
	var head bytes.Buffer
	mw := multipart.NewWriter(&head)
	if _, err := mw.CreateFormFile("file", "big.bin"); err != nil {
		t.Fatal(err)
	}
	tail := "\r\n--" + mw.Boundary() + "--\r\n"
	return io.MultiReader(&head, io.LimitReader(zeroReader{}, size), strings.NewReader(tail)), mw.FormDataContentType()
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

func limitEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.MaxMultipartMemory = MaxMultipartBodyBytes
	r.Use(RequestSizeLimit())
	r.POST("/upload", func(c *gin.Context) {
		if response.RejectMultipartTooLarge(c) {
			return
		}
		if _, err := c.FormFile("file"); err != nil {
			response.HandleError(c, err)
			return
		}
		c.Status(http.StatusOK)
	})
	return r
}

func doUpload(r *gin.Engine, body io.Reader, ct string, contentLength int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", ct)
	req.ContentLength = contentLength
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// #4: request multipart CHUNKED (Content-Length -1) vượt trần ⇒ 413 + i18n payload_too_large.
// Trước đây middleware chỉ so Content-Length nên body chunked lọt nguyên vẹn, không giới hạn.
func TestRequestSizeLimit_ChunkedMultipartOverLimit413(t *testing.T) {
	r := limitEngine()
	body, ct := chunkedMultipart(t, MaxMultipartRequestBytes+1)
	w := doUpload(r, body, ct, -1)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, muốn 413; body=%s", w.Code, w.Body.String())
	}
	var got struct{ Message string }
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if want := i18n.T(ctx, "payload_too_large"); want == "payload_too_large" || got.Message != want {
		t.Fatalf("message = %q, muốn %q", got.Message, want)
	}
}

// Đối chứng: file đúng trần 1 file (32 MB) gửi chunked vẫn qua — trần body chừa biên cho multipart.
func TestRequestSizeLimit_ChunkedMultipartAtFileLimitPasses(t *testing.T) {
	r := limitEngine()
	body, ct := chunkedMultipart(t, MaxMultipartBodyBytes)
	if w := doUpload(r, body, ct, -1); w.Code != http.StatusOK {
		t.Fatalf("status = %d, muốn 200; body=%s", w.Code, w.Body.String())
	}
}

// Content-Length khai vượt trần ⇒ 413 ngay ở middleware (không đọc body).
func TestRequestSizeLimit_DeclaredContentLengthOverLimit413(t *testing.T) {
	r := limitEngine()
	body, ct := chunkedMultipart(t, 10)
	if w := doUpload(r, body, ct, MaxMultipartRequestBytes+1); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, muốn 413", w.Code)
	}
}

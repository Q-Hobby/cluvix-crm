package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// TestJWTAuth_IssuerKhacKhach_401 — R-1: token ký ĐÚNG TOKEN_SECRET nhưng issuer/audience của khách
// khác (DB khác) ⇒ 401; token đúng khách ⇒ qua; cấu hình thiếu issuer ⇒ fail-closed.
func TestJWTAuth_IssuerKhacKhach_401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if logger.L == nil {
		logger.L = zap.NewNop()
	}
	const secret = "shared-secret"
	sign := func(iss, aud string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer: iss, Audience: jwt.ClaimStrings{aud},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
			UID: 1,
		})
		s, err := tok.SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	call := func(tok string) int {
		r := gin.New()
		r.GET("/x", JWTAuth(nil), func(c *gin.Context) { c.Status(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	t.Cleanup(func() { SetJWTVerifier("", "", "") })

	SetJWTVerifier(secret, "C.prod.khach_b.issuer", "C.prod.khach_b.aud")
	if got := call(sign("C.prod.khach_b.issuer", "C.prod.khach_b.aud")); got != http.StatusOK {
		t.Fatalf("token đúng khách: %d, muốn 200", got)
	}
	if got := call(sign("C.prod.khach_a.issuer", "C.prod.khach_a.aud")); got != http.StatusUnauthorized {
		t.Fatalf("SECURITY: token khách A ở khách B: %d, muốn 401", got)
	}
	if got := call(sign("C.prod.khach_b.issuer", "C.prod.khach_a.aud")); got != http.StatusUnauthorized {
		t.Fatalf("audience sai: %d, muốn 401", got)
	}
	if got := call(sign("", "")); got != http.StatusUnauthorized {
		t.Fatalf("token cũ không issuer: %d, muốn 401", got)
	}

	SetJWTVerifier(secret, "", "")
	if got := call(sign("", "")); got != http.StatusUnauthorized {
		t.Fatalf("thiếu issuer cấu hình phải fail-closed: %d, muốn 401", got)
	}
}

package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/logger"
)

func newRenderGuardEngine() *gin.Engine {
	if logger.L == nil {
		logger.Init(logger.Config{Env: "test", AppCode: "mw_render_guard_test"})
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RenderErrorGuard())
	r.GET("/bad", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"note": json.RawMessage("[11, 21, , 14]")})
	})
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"items": []int{1}})
	})
	r.GET("/abort", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "no"})
		_ = c.Error(http.ErrAbortHandler)
	})
	return r
}

func TestRenderErrorGuard_MarshalFailureBecomes500WithBody(t *testing.T) {
	w := httptest.NewRecorder()
	newRenderGuardEngine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("muốn 500, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatal("body không được rỗng")
	}
}

func TestRenderErrorGuard_DoesNotTouchWrittenResponses(t *testing.T) {
	for path, want := range map[string]int{"/ok": http.StatusOK, "/abort": http.StatusForbidden} {
		w := httptest.NewRecorder()
		newRenderGuardEngine().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Fatalf("%s: muốn %d, got %d", path, want, w.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: body phải là 1 JSON hợp lệ (không bị ghi nối), err=%v body=%q", path, err, w.Body.String())
		}
	}
}

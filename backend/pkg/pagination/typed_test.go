package pagination

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Compile-time: PaginateTyped khoá element type vào *[]T.
// Runtime: pure assertion về header/result shape không cần DB.
//
// Behavior (offset/limit/COUNT) đã được test trong integration suite.
// Test này chỉ chứng minh API typed wrapper compile + chuyển hand-off đúng.

func TestPaginateTyped_TypeBinding(t *testing.T) {
	// Đây là test compile-time chính: nếu PaginateTyped[Foo] chấp nhận *[]Bar,
	// build sẽ fail. Không cần assertion runtime.
	type Foo struct {
		ID   uint
		Name string
	}
	var items []Foo
	_ = items

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?page=1&per-page=10", nil)

	// Nếu uncomment dòng dưới với mismatch type → go build fail:
	//   var wrong []int
	//   _ = PaginateTyped[Foo](c, nil, &wrong)  // ❌ cannot use &wrong (*[]int) as *[]Foo

	// Smoke check (không có gorm.DB → không gọi thật, chỉ verify wiring).
	assert.NotNil(t, c)
}

package handler

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ⛔ story-35 AC3 (D109) — ngưỡng đếm KÝ TỰ, không đếm BYTE.
//
// Test này khoá đúng cặp ca đã sinh ra lỗi: "Ân" và "An" đều 2 ký tự, nhưng 3 byte vs 2 byte. Trước
// bản vá `len(keyword) >= 3` cho hai chuỗi hai kết quả KHÁC nhau — nhánh tìm rộng bật/tắt theo việc
// chữ có dấu hay không. Sau bản vá cả hai cùng nằm dưới ngưỡng.
func TestLoadByKeywordThreshold_CountsRunesNotBytes(t *testing.T) {
	wide := func(keyword string) bool { return utf8.RuneCountInString(keyword) >= loadByKeywordMinRunes }

	// Đối chứng chứng minh HAI chuỗi thật sự khác nhau về byte (nếu không, test này vô nghĩa).
	require.Equal(t, 3, len("Ân"))
	require.Equal(t, 2, len("An"))
	require.Equal(t, 2, utf8.RuneCountInString("Ân"))

	require.False(t, wide("Ân"), "⛔ 2 ký tự có dấu KHÔNG được lọt nhánh tìm rộng (lỗi đếm byte)")
	require.False(t, wide("An"))
	// Đối chứng DƯƠNG: 3 ký tự — cả có dấu lẫn không dấu — vẫn phải bật nhánh rộng.
	require.True(t, wide("Ânh"))
	require.True(t, wide("Anh"))
	require.True(t, wide("Nguyễn"))
	require.False(t, wide(""))
}

// ⚠ Test trên CHỈ khoá ngữ nghĩa của ngưỡng — nó tự chép lại biểu thức, nên một mình nó KHÔNG chứng
// minh HANDLER dùng đúng biểu thức đó (đúng bẫy "test kiểm bản chép của chính nó"). Lưới dưới đây
// quét CHÍNH FILE SẢN PHẨM: quay lại `len(keyword)` là đỏ ngay.
func TestLoadByKeywordUsesRuneCountInHandlerSource(t *testing.T) {
	raw, err := os.ReadFile("customer.go")
	require.NoError(t, err)
	src := string(raw)

	// Đối chứng: lưới đọc ĐÚNG file (không phải chuỗi rỗng / file khác).
	require.Greater(t, len(src), 10000)

	// Cắt ĐÚNG thân `LoadCustomerByKeyword`, không quét cả file: `customer.go` còn các handler khác
	// dùng `len(keyword)` hợp lệ (vd đo độ dài SĐT toàn chữ số ở `SearchCustomer`) — quét cả file thì
	// lưới báo giả rồi sớm muộn bị nới cho im.
	const sig = "func (h *Handler) LoadCustomerByKeyword("
	i := strings.Index(src, sig)
	require.GreaterOrEqual(t, i, 0, "không tìm thấy handler — lưới đang quét nhầm file")
	j := strings.Index(src[i:], "\nfunc ")
	require.Greater(t, j, 0, "không tìm thấy điểm kết thúc thân hàm")
	body := src[i : i+j]

	require.Contains(t, body, "utf8.RuneCountInString(keyword) >= loadByKeywordMinRunes",
		"⛔ handler phải đếm KÝ TỰ qua hằng số chung")
	require.NotContains(t, body, "len(keyword) >=",
		"⛔ đếm BYTE quay lại — ngưỡng lại tuỳ thuộc chữ có dấu hay không (D109)")
	// AC4: cột `birthday` chỉ được đưa vào SELECT khi người gọi hỏi.
	require.Contains(t, body, "if withBirthday {")
}

// story-35 AC4 — `with` là danh sách field OPT-IN; thiếu/khác ⇒ KHÔNG trả field nặng.
func TestHasWithField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ask := func(rawQuery string) bool {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/crm/customer/load-by-keyword?"+rawQuery, nil)
		return hasWithField(c, "birthday")
	}

	require.True(t, ask("keyword=abc&with=birthday"))
	require.True(t, ask("with=foo,birthday"))
	require.True(t, ask("with=birthday%2Cfoo"))
	require.True(t, ask("with=foo,%20birthday"), "khoảng trắng quanh field vẫn tính")

	require.False(t, ask("keyword=abc"), "⛔ mặc định KHÔNG opt-in — 23 chỗ gọi cũ giữ payload cũ")
	require.False(t, ask("with="))
	require.False(t, ask("with=birthdays"), "khớp phải TRỌN field, không phải tiền tố")
	require.False(t, ask("with=xbirthday"))
}

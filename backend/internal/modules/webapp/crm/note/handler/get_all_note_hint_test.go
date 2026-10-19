package handler

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Neo quyết định index hint của GET /crm/note/get-all-note (2026-09-09).
// Chỉ hint khi CÓ từ khoá VÀ cửa sổ ngày ≤ 31 ngày — số đo đầy đủ nằm ở doc của
// `maybeHintDateIndex`. Hint sai chỗ không làm sai KẾT QUẢ (đã đối chiếu 11/11 ca khớp) nhưng
// làm chậm: cửa sổ 1 năm + từ khoá phổ biến bị phạt 1,3ms -> 680ms.
func TestMaybeHintDateIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		d1  = int64(86400)
		t0  = int64(1788195600) // 2026-09-01 00:00 +07
		d31 = 31 * d1
	)

	cases := []struct {
		ten  string
		qs   string
		muon bool
	}{
		{"không từ khoá, có ngày", "filter[created_at][gte]=1788195600&filter[created_at][lte]=1788973199", false},
		{"có từ khoá, KHÔNG ngày", "_keyword=abc", false},
		{"từ khoá + cửa sổ 1 ngày", kwWindow(t0, t0+d1), true},
		{"từ khoá + cửa sổ đúng 31 ngày", kwWindow(t0, t0+d31), true},
		{"từ khoá + cửa sổ 32 ngày (quá trần)", kwWindow(t0, t0+d31+d1), false},
		{"từ khoá + cửa sổ 1 năm", kwWindow(t0, t0+366*d1), false},
		{"từ khoá + ngày đảo đầu-cuối", kwWindow(t0+d1, t0), false},
		{"từ khoá + ngày rác", "_keyword=abc&filter[created_at][gte]=abc&filter[created_at][lte]=def", false},
		// format thứ 2 mà ApplyFilter chấp nhận — tự ghép chuỗi tay sẽ trượt đúng nhánh này
		{"từ khoá + format `created_at[gte]`", "_keyword=abc&created_at[gte]=1788195600&created_at[lte]=1788282000", true},
	}

	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/crm/note/get-all-note?"+tc.qs, nil)
			hasKeyword := c.Query("_keyword") != ""
			if got := maybeHintDateIndex(c, hasKeyword); got != tc.muon {
				t.Fatalf("maybeHintDateIndex = %v, muốn %v (query: %s)", got, tc.muon, tc.qs)
			}
		})
	}
}

func kwWindow(from, to int64) string {
	return "_keyword=abc" +
		"&filter[created_at][gte]=" + itoa64(from) +
		"&filter[created_at][lte]=" + itoa64(to)
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// Tên chỉ mục nằm TRONG SQL. Đổi tên chỉ mục trong migration mà quên sửa hằng số ⇒ MySQL trả lỗi
// 1176 lúc chạy (lớn tiếng, không hỏng im lặng) — nhưng bắt ngay ở đây thì rẻ hơn nhiều.
func TestNoteDateIndexHintKhopVoiMigration(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("đọc thư mục migrations (%s): %v", dir, err)
	}
	found := false
	scanned := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		scanned++
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("đọc %s: %v", e.Name(), err)
		}
		if strings.Contains(string(b), noteDateIndexHint) {
			found = true
		}
	}
	// Đối chứng: nếu không quét được file nào thì kết luận "không tìm thấy" là vô nghĩa.
	if scanned < 10 {
		t.Fatalf("chỉ quét được %d file migration — đường dẫn sai, phép kiểm vô nghĩa", scanned)
	}
	if !found {
		t.Fatalf("không migration nào tạo chỉ mục %q mà code đang hint tới ⇒ endpoint sẽ lỗi 1176", noteDateIndexHint)
	}
}

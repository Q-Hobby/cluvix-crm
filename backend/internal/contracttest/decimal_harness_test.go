//go:build integration

package contracttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// DecimalCase mô tả 1 endpoint + danh sách field PHẢI ra JSON number.
// Story-04..10 chỉ cần APPEND case vào bảng `decimalCases` (decimal_cases_test.go),
// KHÔNG viết lại test.
type DecimalCase struct {
	// Name — tên sub-test (t.Run).
	Name string
	// Endpoint — dạng "GET /crm/customer/:id". Chỉ để đọc/trace, harness không parse.
	Endpoint string
	// Handler — "webapp/crm/customer/handler.GetCustomer". Chỉ để trace khi test đỏ.
	Handler string
	// Skip — != "" ⇒ t.Skip(Skip). Dùng khi case tạm chưa dựng được seed.
	Skip string
	// Call — seed dữ liệu cần thiết rồi gọi thẳng handler qua httptest.
	// KHÔNG dựng server thật (Hard Rule: không bind port 8081).
	// Giá trị DECIMAL seed BẮT BUỘC khác NULL và nên khác 0 để phân biệt được số thật.
	Call func(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder
	// WantStatus — HTTP status kỳ vọng (0 ⇒ 200).
	WantStatus int
	// NumberFields — JSON path tới field DECIMAL, tính từ gốc response body.
	// Cú pháp: segment ngăn bởi '.', segment số = index mảng, '*' = mọi phần tử mảng.
	//   "data.totalRevenue"          → object
	//   "data.items.*.total_amount"  → mọi phần tử mảng
	//   "data.rows.0.amount"         → phần tử đầu
	NumberFields []string
}

// runDecimalCase — lõi harness: gọi endpoint, decode bằng UseNumber, khẳng định từng field
// là json.Number.
//
// AC2: decode bằng json.Decoder + UseNumber() vào `interface{}`, TUYỆT ĐỐI không unmarshal
// vào struct typed — `json:"x"` trên field float64 sẽ tự parse cả `"640000000.00"` lẫn
// `640000000` thành 640000000, tức là struct typed che mất CHÍNH lỗi cần bắt.
func runDecimalCase(t *testing.T, db *gorm.DB, tc DecimalCase) {
	t.Helper()

	if tc.Skip != "" {
		t.Skip(tc.Skip)
	}
	if tc.Call == nil {
		t.Fatalf("case %q: thiếu Call", tc.Name)
	}
	if len(tc.NumberFields) == 0 {
		t.Fatalf("case %q: NumberFields rỗng — case không khẳng định gì", tc.Name)
	}

	w := tc.Call(t, db)

	want := tc.WantStatus
	if want == 0 {
		want = 200
	}
	if w.Code != want {
		t.Fatalf("%s (%s): HTTP %d, muốn %d\nbody: %s", tc.Endpoint, tc.Handler, w.Code, want, w.Body.String())
	}

	var root interface{}
	dec := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		t.Fatalf("%s: decode response: %v\nbody: %s", tc.Endpoint, err, w.Body.String())
	}

	var failures []string
	for _, path := range tc.NumberFields {
		vals, err := resolvePath(root, path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("  %-28s KHÔNG TÌM THẤY trong response (%v)", path, err))
			continue
		}
		if len(vals) == 0 {
			failures = append(failures, fmt.Sprintf("  %-28s path khớp 0 giá trị (mảng rỗng?) — không chứng minh được gì", path))
			continue
		}
		for i, v := range vals {
			label := path
			if len(vals) > 1 {
				label = fmt.Sprintf("%s[#%d]", path, i)
			}
			switch tv := v.(type) {
			case json.Number:
				// ✅ đúng contract
			case string:
				failures = append(failures, fmt.Sprintf("  %-28s là JSON string %q — phải là JSON number", label, tv))
			case nil:
				failures = append(failures, fmt.Sprintf("  %-28s là null — seed phải đặt giá trị DECIMAL khác NULL, nếu không case không chứng minh được gì", label))
			case bool:
				failures = append(failures, fmt.Sprintf("  %-28s là bool %v — phải là JSON number", label, tv))
			default:
				failures = append(failures, fmt.Sprintf("  %-28s là %T (%v) — phải là JSON number", label, v, v))
			}
		}
	}

	if len(failures) > 0 {
		t.Errorf("%s → %s\nfield DECIMAL KHÔNG ra JSON number (%d/%d lỗi):\n%s\nraw body: %s",
			tc.Endpoint, tc.Handler, len(failures), len(tc.NumberFields), strings.Join(failures, "\n"), w.Body.String())
	}
}

// resolvePath đi theo JSON path và trả về TẤT CẢ giá trị khớp (nhiều giá trị khi có '*').
// Trả lỗi khi một segment không tồn tại — field gõ sai phải làm test ĐỎ, không được im lặng pass.
func resolvePath(root interface{}, path string) ([]interface{}, error) {
	cur := []interface{}{root}
	for _, seg := range strings.Split(path, ".") {
		var next []interface{}
		for _, node := range cur {
			switch n := node.(type) {
			case map[string]interface{}:
				v, ok := n[seg]
				if !ok {
					return nil, fmt.Errorf("thiếu key %q (các key có: %s)", seg, keysPreview(n))
				}
				next = append(next, v)
			case []interface{}:
				if seg == "*" {
					next = append(next, n...)
					continue
				}
				idx, err := strconv.Atoi(seg)
				if err != nil {
					return nil, fmt.Errorf("segment %q không phải index/'*' nhưng node là mảng", seg)
				}
				if idx < 0 || idx >= len(n) {
					return nil, fmt.Errorf("index %d ngoài phạm vi (len=%d)", idx, len(n))
				}
				next = append(next, n[idx])
			default:
				return nil, fmt.Errorf("không đi tiếp được vào %q: node là %T", seg, node)
			}
		}
		cur = next
	}
	return cur, nil
}

// keysPreview — trợ giúp debug khi path sai: liệt kê tối đa 25 key của object.
func keysPreview(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if len(keys) > 25 {
		keys = append(keys[:25], "…")
	}
	return strings.Join(keys, ", ")
}

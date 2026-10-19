package userscope

import "testing"

func strPtr(s string) *string { return &s }

// TestParseIDs — 4 ca của AC3, chạy KHÔNG cần DB: NULL/chuỗi rỗng/"null"/"[]" ⇒ mảng rỗng; JSON
// hỏng ⇒ error. Cột JSON thật của MySQL không thể chứa chuỗi rỗng (đã đo bằng tay: gán chuỗi rỗng
// cho cột JSON bị MySQL từ chối "Invalid JSON text: document is empty") nên ca chuỗi rỗng chỉ tái
// hiện được ở tầng parse này.
func TestParseIDs(t *testing.T) {
	cases := []struct {
		name    string
		raw     *string
		want    []uint
		wantErr bool
	}{
		{name: "sql_null", raw: nil, want: nil},
		{name: "empty_string", raw: strPtr(""), want: nil},
		{name: "json_null_literal", raw: strPtr("null"), want: nil},
		{name: "empty_array", raw: strPtr("[]"), want: []uint{}},
		{name: "valid_array", raw: strPtr("[3,5]"), want: []uint{3, 5}},
		{name: "invalid_json_scalar_string", raw: strPtr(`"not-an-array"`), wantErr: true},
		{name: "invalid_json_object", raw: strPtr(`{}`), wantErr: true},
		{name: "invalid_json_syntax", raw: strPtr(`not-json`), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIDs(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseIDs(%v) = %v, nil — muốn có error", derefOrNil(tc.raw), got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseIDs(%v) trả error không mong muốn: %v", derefOrNil(tc.raw), err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseIDs(%v) = %v, muốn %v", derefOrNil(tc.raw), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseIDs(%v) = %v, muốn %v", derefOrNil(tc.raw), got, tc.want)
				}
			}
		})
	}
}

func derefOrNil(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestAppendID_RejectsColumnOutsideWhitelist(t *testing.T) {
	// Không cần DB: whitelist reject xảy ra TRƯỚC bất kỳ query nào ⇒ truyền db=nil an toàn.
	err := AppendID(nil, 1, "password_hash", 1)
	if err == nil {
		t.Fatal("AppendID với cột ngoài whitelist phải trả error, không được chạm DB")
	}
}

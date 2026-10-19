package export

import "testing"

func TestHeaderRows(t *testing.T) {
	t.Run("không có FieldKeys → 1 dòng header", func(t *testing.T) {
		got := headerRows(Config{Headers: []string{"A", "B"}})
		if len(got) != 1 || got[0][0] != "A" {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("có FieldKeys → 2 dòng, key ở dòng 2", func(t *testing.T) {
		got := headerRows(Config{Headers: []string{"Tên", "Mã"}, FieldKeys: []string{"name", "code"}})
		if len(got) != 2 {
			t.Fatalf("got %d dòng, muốn 2", len(got))
		}
		if got[1][0] != "name" || got[1][1] != "code" {
			t.Fatalf("dòng field key sai: %v", got[1])
		}
	})
}

// FieldKeys lệch độ dài Headers = dòng key không khớp cột dữ liệu → importer ghi giá trị cột A vào
// cột B. Phải chặn tại cửa, không sinh ra file rồi mới hỏng.
func TestValidateConfigFieldKeysLength(t *testing.T) {
	if err := validateConfig(Config{Headers: []string{"A", "B"}, FieldKeys: []string{"a"}}); err == nil {
		t.Fatal("muốn lỗi khi FieldKeys ngắn hơn Headers")
	}
	if err := validateConfig(Config{Headers: []string{"A", "B"}, FieldKeys: []string{"a", "b"}}); err != nil {
		t.Fatalf("độ dài khớp mà báo lỗi: %v", err)
	}
	if err := validateConfig(Config{Headers: []string{"A", "B"}}); err != nil {
		t.Fatalf("không dùng FieldKeys mà báo lỗi: %v", err)
	}
}

// Unsanitize phải đảo đúng sanitize và chỉ đảo đúng phần sanitize gây ra.
func TestSanitizeUnsanitizeRoundTrip(t *testing.T) {
	for _, in := range []string{"- Combo cấy tóc", "=A1", "+84", "@here", "Cấy tóc FUE", "", "0"} {
		out := sanitize(in)
		s, ok := out.(string)
		if !ok {
			t.Fatalf("sanitize(%q) không trả string: %T", in, out)
		}
		if got := Unsanitize(s); got != in {
			t.Errorf("round-trip %q → %q → %q", in, s, got)
		}
	}
}

func TestUnsanitizeKeepsLegitimateApostrophe(t *testing.T) {
	for _, in := range []string{"'Combo", "'", "l'Oreal"} {
		if got := Unsanitize(in); got != in {
			t.Errorf("Unsanitize(%q) = %q — nháy đơn không do sanitize sinh ra phải giữ nguyên", in, got)
		}
	}
}

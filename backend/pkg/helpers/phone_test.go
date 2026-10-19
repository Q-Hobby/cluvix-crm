package helpers_test

import (
	"testing"

	"erpCluvix/pkg/helpers"
)

// Mọi định dạng của CÙNG một số phải sinh ra tập biến thể phủ được nhau —
// đây là điều kiện để dò trùng khách hàng không sinh bản ghi mới.
func TestPhoneVariants_AllFormatsMatchEachOther(t *testing.T) {
	inputs := []string{"0906904884", "84906904884", "+84906904884", "906904884"}
	stored := []string{"0906904884", "84906904884", "+84906904884", "906904884"}

	for _, in := range inputs {
		variants := helpers.PhoneVariants(in, "84")
		set := map[string]bool{}
		for _, v := range variants {
			set[v] = true
		}
		for _, s := range stored {
			if !set[s] {
				t.Errorf("input %q: variants %v không phủ được số đã lưu %q", in, variants, s)
			}
		}
	}
}

func TestPhoneVariants_EdgeCases(t *testing.T) {
	if got := helpers.PhoneVariants("", "84"); len(got) != 1 || got[0] != "" {
		t.Errorf("chuỗi rỗng: got %v", got)
	}
	// Số có dấu cách/chấm vẫn phải normalize được.
	got := helpers.PhoneVariants("0906.904 884", "84")
	set := map[string]bool{}
	for _, v := range got {
		set[v] = true
	}
	if !set["84906904884"] || !set["0906904884"] {
		t.Errorf("số có ký tự phân cách: got %v", got)
	}
	// telephoneCode rỗng không được panic / cắt chuỗi sai.
	if got := helpers.PhoneVariants("0906904884", ""); len(got) == 0 {
		t.Errorf("telephoneCode rỗng: got %v", got)
	}
	// Chuỗi đúng bằng mã vùng — không được sinh biến thể rỗng.
	for _, v := range helpers.PhoneVariants("84", "84") {
		if v == "" {
			t.Errorf("sinh biến thể rỗng từ %q: %v", "84", helpers.PhoneVariants("84", "84"))
		}
	}
}

func TestPhoneLast9(t *testing.T) {
	cases := map[string]string{
		"0906904884":   "906904884",
		"84906904884":  "906904884",
		"+84906904884": "906904884",
		"906904884":    "", // 9 chữ số — quá ngắn, không dùng làm khoá dò trùng
		"123":          "",
		"":             "",
	}
	for in, want := range cases {
		if got := helpers.PhoneLast9(in); got != want {
			t.Errorf("PhoneLast9(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPhoneMatchCond(t *testing.T) {
	cond, args := helpers.PhoneMatchCond("phone", "84906904884", "84")
	if cond != "(phone IN ? OR phone LIKE ?)" {
		t.Errorf("cond = %q", cond)
	}
	if len(args) != 2 || args[1] != "%906904884" {
		t.Errorf("args = %v", args)
	}

	// Số ngắn: không có nhánh LIKE (tránh khớp nhầm hàng loạt).
	cond, args = helpers.PhoneMatchCond("phone", "906904884", "84")
	if cond != "(phone IN ?)" || len(args) != 1 {
		t.Errorf("số ngắn: cond = %q args = %v", cond, args)
	}
}

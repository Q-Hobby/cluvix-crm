package codegen

import (
	"testing"

	"erpCluvix/pkg/define"
)

// Quy tắc toàn dự án: khách chưa có chi nhánh ⇒ chưa có mã. Nhánh này phải trả "" TRƯỚC khi chạm
// DB (tx = nil ở đây — chạm vào là panic), nên không cần DB test.
func TestCustomerCodeRequiresBranch(t *testing.T) {
	for _, branchID := range []int64{0, -1} {
		if got := NewCustomerCode(nil, branchID); got != "" {
			t.Errorf("NewCustomerCode(branch=%d) = %q, muốn \"\"", branchID, got)
		}
		if got := EnsureCustomerCode(nil, 1, branchID); got != "" {
			t.Errorf("EnsureCustomerCode(branch=%d) = %q, muốn \"\"", branchID, got)
		}
	}
	if got := EnsureCustomerCode(nil, 0, 1); got != "" {
		t.Errorf("EnsureCustomerCode(customer=0) = %q, muốn \"\"", got)
	}
}

func TestCustomerCodeOptionFromEnv(t *testing.T) {
	cases := map[string]int{
		"":    define.CustomerCodeOptionDefault,
		"1":   define.CustomerCodeOptionDefault,
		" 4 ": define.CustomerCodeOptionBranchID,
		"6":   define.CustomerCodeOptionBrandInfixMonth,
		"0":   define.CustomerCodeOptionDefault, // không có option 0 ⇒ mặc định
		"99":  define.CustomerCodeOptionDefault,
		"abc": define.CustomerCodeOptionDefault,
	}
	for raw, want := range cases {
		t.Setenv(define.CustomerCodeOptionEnv, raw)
		if got := customerCodeOption(); got != want {
			t.Errorf("%s=%q ⇒ option %d, muốn %d", define.CustomerCodeOptionEnv, raw, got, want)
		}
	}
}

// joinCode là phần ghép chung — option 1 (mặc định) phải ra ĐÚNG định dạng như trước khi tách hàm.
func TestJoinCode(t *testing.T) {
	cases := []struct {
		parts []string
		ym    string
		n     int64
		width int
		want  string
	}{
		{[]string{"clx", "Q1", "KH"}, "2609", 1, 2, "CLX.Q1.KH.2609.01"},
		{[]string{"CLX", "", "KH"}, "", 7, 3, "CLX.KH.007"},
		{nil, "2609", 123, 2, "2609.123"}, // tự nở khi vượt độ dài
		{[]string{"3"}, "", 1, 5, "3.00001"},
		{nil, "", 1, 0, "01"}, // tối thiểu 2
	}
	for _, c := range cases {
		if got := joinCode(c.parts, c.ym, c.n, c.width); got != c.want {
			t.Errorf("joinCode(%v, %q, %d, %d) = %q, muốn %q", c.parts, c.ym, c.n, c.width, got, c.want)
		}
	}
}

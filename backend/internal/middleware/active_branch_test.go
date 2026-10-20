package middleware

import "testing"

// TestEffectiveActiveBranch — branchActive_id chỉ được tin khi còn thuộc branchIds; lệch (user vừa bị
// gỡ khỏi chi nhánh) ⇒ về chi nhánh đầu tiên được phép. branchIds rỗng = không giới hạn ⇒ giữ nguyên.
func TestEffectiveActiveBranch(t *testing.T) {
	cases := []struct {
		name    string
		active  uint
		allowed []uint
		want    uint
	}{
		{"còn thuộc phạm vi", 2, []uint{1, 2}, 2},
		{"bị gỡ khỏi chi nhánh", 3, []uint{1, 2}, 1},
		{"chưa chọn chi nhánh", 0, []uint{1, 2}, 0},
		{"không giới hạn chi nhánh", 3, nil, 3},
	}
	for _, tc := range cases {
		if got := effectiveActiveBranch(tc.active, tc.allowed); got != tc.want {
			t.Errorf("%s: effectiveActiveBranch(%d, %v) = %d, muốn %d", tc.name, tc.active, tc.allowed, got, tc.want)
		}
	}
}

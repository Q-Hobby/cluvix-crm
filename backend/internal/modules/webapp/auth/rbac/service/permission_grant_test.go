package service

import (
	"errors"
	"strings"
	"testing"
)

// Unit test (thuần, không DB) cho nhánh "quyền lạ" của UpdatePermissionWithRole — xem
// mobile/plan/42: trước đây quyền không có trong catalog bị `continue` bỏ qua im lặng rồi vẫn
// commit + trả nil ⇒ FE báo lưu thành công, DB không có gì.

func TestDedupStrings(t *testing.T) {
	got := dedupStrings([]string{"b", "a", "b", "", "a", "c"})
	want := []string{"b", "a", "c"} // giữ thứ tự xuất hiện đầu tiên, bỏ rỗng + trùng
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dedupStrings = %v, kỳ vọng %v", got, want)
	}
	if got := dedupStrings(nil); len(got) != 0 {
		t.Fatalf("nil phải ra rỗng, được %v", got)
	}
}

func TestDiffUnknown(t *testing.T) {
	// Tất cả hợp lệ → không có quyền lạ.
	if got := diffUnknown([]string{"viewUser", "createUser"}, []string{"createUser", "viewUser"}); len(got) != 0 {
		t.Fatalf("kỳ vọng rỗng, được %v", got)
	}

	// Thiếu trong catalog → báo đúng tên, đúng thứ tự đầu vào.
	got := diffUnknown(
		[]string{"viewUser", "viewHrReport", "createUser", "viewCareReport"},
		[]string{"viewUser", "createUser"},
	)
	want := []string{"viewHrReport", "viewCareReport"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("diffUnknown = %v, kỳ vọng %v", got, want)
	}

	// Catalog rỗng (artifact chưa seed) → toàn bộ là lạ, KHÔNG được coi là hợp lệ.
	if got := diffUnknown([]string{"a", "b"}, nil); len(got) != 2 {
		t.Fatalf("catalog rỗng phải trả đủ 2 quyền lạ, được %v", got)
	}
}

// Handler dùng errors.As để map sang 422 — kiểu lỗi phải bắt được qua con trỏ và giữ nguyên Names.
func TestErrUnknownPermissionsIsTyped(t *testing.T) {
	var err error = &ErrUnknownPermissions{Names: []string{"viewHrReport", "viewCareReport"}}

	var target *ErrUnknownPermissions
	if !errors.As(err, &target) {
		t.Fatal("errors.As phải bắt được *ErrUnknownPermissions — handler dựa vào đây để trả 422 thay vì 500")
	}
	if len(target.Names) != 2 || target.Names[0] != "viewHrReport" {
		t.Fatalf("Names bị mất/đổi: %v", target.Names)
	}
	if !strings.Contains(err.Error(), "viewCareReport") {
		t.Fatalf("Error() phải liệt kê tên quyền, đang là %q", err.Error())
	}
}

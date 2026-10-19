package seed

import (
	"encoding/json"
	"testing"
)

// Verify artifact embed parse được về []{name, description} — contract cho SeedPermissions.
// (Hành vi seed/prune trên DB thật: story-07 e2e.)
func TestEmbeddedPermissionsParse(t *testing.T) {
	if len(PermissionsJSON) == 0 {
		t.Fatal("PermissionsJSON embed rỗng")
	}
	var items []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(PermissionsJSON, &items); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}
	// story-16 (be-rbac-trim, 2026-09-24): trim catalog 468 → 55 quyền (chỉ giữ quyền còn route thật
	// mount + quyền dùng trực tiếp trong guard handler, xem Log story-16 danh sách đầy đủ).
	// 2026-10-19: bỏ thêm 6 quyền thừa (5 quyền in phiếu gộp vào viewPayment + configDevelop) ⇒ 46.
	// Ngưỡng dưới chỉ để bắt artifact hỏng/rỗng, không phải con số chính xác.
	if len(items) < 40 {
		t.Fatalf("kỳ vọng ~46 quyền, chỉ có %d — artifact hỏng?", len(items))
	}
	seen := map[string]bool{}
	for i, it := range items {
		if it.Name == "" {
			t.Fatalf("item %d có name rỗng", i)
		}
		if seen[it.Name] {
			t.Fatalf("name trùng trong artifact: %s", it.Name)
		}
		seen[it.Name] = true
	}
}

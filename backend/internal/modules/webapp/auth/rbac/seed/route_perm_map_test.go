package seed

import "testing"

// Verify artifact route_perm_map.json embed parse được và giữ invariant middleware dựa vào:
// key (method+path) duy nhất, kind hợp lệ, entry perm/role có perm, role đúng prefix.
func TestRoutePermMapParse(t *testing.T) {
	entries, err := LoadRoutePermMap()
	if err != nil {
		t.Fatalf("LoadRoutePermMap: %v", err)
	}
	// story-16 (be-rbac-trim, 2026-09-24): trim 1974 → 403 entry (xoá mọi entry KHÔNG khớp route
	// đang mount thật — cross-check bằng permission_coverage_test.go realRoutes(), 0 drift sau trim).
	if len(entries) < 350 {
		t.Fatalf("kỳ vọng ~403 entry (sau story-16 trim), chỉ có %d", len(entries))
	}
	seen := map[string]bool{}
	valid := map[string]bool{KindPerm: true, KindRole: true, KindPublic: true, KindUnmapped: true}
	for i, e := range entries {
		if e.Method == "" || e.Path == "" {
			t.Fatalf("entry %d thiếu method/path", i)
		}
		key := e.Method + " " + e.Path
		if seen[key] {
			t.Fatalf("key trùng: %s", key)
		}
		seen[key] = true
		if !valid[e.Kind] {
			t.Fatalf("entry %d kind không hợp lệ: %q", i, e.Kind)
		}
		switch e.Kind {
		case KindPerm:
			if e.Perm == "" {
				t.Fatalf("entry %d kind=perm nhưng perm rỗng: %s", i, key)
			}
		case KindRole:
			if len(e.Perm) <= len(rolePrefixTest) || e.Perm[:len(rolePrefixTest)] != rolePrefixTest {
				t.Fatalf("entry %d kind=role perm phải có prefix %q: %q", i, rolePrefixTest, e.Perm)
			}
		default:
			// Chiều ngược (public/unmapped ⇒ perm PHẢI rỗng). Thiếu invariant này thì entry đã gỡ
			// gate mà quên xoá perm (vd {"kind":"public","perm":"viewClsResult"}) trở thành tham
			// chiếu chết: đọc map tưởng còn gate, mà TestRoutePermMapPermsInCatalog cũng bỏ qua vì
			// nó chỉ soi kind=perm. Đổi kind = phải xoá perm cùng lúc.
			if e.Perm != "" {
				t.Fatalf("entry %d kind=%s phải có perm rỗng, đang là %q: %s", i, e.Kind, e.Perm, key)
			}
		}
	}
}

const rolePrefixTest = "role:"

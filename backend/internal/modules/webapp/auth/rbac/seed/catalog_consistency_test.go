package seed

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestRoutePermMapPermsInCatalog — FAIL CỨNG nếu route_perm_map.json tham chiếu 1 quyền KHÔNG có
// trong catalog artifact permissions.gen.json.
//
// Vì sao cần (sự cố mobile/plan/41 MOB-109): route_perm_map.json KHÔNG tạo permission. Nguồn duy
// nhất sinh row `rbac_auth_item` là artifact permissions.gen.json (codegen từ FE
// permission.define.ts, seed lúc boot — internal/bootstrap/app.go seedPermissions). Thêm entry vào
// map mà quên khai key ở FE ⇒ quyền không tồn tại ⇒ KHÔNG ai giữ được, kể cả super role
// (`GetUserPermissions` với super = "mọi row rbac_auth_item type=2"). Hệ quả im lặng: màn mới bị
// khoá với 100% user; và khi bật PERM_ENFORCE=true thì mọi route enforce_phase1 dùng key ma sẽ 403
// toàn bộ user thường. TestRoutePermMapCoverage (internal/router) chỉ soi route↔map nên mù lớp này.
//
// Test sống ở package seed — cạnh 2 artifact nó đối chiếu, KHÔNG cần dựng router. Đặt chung với
// TestRoutePermMapCoverage thì gate tắt im lặng mỗi khi Setup()/prefix hỏng làm test kia Fatal sớm.
//
// Bỏ qua: kind != perm (role:<name> là role trong DB, không thuộc catalog quyền). Entry perm dạng
// CSV "a,b" (OR-gate) tách từng phần — khớp middleware.splitPerms.
func TestRoutePermMapPermsInCatalog(t *testing.T) {
	known := catalogNames(t)
	entries, err := LoadRoutePermMap()
	if err != nil {
		t.Fatalf("LoadRoutePermMap: %v", err)
	}

	// perm ma → danh sách route dùng nó (route đầu tiên đủ để lần ra chỗ khai thiếu).
	orphans := make(map[string][]string)
	for _, e := range entries {
		if e.Kind != KindPerm {
			continue
		}
		for _, p := range strings.Split(e.Perm, ",") {
			if p = strings.TrimSpace(p); p == "" || known[p] {
				continue
			}
			orphans[p] = append(orphans[p], e.Method+" "+e.Path)
		}
	}
	if len(orphans) == 0 {
		t.Logf("CATALOG: %d quyền, mọi perm trong route_perm_map đều tồn tại", len(known))
		return
	}

	names := make([]string, 0, len(orphans))
	for p := range orphans {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		t.Errorf("perm %q có trong route_perm_map.json nhưng KHÔNG có trong permissions.gen.json (%d route, vd %s)\n"+
			"    → khai key ở frontend/src/app/define/permission.define.ts rồi chạy `node scripts/perm-codegen.mjs`,\n"+
			"      HOẶC đổi entry sang kind=public nếu route chủ đích không gate quyền.",
			p, len(orphans[p]), orphans[p][0])
	}
}

// catalogNames đọc set tên quyền từ artifact embed. Fatal nếu artifact hỏng/rỗng — không để test
// "xanh vì không có gì để so".
func catalogNames(t *testing.T) map[string]bool {
	t.Helper()
	var catalog []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(PermissionsJSON, &catalog); err != nil {
		t.Fatalf("parse permissions.gen.json: %v", err)
	}
	known := make(map[string]bool, len(catalog))
	for _, p := range catalog {
		known[p.Name] = true
	}
	if len(known) == 0 {
		t.Fatalf("catalog permissions.gen.json rỗng — nghi artifact hỏng")
	}
	return known
}

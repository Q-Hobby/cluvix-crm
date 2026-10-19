package seed

import (
	"encoding/json"
	"fmt"
)

// RoutePermEntry là 1 dòng của route_perm_map.json — ánh xạ 1 endpoint BE
// (method + route pattern có prefix /api, param dạng :id) sang quyền cần có.
// Sinh bởi BMAD permission-enforcement P0.2, dùng bởi middleware.PermissionEnforcer.
type RoutePermEntry struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	Module        string `json:"module"`
	Perm          string `json:"perm"` // kind=perm: tên quyền; kind=role: "role:<name>"; khác: ""
	Kind          string `json:"kind"` // perm | role | public | unmapped
	EnforcePhase1 bool   `json:"enforce_phase1"`
	Note          string `json:"note"`
}

// Kind constants cho RoutePermEntry.
const (
	KindPerm     = "perm"
	KindRole     = "role"
	KindPublic   = "public"
	KindUnmapped = "unmapped"
)

// LoadRoutePermMap parse artifact embed route_perm_map.json về slice entry.
// Gọi 1 lần lúc khởi tạo middleware — KHÔNG parse mỗi request.
func LoadRoutePermMap() ([]RoutePermEntry, error) {
	var entries []RoutePermEntry
	if err := json.Unmarshal(RoutePermMapJSON, &entries); err != nil {
		return nil, fmt.Errorf("parse route_perm_map.json: %w", err)
	}
	return entries, nil
}

package router

import (
	"sync"
	"testing"

	"erpCluvix/config"
	"erpCluvix/internal/deps"
	"erpCluvix/internal/modules/webapp/auth/rbac/seed"
	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
)

// mutatingMethods — giai đoạn P1 chỉ enforce mutating (GET để mở).
var mutatingMethods = map[string]bool{"POST": true, "PUT": true, "PATCH": true, "DELETE": true}

// realRoutesOnce/realRoutesCache — cache engine Setup() 1 lần bằng sync.Once, dùng chung cho mọi
// test trong package. Lý do gốc: Setup() từng đăng ký adapter kênh chat vào registry SINGLETON
// process-global (panic khi trùng Key) — module đó đã xoá ở crm-standalone story-05, cache vẫn giữ
// vì rẻ và KHÔNG đổi ngữ nghĩa (route thật vẫn đọc trực tiếp từ engine Setup() dựng).
var (
	realRoutesOnce  sync.Once
	realRoutesCache map[string]gin.RouteInfo
)

// realRoutes dựng engine thật (zero deps — Register chỉ wire closure, không chạm DB)
// rồi trả set key `METHOD PATH` của mọi route đã đăng ký (path = c.FullPath pattern,
// đúng thứ middleware nhìn thấy lúc runtime).
func realRoutes(t *testing.T) map[string]gin.RouteInfo {
	t.Helper()
	realRoutesOnce.Do(func() {
		// logger.L nil trong test process (chưa qua main.go bootstrap) — Setup() gọi logger.L.Error ở
		// nhánh lỗi (SetTrustedProxies, LoadRoutePermMap…) ⇒ nil pointer panic. Production
		// luôn Init logger lúc boot; ở đây chỉ khởi tạo no-op-ish cho test, KHÔNG đổi code production.
		// logger.Init tự có sync.Once riêng nên gọi lại từ file test khác trong cùng process vẫn an toàn.
		logger.Init(logger.Config{Env: "test"})
		gin.SetMode(gin.TestMode)
		r := Setup(&config.Config{}, deps.Deps{})
		out := make(map[string]gin.RouteInfo)
		for _, ri := range r.Routes() {
			out[ri.Method+" "+ri.Path] = ri
		}
		realRoutesCache = out
	})
	return realRoutesCache
}

// TestRoutePermMapCoverage đối chiếu route_perm_map.json ↔ route thật đã đăng ký.
// Báo 3 số liệu (điểm dễ sai: param name :id vs :userId, trailing slash, prefix lệch):
//   - matched: entry enforce_phase1 khớp đúng 1 route thật.
//   - drift:   entry enforce_phase1 KHÔNG khớp route nào (path lệch — cần sửa map).
//   - missing: route mutating thật KHÔNG có entry nào trong map (thiếu định nghĩa quyền).
func TestRoutePermMapCoverage(t *testing.T) {
	entries, err := seed.LoadRoutePermMap()
	if err != nil {
		t.Fatalf("LoadRoutePermMap: %v", err)
	}
	routes := realRoutes(t)

	// Tập key của MỌI entry trong map (mọi kind) — để tính "route thật không có trong map".
	inMap := make(map[string]bool, len(entries))
	for _, e := range entries {
		inMap[e.Method+" "+e.Path] = true
	}

	var matched, drift []string
	for _, e := range entries {
		if !e.EnforcePhase1 {
			continue
		}
		if e.Kind != seed.KindPerm && e.Kind != seed.KindRole {
			continue
		}
		key := e.Method + " " + e.Path
		if _, ok := routes[key]; ok {
			matched = append(matched, key)
		} else {
			drift = append(drift, key)
		}
	}

	// Route mutating thật, thuộc /api, không có entry nào trong map.
	var missing []string
	for key, ri := range routes {
		if !mutatingMethods[ri.Method] {
			continue
		}
		if len(ri.Path) < 5 || ri.Path[:5] != "/api/" {
			continue
		}
		if !inMap[key] {
			missing = append(missing, key)
		}
	}

	t.Logf("COVERAGE: enforce entries matched=%d drift=%d ; mutating /api routes missing-from-map=%d ; total routes=%d total map entries=%d",
		len(matched), len(drift), len(missing), len(routes), len(entries))
	for _, k := range drift {
		t.Logf("  DRIFT (map entry not matching any real route): %s", k)
	}
	for _, k := range missing {
		t.Logf("  MISSING (real mutating route not in map): %s", k)
	}

	// Không fail cứng ở đây (audit-mode, map đang tiến hóa). Coverage là thông tin để sửa map.
	// Chỉ fail nếu KHÔNG khớp được gì cả (dấu hiệu prefix/param sai toàn cục).
	if len(matched) == 0 {
		t.Fatalf("0 enforce entry khớp route thật — nghi prefix/param lệch toàn cục")
	}
	// Drift = entry enforce trỏ vào route không tồn tại ⇒ route thật (path đã đổi) chạy KHÔNG gate.
	if len(drift) > 0 {
		t.Fatalf("%d entry enforce_phase1 không khớp route thật (drift) — sửa path trong route_perm_map.json", len(drift))
	}

	// Invariant "mọi perm trong map phải tồn tại trong catalog" KHÔNG nằm ở đây: nó độc lập với
	// router (không cần dựng engine) nên sống ở package seed cạnh 2 artifact —
	// TestRoutePermMapPermsInCatalog. Đặt chung test này thì gate sẽ tắt im lặng mỗi khi
	// Setup()/prefix hỏng làm t.Fatalf phía trên bắn trước.
}

// businessGetPrefixes — route GET đọc dữ liệu nghiệp vụ (khách, lịch hẹn, đơn, phiếu, báo cáo, lịch sử,
// nhân sự, in) PHẢI có entry enforce_phase1=true (S-9 2026-10-18). Ngoại lệ = route dùng chung cho mọi
// user đã đăng nhập, không chở dữ liệu nghiệp vụ.
var businessGetPrefixes = []string{"/api/crm/", "/api/hr/", "/api/history/", "/api/print/"}

var businessGetExempt = map[string]bool{
	"GET /api/crm/indexeddb/sync":         true, // sync danh mục cấu hình IndexedDB cho mọi user
	"GET /api/crm/exchange-rate/currency": true, // tra tỷ giá công khai, không dữ liệu khách
}

// TestRoutePermMap_BusinessGetEnforced — route GET nghiệp vụ mới thêm mà quên entry (hoặc để
// enforce=false) ⇒ đỏ, thay vì lặng lẽ mở cho mọi nhân viên đã đăng nhập.
func TestRoutePermMap_BusinessGetEnforced(t *testing.T) {
	entries, err := seed.LoadRoutePermMap()
	if err != nil {
		t.Fatalf("LoadRoutePermMap: %v", err)
	}
	enforced := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.EnforcePhase1 && (e.Kind == seed.KindPerm || e.Kind == seed.KindRole) {
			enforced[e.Method+" "+e.Path] = true
		}
	}
	checked := 0
	var open []string
	for key, ri := range realRoutes(t) {
		if ri.Method != "GET" || businessGetExempt[key] {
			continue
		}
		biz := false
		for _, p := range businessGetPrefixes {
			if len(ri.Path) >= len(p) && ri.Path[:len(p)] == p {
				biz = true
				break
			}
		}
		if !biz {
			continue
		}
		checked++
		if !enforced[key] {
			open = append(open, key)
		}
	}
	// Đối chứng lệnh có chạy: phải soi được route GET nghiệp vụ (≈140), không phải tập rỗng.
	if checked < 50 {
		t.Fatalf("chỉ soi được %d route GET nghiệp vụ — nghi realRoutes/prefix hỏng", checked)
	}
	for _, k := range open {
		t.Errorf("route GET nghiệp vụ không có entry enforce_phase1=true: %s", k)
	}
	t.Logf("business GET routes checked=%d open=%d", checked, len(open))
}

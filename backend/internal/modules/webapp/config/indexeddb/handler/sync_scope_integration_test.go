//go:build integration

// Story-40/62 (E6/D15) — chốt hành vi `GET /crm/indexeddb/sync` sau khi schema bỏ thương hiệu:
//   - `services`/`serviceTypes` trả danh mục của DB.
//   - `users` KHÔNG còn key FE-compat `companyId` (cầu story-62 đã gỡ ở story-66 — FE story-43 bỏ
//     lọc dropdown theo company).
//
// ⛔ Chỉ chạy trên DB DÙNG-MỘT-LẦN (`TEST_MYSQL_DSN`); testhelpers.New DROP mọi bảng.
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"erpCluvix/internal/testhelpers"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

func insertRow(t *testing.T, table string, row map[string]any) uint {
	t.Helper()
	require.NoError(t, testDB.Table(table).Create(&row).Error, "seed %s", table)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM `"+table+"` WHERE id = ?", id) })
	return id
}

func syncAs(t *testing.T, branchID, userID uint, tables string) map[string][]map[string]any {
	t.Helper()
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(branchID, userID))
	h := New(Deps{DB: testDB})
	r.GET("/sync", h.Sync)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sync?tables="+tables, nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out map[string][]map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func idsOf(rows []map[string]any) map[uint]bool {
	out := map[uint]bool{}
	for _, r := range rows {
		if v, ok := r["id"].(float64); ok {
			out[uint(v)] = true
		}
	}
	return out
}

func TestSync_ServicesVaServiceTypes_TraDanhMuc(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	now := time.Now().Unix()
	name := fmt.Sprintf("S40 %d", time.Now().UnixNano())

	stID := insertRow(t, "service_type", map[string]any{"name": name, "status": 1, "created_at": now, "updated_at": now})
	svID := insertRow(t, "service", map[string]any{"code": fmt.Sprintf("S40DV%d", time.Now().UnixNano()), "name": name, "service_type_id": stID, "status": 1, "created_at": now, "updated_at": now})

	out := syncAs(t, tn.BranchID, 1, "services,serviceTypes")
	require.True(t, idsOf(out["services"])[svID], "services phải có dịch vụ vừa tạo")
	require.True(t, idsOf(out["serviceTypes"])[stID], "serviceTypes phải có nhóm dịch vụ vừa tạo")
}

func TestSync_Users_KhongConKeyFECompat(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	// Sync `users` loại cứng `u.id = 1` (tài khoản hệ thống). SchemaAll là DB trắng (story-62) ⇒ user
	// đầu tiên của package nhận id 1 — chèn 1 user đệm trước để user kiểm tra không rơi vào id đó.
	testhelpers.CreateUser(t, testDB, "s40pad", tn)
	uid := testhelpers.CreateScopedUser(t, testDB, "s40sync", tn, tn.BranchID)
	require.NotEqual(t, uint(1), uid)
	insertRow(t, "rbac_auth_assignment", map[string]any{"item_name": "admin", "user_id": uid, "created_at": time.Now().Unix()})

	rows := syncAs(t, tn.BranchID, 1, "users")["users"]
	var mine map[string]any
	for _, r := range rows {
		if v, ok := r["user_id"].(float64); ok && uint(v) == uid {
			mine = r
		}
	}
	require.NotNil(t, mine, "user vừa tạo phải có trong sync users")
	_, has := mine["companyId"]
	require.False(t, has, "users KHÔNG được còn key FE-compat companyId (story-66)")
	_, hasBranch := mine["branchId"]
	require.True(t, hasBranch)
}

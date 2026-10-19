//go:build integration

// Integration test helpers dùng chung cho package (TestMain, seed user/service/config_code, dựng context
// UpdateService). Test gốc của file (khoa trỏ chéo thương hiệu, 2026-08-23) đã bỏ cùng khái niệm khoa
// ở story-12 crm-standalone (D16).
package handler

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB
var seedSeq int64

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "config_service_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	appdb.DB = s.DB
	os.Exit(m.Run())
}

func nextSeq() string {
	return strconv.FormatInt(time.Now().Unix(), 10) + "_" + strconv.FormatInt(atomic.AddInt64(&seedSeq, 1), 10)
}

// ensureServiceConfigCode — bảo đảm có 1 dòng config_code "service" (codegen tra theo parent_table;
// SchemaAll là DB trắng, không seed sẵn). Idempotent (WHERE NOT EXISTS); dòng do hàm này chèn được dọn
// khi test xong.
func ensureServiceConfigCode(t *testing.T) {
	t.Helper()
	var n int64
	testDB.Table("config_code").Where("parent_table = 'service'").Count(&n)
	if n > 0 {
		return
	}
	branchID := testhelpers.CreateBranch(t, testDB)
	now := time.Now().Unix()
	if err := testDB.Exec(`INSERT INTO config_code (branch_id, name, parent_table, infix_code,
		include_branch_code, monthly_reset, serial_length, created_at, updated_at)
		SELECT ?, 'Dịch vụ', 'service', 'DV', 0, 0, 3, ?, ?
		WHERE NOT EXISTS (SELECT 1 FROM config_code WHERE parent_table = 'service')`,
		branchID, now, now).Error; err != nil {
		t.Fatalf("seed config_code: %v", err)
	}
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM config_code WHERE branch_id = ? AND parent_table = 'service'", branchID)
	})
}

func seedUser(t *testing.T) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{"username": "svc_test_" + nextSeq(), "password_hash": "x", "status": 1, "created_at": now, "updated_at": now}
	if err := testDB.Table("user").Create(&row).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	id := uint(testhelpers.LastInsertID(testDB))
	t.Cleanup(func() { testDB.Exec("DELETE FROM user WHERE id = ?", id) })
	return id
}

// grantPerm gán permission `perm` thẳng cho userID qua RBAC thật (rbac_auth_item/_child/_assignment)
// — handler.guard.go tự check quyền manageConfig trong handler (module config KHÔNG có
// PermissionEnforcer toàn cục, xem handler.go:42-43), nên middleware audit-mode không thay được;
// test phải seed quyền thật để RbacSvc.GetUserPermissions trả đúng perm.
//
// Role tạo riêng theo user (tên unique) để nhiều lần gọi grantPerm trong cùng suite không đụng
// PRIMARY KEY (item_name,user_id)/(parent,child); permission item dùng chung nên INSERT IGNORE.
func grantPerm(t *testing.T, userID uint, perm string) {
	t.Helper()
	now := time.Now().Unix()
	role := "test_role_" + nextSeq()

	if err := testDB.Exec(
		"INSERT IGNORE INTO rbac_auth_item (name, type, created_at, updated_at) VALUES (?, 2, ?, ?)",
		perm, now, now,
	).Error; err != nil {
		t.Fatalf("grantPerm seed permission item: %v", err)
	}
	if err := testDB.Exec(
		"INSERT INTO rbac_auth_item (name, type, created_at, updated_at) VALUES (?, 1, ?, ?)",
		role, now, now,
	).Error; err != nil {
		t.Fatalf("grantPerm seed role item: %v", err)
	}
	if err := testDB.Exec(
		"INSERT INTO rbac_auth_item_child (parent, child) VALUES (?, ?)", role, perm,
	).Error; err != nil {
		t.Fatalf("grantPerm link role->perm: %v", err)
	}
	if err := testDB.Exec(
		"INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, ?)",
		role, userID, now,
	).Error; err != nil {
		t.Fatalf("grantPerm assign role: %v", err)
	}
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM rbac_auth_assignment WHERE item_name = ? AND user_id = ?", role, userID)
		testDB.Exec("DELETE FROM rbac_auth_item_child WHERE parent = ? AND child = ?", role, perm)
		testDB.Exec("DELETE FROM rbac_auth_item WHERE name = ?", role)
	})
}

// seedService tạo 1 service.
func seedService(t *testing.T) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{
		"name": "Dịch vụ cũ", "code": "DV_" + nextSeq(),
		"status": 1, "created_at": now, "updated_at": now,
	}
	if err := testDB.Table("service").Create(&row).Error; err != nil {
		t.Fatalf("seed service: %v", err)
	}
	id := uint(testhelpers.LastInsertID(testDB))
	t.Cleanup(func() { testDB.Exec("DELETE FROM service WHERE id = ?", id) })
	return id
}

// svcUpdatedAt — updated_at hiện tại của dịch vụ (khoá lạc quan D6): request sửa phải gửi đúng giá trị đã đọc.
func svcUpdatedAt(t *testing.T, serviceID uint) int64 {
	t.Helper()
	var ts int64
	if err := testDB.Raw("SELECT updated_at FROM service WHERE id = ?", serviceID).Scan(&ts).Error; err != nil {
		t.Fatalf("đọc updated_at service: %v", err)
	}
	return ts
}

func updateCtx(t *testing.T, serviceID, userID uint, body map[string]any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	idStr := strconv.FormatUint(uint64(serviceID), 10)
	c.Request = httptest.NewRequest(http.MethodPut, "/config/service/"+idStr, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Set("userID", userID)
	return c, w
}

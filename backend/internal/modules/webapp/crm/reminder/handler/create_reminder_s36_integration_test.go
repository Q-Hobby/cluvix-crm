//go:build integration

package handler

// Story-36/62 (E6/D15): CreateReminder không còn khái niệm thương hiệu. FE cũ còn gửi khoá
// `company_id` ⇒ handler map-based phải gỡ khỏi body trước khi Create (bảng reminder không có cột
// này); mã nhắc việc vẫn sinh qua config_code của chi nhánh.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"erpCluvix/db"
	"erpCluvix/internal/testhelpers"
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
	db.DB = testDB // code dùng chung (codegen…) có thể đọc db.DB toàn cục
	os.Exit(m.Run())
}

func TestCreateReminder_BoQuaKhoaThuongHieuCuTuFE(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	now := time.Now().Unix()
	cfg := testhelpers.WithTenantCols(tn, map[string]any{
		"name":         "Nhắc việc",
		"parent_table": "reminder", "infix_code": "RE", "created_at": now, "updated_at": now,
	})
	require.NoError(t, testDB.Table("config_code").Create(&cfg).Error)
	cfgID := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id = ?", cfgID)
		testDB.Exec("DELETE FROM config_code WHERE id = ?", cfgID)
		testDB.Exec("DELETE FROM reminder WHERE customer_id = ?", tn.CustomerID)
	})
	uid := testhelpers.CreateScopedUser(t, testDB, "s36re", tn, tn.BranchID)

	for _, ca := range []struct {
		name   string
		legacy any // nil = không gửi
	}{{"KhongGuiKhoaCu", nil}, {"GuiKhoaCu", 987654}} {
		t.Run(ca.name, func(t *testing.T) {
			body := map[string]any{
				"type": 1, "start_time": now + 3600, "relatedUser": []uint{uid},
				// KHÔNG gửi branch_id: bảng reminder không có cột này (insert map sẽ lỗi) — chi nhánh cho
				// mã lấy từ config_code trong phạm vi chi nhánh của session.
				"note": "nhac viec s36 " + ca.name, "customer_id": tn.CustomerID,
			}
			if ca.legacy != nil {
				body["company_id"] = ca.legacy // FE-compat: khoá cũ phải bị gỡ khỏi map
			}
			raw, err := json.Marshal(body)
			require.NoError(t, err)
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/crm/reminder", strings.NewReader(string(raw)))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set("branchActiveId", tn.BranchID)
			c.Set("branchIds", []uint{tn.BranchID})
			c.Set("userID", uid)
			(&Handler{DB: testDB}).CreateReminder(c)
			require.Equal(t, 201, w.Code, "body: %s", w.Body.String())

			var row struct {
				Code string
			}
			require.NoError(t, testDB.Table("reminder").Select("code").
				Where("note = ?", "nhac viec s36 "+ca.name).Take(&row).Error)
			require.NotEmpty(t, row.Code, "mã nhắc việc phải sinh qua config_code của chi nhánh")
		})
	}
}

// Story-65 (D26e): bảng `reminder` KHÔNG có cột `branch_id` — client vẫn được phép gửi nó trong
// body để CHỌN chi nhánh sinh mã, nhưng handler PHẢI gỡ khỏi map trước khi Create, nếu không
// `INSERT INTO reminder (..., branch_id, ...)` nổ lỗi 1054 Unknown column (500).
func TestCreateReminder_BranchIDChonChiNhanhSinhMaKhongInsert(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	now := time.Now().Unix()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// config_code riêng cho branchB, infix khác branch mặc định của tn — dùng để chứng minh mã
	// sinh ra ĐÚNG chi nhánh được chọn qua body["branch_id"], không phải chi nhánh của session.
	// Infix NGẮN: mã cuối cùng ghép brand + branch.code (12 ký tự "BR"+10 số, xem CreateBranch) +
	// infix + YYMM + serial, cột `reminder.code` chỉ varchar(25) — dài hơn "RB" sẽ vỡ 1406.
	infixB := "RB"
	cfgB := map[string]any{
		"branch_id": branchB, "name": "Nhắc việc B", "parent_table": "reminder",
		"infix_code": infixB, "created_at": now, "updated_at": now,
	}
	require.NoError(t, testDB.Table("config_code").Create(&cfgB).Error)
	cfgBID := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id = ?", cfgBID)
		testDB.Exec("DELETE FROM config_code WHERE id = ?", cfgBID)
		testDB.Exec("DELETE FROM branch WHERE id = ?", branchB)
	})
	uid := testhelpers.CreateScopedUser(t, testDB, "s65re", tn, tn.BranchID)
	note := "nhac viec s65 branch " + suffix

	body := map[string]any{
		"type": 1, "start_time": now + 3600, "relatedUser": []uint{uid},
		"branch_id": branchB, "note": note, "customer_id": tn.CustomerID,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/crm/reminder", strings.NewReader(string(raw)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("branchActiveId", tn.BranchID) // session ở chi nhánh KHÁC branchB — không liên quan tới lựa chọn mã
	c.Set("branchIds", []uint{tn.BranchID})
	c.Set("userID", uid)
	(&Handler{DB: testDB}).CreateReminder(c)
	require.Equal(t, 201, w.Code, "gửi branch_id không được làm 500 (cột không tồn tại) — body: %s", w.Body.String())
	t.Cleanup(func() { testDB.Exec("DELETE FROM reminder WHERE note = ?", note) })

	var row struct {
		Code string
	}
	require.NoError(t, testDB.Table("reminder").Select("code").
		Where("note = ?", note).Take(&row).Error)
	require.Contains(t, row.Code, infixB, "mã phải sinh theo config_code của branchB (branch_id trong body chỉ dùng chọn chi nhánh, không insert)")
}

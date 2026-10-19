//go:build integration

// Story-39/62 (E6/D15, Q2): mẫu in mặc định chọn theo (branch, type) → (0, type); trùng khoá lấy bản
// id lớn nhất (ORDER rõ ràng). Session không cần gì ngoài chi nhánh.
// Chi nhánh A không bao giờ nhận mẫu riêng của chi nhánh B (branch vẫn là khoá chọn mẫu).
// Create bỏ cờ mặc định của mẫu cũ cùng (branch, type).
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ptDao "erpCluvix/internal/modules/webapp/config/print_template/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"

	"github.com/gin-gonic/gin"
)

func s39SeedTpl(t *testing.T, branchID uint, typ, name string, isDefault int) uint {
	t.Helper()
	now := time.Now().Unix()
	item := ptDao.PrintTemplate{
		BranchId: branchID, TemplateType: typ, Name: name,
		TemplateHtml: "<div>" + name + "</div>", IsDefault: isDefault, IsActive: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := testDB.Create(&item).Error; err != nil {
		t.Fatalf("seed print_templates: %v", err)
	}
	t.Cleanup(func() { testDB.Exec("DELETE FROM print_templates WHERE id = ?", item.ID) })
	return item.ID
}

func s39GetDefault(t *testing.T, query string) (int, *ptDao.PrintTemplate) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/config/print-template/default?"+query, nil)
	New(Deps{DB: testDB}).GetDefault(c)
	var body struct {
		Data *ptDao.PrintTemplate `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body.Data
}

func TestPrintTemplate_S39_GetDefaultTheoBranch(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	a := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	// Loại mẫu riêng cho test để không đụng dữ liệu seed.
	typ := define.PrintTemplatePaymentInDay
	testDB.Exec("UPDATE print_templates SET is_default = 0 WHERE template_type = ?", typ)

	globalOld := s39SeedTpl(t, 0, typ, "S39 global cũ", 1)
	globalNew := s39SeedTpl(t, 0, typ, "S39 global mới", 1) // id lớn hơn
	onlyB := s39SeedTpl(t, uint(branchB), typ, "S39 riêng B", 1)
	_ = globalOld

	// Không truyền chi nhánh ⇒ mẫu chung id lớn nhất.
	code, got := s39GetDefault(t, "type="+typ)
	if code != http.StatusOK || got == nil || got.ID != globalNew {
		t.Fatalf("không branch: HTTP %d, mẫu %v — muốn 200 + mẫu chung id lớn nhất %d", code, got, globalNew)
	}
	code, got = s39GetDefault(t, fmt.Sprintf("type=%s&branch_id=%d", typ, a.BranchID))
	if code != http.StatusOK || got == nil || got.ID != globalNew {
		t.Errorf("chi nhánh A (không có mẫu riêng): mẫu %v, muốn mẫu chung %d — KHÔNG được lấy mẫu riêng của chi nhánh B", got, globalNew)
	}
	code, got = s39GetDefault(t, fmt.Sprintf("type=%s&branch_id=%d", typ, branchB))
	if code != http.StatusOK || got == nil || got.ID != onlyB {
		t.Errorf("chi nhánh B: mẫu %v, muốn mẫu riêng %d", got, onlyB)
	}
}

func TestPrintTemplate_S39_CreateBoMacDinhTheoBranchType(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	branchX := testhelpers.CreateBranch(t, testDB)
	typ := define.PrintTemplatePayment
	oldDefault := s39SeedTpl(t, uint(branchX), typ, "S39 mặc định cũ", 1)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := fmt.Sprintf(`{"branch_id": %d, "template_type": %q, "name": "S39 mới", "template_html": "<p>x</p>", "is_default": 1}`, branchX, typ)
	c.Request = httptest.NewRequest(http.MethodPost, "/config/print-template", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", uint(1))
	New(Deps{DB: testDB}).Create(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("HTTP %d, muốn 201 — %s", w.Code, w.Body.String())
	}
	var created ptDao.PrintTemplate
	testDB.Where("name = ? AND branch_id = ?", "S39 mới", branchX).Order("id DESC").First(&created)
	t.Cleanup(func() { testDB.Exec("DELETE FROM print_templates WHERE id = ?", created.ID) })
	if created.ID == 0 {
		t.Fatal("không tìm thấy mẫu vừa tạo")
	}
	var oldFlag int
	testDB.Raw("SELECT is_default FROM print_templates WHERE id = ?", oldDefault).Scan(&oldFlag)
	if oldFlag != 0 {
		t.Errorf("mẫu mặc định cũ cùng (branch, type) phải bị bỏ cờ: is_default = %d", oldFlag)
	}
}

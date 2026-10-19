//go:build integration

// Integration test — QA pass 1 finding "patch/low" (story-03, epic onboarding-setup-wizard):
// nhánh `if err := h.assignBranchToCreator(...); err != nil { response.ServerError(c, err) }`
// trong `CreateBranch` (AC4) KHÔNG có test nào khoá (package trước đây `[no test files]`).
// Mutation `_ = h.assignBranchToCreator(...)` (nuốt lỗi, quay về hành vi cũ) build/vet vẫn xanh —
// phải test qua HTTP thật (httptest) để bắt được.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erpCluvix/internal/testhelpers"
)

// newCreateBranchRouter — gắn userID vào context như middleware thật set.
func newCreateBranchRouter(userID uint) *gin.Engine {
	h := New(Deps{DB: testDB, Cache: nil})
	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})
	r.POST("/branch", h.CreateBranch)
	return r
}

func postCreateBranchJSON(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/branch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setUserColumnRawCB(t *testing.T, userID uint, column, sqlExpr string) {
	t.Helper()
	if err := testDB.Exec("UPDATE user SET "+column+" = "+sqlExpr+" WHERE id = ?", userID).Error; err != nil {
		t.Fatalf("setUserColumnRawCB(%s = %s): %v", column, sqlExpr, err)
	}
}

func readUserColumnRawCB(t *testing.T, userID uint, column string) *string {
	t.Helper()
	var row struct {
		Value *string `gorm:"column:value"`
	}
	if err := testDB.Table("user").Select(column+" AS value").Where("id = ?", userID).Take(&row).Error; err != nil {
		t.Fatalf("readUserColumnRawCB(%s): %v", column, err)
	}
	return row.Value
}

// TestCreateBranch_AssignCreatorFails_Returns500_ColumnUnchanged — AC4: creator có
// `user.branchId` là JSON hợp lệ nhưng không phải mảng số ⇒ AppendID lỗi ⇒ handler PHẢI trả 500,
// cột giữ nguyên (không bị ghi đè).
func TestCreateBranch_AssignCreatorFails_Returns500_ColumnUnchanged(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	tenant, err := testhelpers.CreateTestTenant(testDB)
	if err != nil {
		t.Fatalf("CreateTestTenant: %v", err)
	}
	userID := testhelpers.CreateUser(t, testDB, "qa03cb", tenant)
	setUserColumnRawCB(t, userID, "branchId", `'"corrupt"'`)

	r := newCreateBranchRouter(userID)
	name := "QA03 Corrupt Branch"
	code := fmt.Sprintf("QB%d", time.Now().UnixNano()%1_000_000) // code size:25 — giữ ngắn
	body := fmt.Sprintf(`{"name":%q,"code":%q,"address":"12 Nguyễn Trãi"}`, name, code)
	w := postCreateBranchJSON(r, body)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, muốn 500. body=%s", w.Code, w.Body.String())
	}
	got := readUserColumnRawCB(t, userID, "branchId")
	if got == nil || *got != `"corrupt"` {
		t.Fatalf("branchId phải giữ nguyên sau lỗi, hiện = %v", got)
	}

	// story-10 AC2: Create → SortOrder → AppendID → SeedConfigCodeForBranch nay nằm TRONG 1
	// transaction ⇒ AppendID lỗi phải rollback branch vừa INSERT. Trước đây branch nằm lại trong DB
	// mà creator không có quyền thấy nó (mồ côi quyền) — deferred-work.md entry story-03. Ca này
	// từng phải DỌN branch rác; nay dọn đổi thành KHẲNG ĐỊNH không có gì để dọn.
	var branchIDs []uint
	if err := testDB.Table("branch").Where("name = ? AND code = ?", name, code).Pluck("id", &branchIDs).Error; err != nil {
		t.Fatalf("tìm branch sau rollback: %v", err)
	}
	if len(branchIDs) != 0 {
		// Dọn trước khi fail để không rò rỉ sang test khác.
		for _, id := range branchIDs {
			testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", id)
			testDB.Exec("DELETE FROM branch WHERE id = ?", id)
		}
		t.Fatalf("còn %d dòng branch sau khi request trả 500 — Create+AppendID chưa nằm trong cùng transaction (record mồ côi quyền)", len(branchIDs))
	}

	// config_code cũng phải rollback theo (seed nằm trong cùng tx).
	var cc int64
	if err := testDB.Table("config_code").Where("branch_id NOT IN (SELECT id FROM branch)").Count(&cc).Error; err != nil {
		t.Fatalf("đếm config_code mồ côi: %v", err)
	}
	if cc != 0 {
		t.Fatalf("còn %d dòng config_code trỏ tới branch không tồn tại — seed không rollback cùng tx", cc)
	}
}

// TestCreateBranch_Success_AssignsCreator — ca thành công: 201, shape response cũ
// ({"data": ..., "message": ...}), và `user.branchId` chứa id branch vừa tạo.
func TestCreateBranch_Success_AssignsCreator(t *testing.T) {
	if testDB == nil {
		t.Skip("Docker not available")
	}
	tenant, err := testhelpers.CreateTestTenant(testDB)
	if err != nil {
		t.Fatalf("CreateTestTenant: %v", err)
	}
	userID := testhelpers.CreateUser(t, testDB, "qa03cb", tenant)
	// CreateUser không set branchId mặc định — nhưng reset tường minh về NULL để
	// assert kết quả gọn (id branch mới tạo là phần tử DUY NHẤT).
	setUserColumnRawCB(t, userID, "branchId", "NULL")

	r := newCreateBranchRouter(userID)
	name := "QA03 OK Branch"
	code := fmt.Sprintf("QB%d", time.Now().UnixNano()%1_000_000) // code size:25 — giữ ngắn
	body := fmt.Sprintf(`{"name":%q,"code":%q,"address":"12 Nguyễn Trãi"}`, name, code)
	w := postCreateBranchJSON(r, body)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, muốn 201. body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v; body=%s", err, w.Body.String())
	}
	if resp.Data.ID == 0 {
		t.Fatalf("response thiếu data.id (shape thay đổi?): %s", w.Body.String())
	}
	if resp.Message == "" {
		t.Fatalf("response thiếu message (shape thay đổi?): %s", w.Body.String())
	}
	t.Cleanup(func() {
		if err := testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", resp.Data.ID).Error; err != nil {
			t.Errorf("dọn config_code test: %v", err)
		}
		if err := testDB.Exec("DELETE FROM branch WHERE id = ?", resp.Data.ID).Error; err != nil {
			t.Errorf("dọn branch test: %v", err)
		}
	})

	got := readUserColumnRawCB(t, userID, "branchId")
	want := fmt.Sprintf("[%d]", resp.Data.ID)
	if got == nil || *got != want {
		t.Fatalf("branchId sau tạo = %v, muốn %s", got, want)
	}
}

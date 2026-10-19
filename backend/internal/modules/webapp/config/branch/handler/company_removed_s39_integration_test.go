//go:build integration

// Story-39/62 (E6/D15): branch không còn thuộc thương hiệu (schema mới không có cột đó).
//   - List/Get trả mọi chi nhánh (không lọc thêm).
//   - Create không bắt buộc/không nhận key FE-compat `companyId` từ client (không còn 422);
//     config_code được seed cho chi nhánh mới.
//   - Update bỏ qua key FE-compat `companyId` client gửi, field khác lưu được.
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

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"
)

func TestBranch_S39_ListTraMoiChiNhanh(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	rows := map[string]uint{
		"chi nhánh 1": testhelpers.CreateBranch(t, testDB),
		"chi nhánh 2": testhelpers.CreateBranch(t, testDB),
	}

	appdb.DB = testDB // pagination.Paginate đếm qua db.DB
	h := New(Deps{DB: testDB})
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?per-page=200", nil)
	h.ListBranch(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	seen := map[uint]int{}
	for _, it := range body.Items {
		seen[it.ID]++
	}
	for kind, id := range rows {
		if seen[id] != 1 {
			t.Errorf("%s id=%d xuất hiện %d lần, muốn đúng 1", kind, id, seen[id])
		}
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c2.Params = gin.Params{{Key: "id", Value: fmt.Sprint(rows["chi nhánh 2"])}}
	h.GetBranch(c2)
	if w2.Code != http.StatusOK {
		t.Errorf("GetBranch: status = %d, muốn 200", w2.Code)
	}
}

func TestBranch_S39_CreateUpdateBoQuaKeyFECompat(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	a := testhelpers.FreshTenant(t, testDB)
	userID := testhelpers.CreateUser(t, testDB, "s39br", a)
	setUserColumnRawCB(t, userID, "branchId", "NULL")

	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) { c.Set("userID", userID); c.Next() })
	r.POST("/branch", h.CreateBranch)
	r.PUT("/branch/:id", h.UpdateBranch)

	code := fmt.Sprintf("S%d", time.Now().UnixNano()%1_000_000_000)
	body := fmt.Sprintf(`{"name":"S39 CN %s","code":%q,"address":"1 Lê Lợi","companyId":[99]}`, code, code)
	w := postCreateBranchJSON(r, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, muốn 201 (key FE-compat bị bỏ qua, không bắt buộc). body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.ID == 0 {
		t.Fatalf("thiếu data.id: %s", w.Body.String())
	}
	id := resp.Data.ID
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", id)
		testDB.Exec("DELETE FROM branch WHERE id = ?", id)
	})

	var total int64
	testDB.Table("config_code").Where("branch_id = ?", id).Count(&total)
	if total == 0 {
		t.Errorf("config_code của chi nhánh mới: 0 dòng — muốn > 0 (SeedConfigCodeForBranch)")
	}

	// Update gửi key FE-compat ⇒ bỏ qua, field khác lưu được.
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/branch/%d", id),
		bytes.NewBufferString(fmt.Sprintf(`{"name":"S39 CN đổi %s","code":%q,"address":"2 Lê Lợi","companyId":[99]}`, code, code)))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("update: status = %d body=%s", w2.Code, w2.Body.String())
	}
	var addr string
	testDB.Raw("SELECT address FROM branch WHERE id = ?", id).Scan(&addr)
	if addr != "2 Lê Lợi" {
		t.Errorf("address = %q, muốn lưu được", addr)
	}
}

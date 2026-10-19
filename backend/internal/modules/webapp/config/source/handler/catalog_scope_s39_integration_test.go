//go:build integration

// Story-39/62 (E6/D15): danh mục config không lọc theo thương hiệu (schema không còn cột đó).
//
//   - List source / group_source / customer_category thấy ĐỦ mọi dòng — mỗi dòng đúng 1 lần
//     (không mất dòng, không nhân đôi).
//   - Branch scope GIỮ nguyên (AC3): OverviewChartSource đếm khách qua ScopedDBNullBranch
//     "customer" ⇒ user chi nhánh A không đếm khách chi nhánh B (khách chưa gắn chi nhánh vẫn tính).
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	customerCategoryHandler "erpCluvix/internal/modules/webapp/config/customer_category/handler"
	groupSourceHandler "erpCluvix/internal/modules/webapp/config/group_source/handler"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "config_source_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		if errors.Is(err, testhelpers.ErrNoDSN) {
			log.Printf("testhelpers: %v — skipping integration tests", err)
			os.Exit(m.Run())
		}
		log.Fatalf("testhelpers: %v", err)
	}
	defer s.Close()
	testDB = s.DB
	appdb.DB = s.DB // pagination.Paginate đếm qua db.DB
	os.Exit(m.Run())
}

func requireDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
}

// listCtx — request GET; branchActive = 0 ⇒ không set phạm vi chi nhánh.
func listCtx(branchActive uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?per-page=200&_keyword=S39", nil)
	c.Set("userID", uint(1))
	if branchActive > 0 {
		c.Set("branchActiveId", branchActive)
		c.Set("branchIds", []uint{branchActive})
	}
	return c, w
}

// seedCatalogRow chèn 1 dòng danh mục.
func seedCatalogRow(t *testing.T, table string) (uint, string) {
	t.Helper()
	now := time.Now().Unix()
	name := fmt.Sprintf("S39 %s %d", table, time.Now().UnixNano())
	row := map[string]any{"name": name, "status": 1, "created_at": now, "updated_at": now}
	if err := testDB.Table(table).Create(&row).Error; err != nil {
		t.Fatalf("seed %s: %v", table, err)
	}
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM "+table+" WHERE id = ?", id) })
	return id, name
}

// countIDs đếm số lần mỗi id xuất hiện trong items của response list.
func countIDs(t *testing.T, w *httptest.ResponseRecorder) map[uint]int {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	out := map[uint]int{}
	for _, it := range body.Items {
		out[it.ID]++
	}
	return out
}

func TestCatalogList_S39_DuDong(t *testing.T) {
	requireDB(t)
	a := testhelpers.FreshTenant(t, testDB)

	lists := []struct {
		table string
		list  func(c *gin.Context)
	}{
		{"source", New(Deps{DB: testDB}).ListSource},
		{"group_source", groupSourceHandler.New(groupSourceHandler.Deps{DB: testDB}).ListGroupSource},
		{"customer_category", customerCategoryHandler.New(customerCategoryHandler.Deps{DB: testDB}).ListCustomerCategory},
	}
	for _, l := range lists {
		t.Run(l.table, func(t *testing.T) {
			r1, _ := seedCatalogRow(t, l.table)
			r2, _ := seedCatalogRow(t, l.table)
			for _, branchActive := range []uint{0, a.BranchID} {
				c, w := listCtx(branchActive)
				l.list(c)
				got := countIDs(t, w)
				for name, id := range map[string]uint{"dòng 1": r1, "dòng 2": r2} {
					if got[id] != 1 {
						t.Errorf("session branch=%d: %s id=%d xuất hiện %d lần, muốn đúng 1", branchActive, name, id, got[id])
					}
				}
			}
		})
	}
}

// AC3 — branch scope giữ nguyên trong phạm vi: OverviewChartSource (ScopedDBNullBranch customer).
func TestOverviewChartSource_S39_BranchScopeGiuNguyen(t *testing.T) {
	requireDB(t)
	a := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	srcID, srcName := seedCatalogRow(t, "source")

	seedCustomer := func(branch *uint) {
		id := testhelpers.CreateScopedCustomer(t, testDB, a, branch)
		if err := testDB.Exec("UPDATE customer SET source_id = ? WHERE id = ?", srcID, id).Error; err != nil {
			t.Fatalf("gán source: %v", err)
		}
	}
	branchA := a.BranchID
	seedCustomer(&branchA)
	seedCustomer(&branchA)
	seedCustomer(nil) // chưa gắn chi nhánh ⇒ vẫn thuộc mọi phạm vi (vế OR branch_id IS NULL)
	for i := 0; i < 5; i++ {
		seedCustomer(&branchB) // chi nhánh B: KHÔNG được đếm khi user ở chi nhánh A
	}

	total := func(branchActive uint) int {
		c, w := listCtx(branchActive)
		New(Deps{DB: testDB}).OverviewChartSource(c)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
		}
		var rows []struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
			t.Fatalf("decode: %v body=%s", err, w.Body.String())
		}
		for _, r := range rows {
			if r.Name == srcName {
				return r.Value
			}
		}
		return 0
	}

	if got := total(a.BranchID); got != 3 {
		t.Errorf("chi nhánh A: đếm %d khách của nguồn, muốn 3 (2 chi nhánh A + 1 chưa gắn chi nhánh; 5 khách chi nhánh B phải bị loại)", got)
	}
	if got := total(branchB); got != 6 {
		t.Errorf("chi nhánh B: đếm %d khách của nguồn, muốn 6 (5 chi nhánh B + 1 chưa gắn chi nhánh)", got)
	}
}

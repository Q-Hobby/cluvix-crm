//go:build integration

// Story-39/62 (E6/D15): nhóm dịch vụ không thuộc thương hiệu (schema không còn cột đó).
//   - Create chỉ cần tên.
//   - Kiểm trùng tên (bulk-insert, import) là TOÀN DB: tên đã có ⇒ bỏ qua, không tạo bản thứ hai.
//   - Import tạo mới chạy được với bộ cột hiện hành.
//   - List thấy đủ nhóm, mỗi dòng 1 lần.
package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "config_service_type_test"})
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

func seedST(t *testing.T, name string) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{"name": name, "status": 1, "created_at": now, "updated_at": now}
	if err := testDB.Table("service_type").Create(&row).Error; err != nil {
		t.Fatalf("seed service_type: %v", err)
	}
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM service_type WHERE id = ?", id) })
	return id
}

func cleanupByName(t *testing.T, names ...string) {
	t.Cleanup(func() { testDB.Exec("DELETE FROM service_type WHERE name IN ?", names) })
}

func countByName(t *testing.T, name string) (n int64) {
	t.Helper()
	testDB.Table("service_type").Where("name = ?", name).Count(&n)
	return
}

func ctxJSON(method, path string, body any, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("userID", userID)
	return c, w
}

func TestServiceType_S39_CreateChiCanTen(t *testing.T) {
	requireDB(t)
	h := New(Deps{DB: testDB})
	name := fmt.Sprintf("S39 ST create %d", time.Now().UnixNano())
	cleanupByName(t, name)

	c, w := ctxJSON(http.MethodPost, "/config/service-type", map[string]any{"name": name}, 1)
	h.CreateServiceType(c)
	if w.Code != http.StatusCreated {
		t.Fatalf("HTTP %d, muốn 201 — %s", w.Code, w.Body.String())
	}
	if n := countByName(t, name); n != 1 {
		t.Errorf("tạo %d dòng — muốn 1", n)
	}
}

func TestServiceType_S39_KiemTrungTenToanDB(t *testing.T) {
	requireDB(t)
	h := New(Deps{DB: testDB})
	seq := time.Now().UnixNano()
	existingB := fmt.Sprintf("S39 ST đã có %d", seq)
	seedST(t, existingB)
	// Import tạo mới; tên trùng toàn DB bỏ qua. (Endpoint bulk-insert "Có sẵn" đã gỡ 2026-10-19.)
	fresh2 := fmt.Sprintf("S39 ST import %d", seq)
	cleanupByName(t, fresh2)
	f := excelize.NewFile()
	sh := f.GetSheetName(0)
	_ = f.SetSheetRow(sh, "A1", &[]any{"id", "Nhóm dịch vụ", "Trạng thái"})
	_ = f.SetSheetRow(sh, "A2", &[]any{"", fresh2, "1"})
	_ = f.SetSheetRow(sh, "A3", &[]any{"", existingB, "1"})
	var xl bytes.Buffer
	if err := f.Write(&xl); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "Nhom_dich_vu.xlsx")
	_, _ = fw.Write(xl.Bytes())
	_ = mw.Close()
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/config/service-type/import-from-excel", &body)
	c3.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c3.Set("userID", uint(1))
	h.ImportServiceTypeFromExcel(c3)
	if w3.Code != http.StatusOK {
		t.Fatalf("import HTTP %d — %s", w3.Code, w3.Body.String())
	}
	if n := countByName(t, fresh2); n != 1 {
		t.Errorf("import dòng mới: %d dòng, muốn 1", n)
	}
	if n := countByName(t, existingB); n != 1 {
		t.Errorf("import tên đã có phải bỏ qua: %d dòng, muốn 1", n)
	}
}

func TestServiceType_S39_ListDuDong(t *testing.T) {
	requireDB(t)
	seq := time.Now().UnixNano()
	ids := []uint{seedST(t, fmt.Sprintf("S39 L A %d", seq)), seedST(t, fmt.Sprintf("S39 L B %d", seq))}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?per-page=200&_keyword=S39", nil)
	New(Deps{DB: testDB}).ListServiceType(c)
	var body struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	seen := map[uint]int{}
	for _, it := range body.Items {
		seen[it.ID]++
	}
	for _, id := range ids {
		if seen[id] != 1 {
			t.Errorf("nhóm id=%d xuất hiện %d lần, muốn 1", id, seen[id])
		}
	}
}

//go:build integration

// Integration test — seed `control_field_in_table` (migration `*_seed_control_field_in_table`,
// story-30 P5). FE gọi CỐ ĐỊNH id 1 (form khách hàng) / id 2 (form lead); thiếu dòng ⇒ 404 ⇒ 2 trang
// cấu hình form văng notfound trên mọi bản cài mới. SchemaFull (có seed), KHÔNG tự chèn dòng.
package handler

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaFull)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

func getFieldCustomer(t *testing.T, id int) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/control-field-in-table/:id/get-field-customer", New(Deps{DB: testDB}).GetFieldCustomer)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/control-field-in-table/%d/get-field-customer", id), nil))
	return w.Code
}

// Bản cài mới (chỉ migrate) ⇒ id 1 và 2 trả 200.
func TestSeedControlField_GetFieldCustomer_Id1Id2(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	for _, id := range []int{1, 2} {
		if code := getFieldCustomer(t, id); code != http.StatusOK {
			t.Fatalf("GET id=%d trên DB vừa migrate: HTTP %d, muốn 200", id, code)
		}
	}
}

// Chạy LẠI file seed: không nhân bản, không đè cấu hình người dùng đã lưu vào `control`.
func TestSeedControlField_Idempotent_KhongDeCauHinh(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	const saved = `[{"field_name":"phone","required":true}]`
	if err := testDB.Exec("UPDATE control_field_in_table SET control = ? WHERE id = 1", saved).Error; err != nil {
		t.Fatalf("lưu cấu hình: %v", err)
	}
	t.Cleanup(func() { testDB.Exec("UPDATE control_field_in_table SET control = NULL WHERE id = 1") })

	files, _ := filepath.Glob(filepath.Join(testhelpers.MigrationsDir(), "*_seed_control_field_in_table.up.sql"))
	if len(files) != 1 {
		t.Fatalf("muốn đúng 1 file *_seed_control_field_in_table.up.sql, có %d", len(files))
	}
	sql, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("đọc %s: %v", files[0], err)
	}
	if err := testDB.Exec(string(sql)).Error; err != nil {
		t.Fatalf("chạy lại seed: %v", err)
	}

	var n int64
	testDB.Table("control_field_in_table").Count(&n)
	if n != 2 {
		t.Fatalf("sau chạy lại seed: %d dòng, muốn 2", n)
	}
	var control string
	testDB.Raw("SELECT CAST(control AS CHAR) FROM control_field_in_table WHERE id = 1").Scan(&control)
	if control == "" {
		t.Fatalf("chạy lại seed đã đè mất cấu hình đã lưu của id=1")
	}
}

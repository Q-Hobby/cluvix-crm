//go:build integration

// Integration test — khoá lạc quan `print_templates` theo `updated_at` (D21/story-57, thay cột
// `version` riêng đã bỏ). Kịch bản đáng test là "cùng giây": 2 request PUT gửi CÙNG giá trị
// updated_at TRONG CÙNG 1 GIÂY — nếu bump SAI (SET updated_at = now thay vì
// GREATEST(now, updated_at+1)) thì request 2 vẫn khớp WHERE ⇒ ghi đè mất nội dung request 1.
// Xem internal/contracttest/story17_optlock_same_second_test.go cho pattern gốc (9 bảng D6).
package handler

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	ptDao "erpCluvix/internal/modules/webapp/config/print_template/dao"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "print_template_handler_test"})
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

func TestS57Optlock_PrintTemplate_SameSecondConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}
	tenant := testhelpers.FreshTenant(t, testDB)
	now := time.Now().Unix()
	item := ptDao.PrintTemplate{
		BranchId:     0,
		TemplateType: define.PrintTemplatePayment,
		Name:         "Mẫu gốc",
		TemplateHtml: "<div>gốc</div>",
		IsDefault:    0,
		IsActive:     1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := testDB.Create(&item).Error; err != nil {
		t.Fatalf("seed print_templates: %v", err)
	}
	t.Cleanup(func() { testDB.Exec("DELETE FROM print_templates WHERE id = ?", item.ID) })

	h := New(Deps{DB: testDB})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/_s57/print-template/:id", func(c *gin.Context) {
		c.Set("branchIds", []uint{tenant.BranchID})
		c.Set("branchActiveId", uint(0))
		c.Set("userID", uint(1))
		h.Update(c)
	})
	put := func(ts int64, name string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		body := fmt.Sprintf(`{"updated_at": %d, "template_type": %q, "name": %q, "template_html": "<div>x</div>"}`,
			ts, define.PrintTemplatePayment, name)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/_s57/print-template/%d", item.ID), bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	for attempt := 0; attempt < 3; attempt++ {
		start := time.Now()
		time.Sleep(time.Until(time.Unix(start.Unix()+1, 0)) + 5*time.Millisecond)
		sec := time.Now().Unix()
		testDB.Exec("UPDATE print_templates SET updated_at = ? WHERE id = ?", sec, item.ID)

		if w := put(sec, "sửa lần 1"); w.Code != http.StatusOK {
			t.Fatalf("request 1 phải 200, got %d — %s", w.Code, w.Body.String())
		}
		var gotUpdatedAt int64
		testDB.Raw("SELECT updated_at FROM print_templates WHERE id = ?", item.ID).Scan(&gotUpdatedAt)
		if gotUpdatedAt <= sec {
			t.Fatalf("updated_at phải tăng nghiêm ngặt: %d → %d", sec, gotUpdatedAt)
		}

		if w := put(sec, "sửa lần 2 (stale)"); w.Code != http.StatusConflict {
			t.Fatalf("request 2 cùng giây, cùng updated_at cũ phải 409, got %d — %s", w.Code, w.Body.String())
		}

		var name string
		testDB.Raw("SELECT name FROM print_templates WHERE id = ?", item.ID).Scan(&name)
		if name != "sửa lần 1" {
			t.Fatalf("tên bị ghi đè bởi request stale: %q", name)
		}
		if time.Now().Unix() == sec {
			return
		}
		t.Logf("vắt qua ranh giới giây (lần %d) — chạy lại", attempt+1)
	}
	t.Fatal("không chạy được kịch bản trong cùng 1 giây sau 3 lần")
}

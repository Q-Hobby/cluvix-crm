//go:build integration

// Integration test — SwitchProfile.
//
// Story-58/62 (E6/D15): handler CHỈ đổi chi nhánh; key FE-compat `companyActive_id` FE cũ còn gửi
// trong body bị bỏ qua (schema không còn cột đó). Ca chi nhánh ngoài quyền / user không có chi nhánh:
// `switch_profile_branch_integration_test.go`.
//
// ⚠ Chi nhánh vẫn phải bỏ trống được. Test `BranchEmpty_Succeeds` khoá hành vi đó để lần dọn dẹp
// "cho đối xứng" sau này không âm thầm biến chi nhánh thành bắt buộc.
package handler

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	logger.Init(logger.Config{Env: "test", AppCode: "user_profile_test"})
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

const (
	testBranchID = 1
	testUserID   = 1
)

// seed dựng branch/user_profile tối thiểu để SwitchProfile chạy được thật.
func seed(t *testing.T) {
	t.Helper()
	testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", testUserID)
	testDB.Exec("INSERT IGNORE INTO branch (id, name) VALUES (?, ?)", testBranchID, "CN Test")
	if err := testDB.Exec(
		"INSERT INTO user_profile (user_id, fullname) VALUES (?, ?)",
		testUserID, "Test User").Error; err != nil {
		t.Fatalf("seed user_profile: %v", err)
	}
}

func callSwitch(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := testhelpers.NewGin()
	r.POST("/config/user-profile/switch-profile",
		testhelpers.AuthMiddleware(testBranchID, testUserID),
		func(c *gin.Context) {
			h := New(Deps{DB: testDB})
			h.SwitchProfile(c)
		})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/config/user-profile/switch-profile",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// AC7 — chi nhánh vẫn PHẢI bỏ trống được.
func TestSwitchProfile_BranchEmpty_Succeeds(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	for _, body := range []string{
		`{}`,
		`{"branchActive_id": null}`,
		`{"branchActive_id": 0}`,
	} {
		seed(t)
		w := callSwitch(t, body)
		if w.Code != http.StatusOK {
			t.Errorf("body %s → HTTP %d, muốn 200. Resp: %s", body, w.Code, w.Body.String())
			continue
		}
		var branch *int
		testDB.Raw("SELECT branchActive_id FROM user_profile WHERE user_id = ?", testUserID).Scan(&branch)
		if branch != nil {
			t.Errorf("body %s → branchActive_id = %d, muốn NULL (chi nhánh không bắt buộc)", body, *branch)
		}
	}
}

// Đối chứng dương tính: đường hợp lệ vẫn ghi được — chống "xanh giả" khi handler từ chối MỌI thứ.
func TestSwitchProfile_HappyPath_Writes(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seed(t)
	// FE cũ còn gửi key FE-compat `companyActive_id` — phải bị bỏ qua, không làm hỏng request.
	w := callSwitch(t, `{"companyActive_id": 1, "branchActive_id": 1}`)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, muốn 200. Body: %s", w.Code, w.Body.String())
	}
	var branch *int
	testDB.Raw("SELECT branchActive_id FROM user_profile WHERE user_id = ?", testUserID).Scan(&branch)
	if branch == nil || *branch != testBranchID {
		t.Fatalf("branchActive_id = %v, muốn %d", branch, testBranchID)
	}
}

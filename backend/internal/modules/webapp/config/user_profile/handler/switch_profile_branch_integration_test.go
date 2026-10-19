//go:build integration

// Integration test — GIỮ HÀNH VI chi nhánh của SwitchProfile (story-58).
//
//   - chọn chi nhánh KHÔNG thuộc `branchIds` của user ⇒ từ chối, DB không đổi;
//   - user KHÔNG có chi nhánh nào ⇒ chọn chi nhánh bất kỳ đều bị từ chối (rỗng ≠ "không giới hạn"
//     ở cửa này — khác IsBranchAllowed);
//   - chi nhánh thuộc quyền ⇒ ghi được; chi nhánh NULL vẫn cho phép (TestSwitchProfile_BranchEmpty_*).
package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// callSwitchAs gọi SwitchProfile với branchIds tuỳ ý (nil ⇒ user không có chi nhánh nào).
func callSwitchAs(t *testing.T, branchIDs []uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/config/user-profile/switch-profile",
		func(c *gin.Context) {
			c.Set("userID", uint(testUserID))
			c.Set("branchIds", branchIDs)
			c.Next()
		},
		func(c *gin.Context) { New(Deps{DB: testDB}).SwitchProfile(c) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/config/user-profile/switch-profile", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func branchActive(t *testing.T) *int {
	t.Helper()
	var b *int
	if err := testDB.Raw("SELECT branchActive_id FROM user_profile WHERE user_id = ?", testUserID).Scan(&b).Error; err != nil {
		t.Fatalf("đọc branchActive_id: %v", err)
	}
	return b
}

func seedBranch2(t *testing.T) {
	t.Helper()
	testDB.Exec("INSERT IGNORE INTO branch (id, name) VALUES (2, 'CN 2')")
}

func TestSwitchProfile_S58_BranchNotAllowed_Rejected(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seed(t)
	seedBranch2(t)
	testDB.Exec("UPDATE user_profile SET branchActive_id = 1 WHERE user_id = ?", testUserID)

	w := callSwitchAs(t, []uint{1}, `{"branchActive_id": 2}`)
	if w.Code == http.StatusOK {
		t.Fatalf("chi nhánh ngoài quyền được chấp nhận: HTTP=%d body=%s", w.Code, w.Body.String())
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("HTTP=%d — muốn 422 forbidden. Body: %s", w.Code, w.Body.String())
	}
	if b := branchActive(t); b == nil || *b != 1 {
		t.Errorf("branchActive_id bị đổi thành %v — muốn giữ 1", b)
	}
}

func TestSwitchProfile_S58_UserWithoutBranch_Rejected(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seed(t)
	w := callSwitchAs(t, nil, `{"branchActive_id": 1}`)
	if w.Code == http.StatusOK {
		t.Fatalf("user không có chi nhánh vẫn chọn được chi nhánh: HTTP=%d body=%s", w.Code, w.Body.String())
	}
	if b := branchActive(t); b != nil {
		t.Errorf("branchActive_id bị ghi %d — muốn NULL", *b)
	}
	// Bỏ trống chi nhánh vẫn được với user không có chi nhánh.
	if w := callSwitchAs(t, nil, `{"branchActive_id": null}`); w.Code != http.StatusOK {
		t.Errorf("bỏ trống chi nhánh: HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
}

func TestSwitchProfile_S58_BranchAllowed_Writes(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seed(t)
	seedBranch2(t)
	w := callSwitchAs(t, []uint{1, 2}, `{"branchActive_id": 2}`)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP=%d body=%s — muốn 200", w.Code, w.Body.String())
	}
	if b := branchActive(t); b == nil || *b != 2 {
		t.Errorf("branchActive_id = %v — muốn 2", b)
	}
}

// Fail-closed: chi nhánh nằm trong `branchIds` của user (JSON user.branchId có thể còn id đã xoá)
// nhưng KHÔNG có thật ⇒ từ chối.
func TestSwitchProfile_S58_BranchInIDsButMissing_Rejected(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	seed(t)
	testDB.Exec("DELETE FROM branch WHERE id = 987654")
	w := callSwitchAs(t, []uint{1, 987654}, `{"branchActive_id": 987654}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("chi nhánh không tồn tại: HTTP=%d body=%s — muốn 422 forbidden", w.Code, w.Body.String())
	}
	if b := branchActive(t); b != nil {
		t.Errorf("branchActive_id bị ghi %d — muốn giữ NULL", *b)
	}
}

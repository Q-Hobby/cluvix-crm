//go:build integration

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

	"erpCluvix/internal/testhelpers"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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
	os.Exit(m.Run())
}

func ccRouter(branchID, userID uint) *gin.Engine {
	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) {
		c.Set("branchActiveId", branchID)
		c.Set("branchIds", []uint{branchID})
		c.Set("userID", userID)
		c.Next()
	})
	r.POST("/config-code", h.CreateConfigCode)
	r.PUT("/config-code/:id", h.UpdateConfigCode)
	return r
}

func ccDo(r *gin.Engine, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func ccBody(branchID uint, table string) string {
	return fmt.Sprintf(`{"branch_id":%d,"name":"t","parent_table":%q,"infix_code":"X"}`, branchID, table)
}

// D8 (retro lần 2): Create/Update validate trùng (branch_id, parent_table) trước khi ghi —
// trả 4xx i18n thân thiện, KHÔNG 500 do lỗi UNIQUE thô; sửa chính nó không bị coi là trùng.
func TestConfigCode_DuplicateParentTable(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}
	branch := testhelpers.CreateBranch(t, testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM config_code WHERE branch_id = ?", branch) })
	r := ccRouter(branch, 1)
	suffix := fmt.Sprintf("d8_%d", time.Now().UnixNano())

	w := ccDo(r, http.MethodPost, "/config-code", ccBody(branch, "tbl_a_"+suffix))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = ccDo(r, http.MethodPost, "/config-code", ccBody(branch, "tbl_a_"+suffix))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "trùng (branch, parent_table) phải bị chặn: %s", w.Body.String())

	w = ccDo(r, http.MethodPost, "/config-code", ccBody(branch, "tbl_b_"+suffix))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var idA, idB uint
	require.NoError(t, testDB.Table("config_code").Select("id").Where("branch_id = ? AND parent_table = ?", branch, "tbl_a_"+suffix).Scan(&idA).Error)
	require.NoError(t, testDB.Table("config_code").Select("id").Where("branch_id = ? AND parent_table = ?", branch, "tbl_b_"+suffix).Scan(&idB).Error)

	// Update B sang bảng của A ⇒ trùng.
	w = ccDo(r, http.MethodPut, fmt.Sprintf("/config-code/%d", idB), ccBody(branch, "tbl_a_"+suffix))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())

	// Update A giữ nguyên parent_table (chỉ đổi tên) ⇒ hợp lệ, không tự trùng chính mình.
	w = ccDo(r, http.MethodPut, fmt.Sprintf("/config-code/%d", idA),
		fmt.Sprintf(`{"branch_id":%d,"name":"đổi tên","parent_table":%q,"infix_code":"Y"}`, branch, "tbl_a_"+suffix))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Update B sang bảng mới chưa ai dùng ⇒ ok.
	w = ccDo(r, http.MethodPut, fmt.Sprintf("/config-code/%d", idB), ccBody(branch, "tbl_c_"+suffix))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

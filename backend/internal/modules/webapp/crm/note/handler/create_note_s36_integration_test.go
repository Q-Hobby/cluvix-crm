//go:build integration

package handler

// Story-36/62 (E6/D15): CreateNote không còn khái niệm thương hiệu. FE cũ còn gửi khoá `company_id`
// trong body ⇒ phải bị bỏ qua (không lỗi cột không tồn tại). Khách không tồn tại ⇒ 422.

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
)

func TestCreateNote_BoQuaKhoaThuongHieuCuTuFE(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	content := fmt.Sprintf("S36 note %d", time.Now().UnixNano())
	t.Cleanup(func() { testDB.Exec("DELETE FROM note WHERE customer_id = ?", tn.CustomerID) })

	call := func(body string) int {
		c, w := newNoteCtx(t, "POST", "/crm/note")
		c.Request.Body = io.NopCloser(strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		(&Handler{DB: testDB}).CreateNote(c)
		return w.Code
	}

	// Body FE cũ còn gửi company_id ⇒ vẫn tạo được ghi chú cho khách.
	require.Equal(t, 201, call(fmt.Sprintf(`{"note":%q,"customer_id":%d,"company_id":%d}`, content, tn.CustomerID, 987654)))
	var n int64
	require.NoError(t, testDB.Table("note").Where("customer_id = ? AND note = ?", tn.CustomerID, content).Count(&n).Error)
	require.EqualValues(t, 1, n, "ghi chú phải được tạo")

	// Khách không tồn tại ⇒ 422, không tạo dòng.
	require.Equal(t, 422, call(fmt.Sprintf(`{"note":"x","customer_id":%d}`, 999999999)))
}

// Story-36 (patch QA-C m2, soát 15 chỗ CrossBranchLookup): danh sách ghi chú attach khách CỐ Ý không
// lọc chi nhánh — khách thuộc chi nhánh B vẫn hiện đủ thông tin khi user đang ở chi nhánh A.
func TestGetAllNote_AttachKhachChiNhanhKhac(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	require.NoError(t, testDB.Exec("UPDATE customer SET branch_id = ? WHERE id = ?", branchB, tn.CustomerID).Error)
	noteID := seedNote(t, tn, tn.CustomerID, fmt.Sprintf("S36 attach %d", time.Now().UnixNano()))
	t.Cleanup(func() { testDB.Exec("DELETE FROM note WHERE id = ?", noteID) })

	c, w := newNoteCtx(t, "GET", "/crm/note?per-page=50")
	c.Set("branchActiveId", tn.BranchID)
	c.Set("branchIds", []uint{tn.BranchID})
	(&Handler{DB: testDB}).GetAllNote(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	var found map[string]any
	for _, it := range body.Items {
		if v, ok := it["id"].(float64); ok && uint(v) == noteID {
			found = it
		}
	}
	require.NotNil(t, found, "đối chứng: ghi chú phải có trong list")
	cust, ok := found["customer"].(map[string]any)
	require.True(t, ok && cust != nil, "khách chi nhánh khác phải được attach, nhận: %v", found["customer"])
}

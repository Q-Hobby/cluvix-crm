//go:build integration

// Story-41 (E6/D15) — CHỐT hành vi HIỆN TẠI (kế thừa HEAD/ERP) của GetCustomer + UpdateCustomer
// với khách ở chi nhánh KHÁC chi nhánh của người gọi. KHÔNG đổi hành vi, chỉ neo bằng test để
// retro hỏi user (deferred-work story-36 QA): cả hai đi `ScopedDBNullBranchSkipBranch(..., true)`
// ⇒ user chi nhánh A đọc/sửa được khách chi nhánh B theo id; scope telesale (scope.Apply) của
// GetCustomer vẫn chặn khách không phải của mình. Nếu user quyết chặn theo chi nhánh ⇒ sửa test
// này cùng lúc với handler (đổi expected có giải trình), không xoá.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	customerService "erpCluvix/internal/modules/webapp/crm/customer/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

// s41SeedCustomerInBranch chèn khách ở chi nhánh branchID.
func s41SeedCustomerInBranch(t *testing.T, tenant *testhelpers.TestTenant, branchID uint, paymentUserID *uint) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{
		"branch_id":       branchID,
		"fullname":        "S41 Cross " + strconv.FormatInt(time.Now().UnixNano(), 10),
		"status":          2,
		"payment_user_id": paymentUserID,
		"created_day":     now,
		"created_at":      now,
		"updated_at":      now,
	}
	require.NoError(t, testDB.Table("customer").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM customer WHERE id = ?", id) })
	return id
}

// s41SeedTelesaleUser — user + user_profile thuộc phòng ban Telesale (scope.Apply lọc theo payment_user_id).
func s41SeedTelesaleUser(t *testing.T) uint {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().Unix()
	user := map[string]any{
		"username": "s41tele" + suffix, "password_hash": "x", "status": 10,
		"created_at": now, "updated_at": now,
	}
	require.NoError(t, testDB.Table("user").Create(&user).Error)
	uid := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM user WHERE id = ?", uid) })
	profile := map[string]any{
		"user_id": uid, "fullname": "S41 Telesale " + suffix,
		"departmentList": fmt.Sprintf("[%d]", define.HrDepartmentTelesale),
	}
	require.NoError(t, testDB.Table("user_profile").Create(&profile).Error)
	t.Cleanup(func() { testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid) })
	return uid
}

// s41Ctx — session chỉ có chi nhánh (story-58: session không còn company).
func s41Ctx(w *httptest.ResponseRecorder, branchID, userID uint) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(w)
	c.Set("branchActiveId", branchID)
	c.Set("branchIds", []uint{branchID})
	c.Set("userID", userID)
	return c
}

func s41Get(t *testing.T, branchID, userID, customerID uint) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c := s41Ctx(w, branchID, userID)
	id := strconv.FormatUint(uint64(customerID), 10)
	c.Request = httptest.NewRequest(http.MethodGet, "/crm/customer/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}
	h := &Handler{DB: testDB, CustomerSvc: customerService.NewCustomerService(testDB)}
	h.GetCustomer(c)
	return w
}

func s41Update(t *testing.T, branchID, userID, customerID uint, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c := s41Ctx(w, branchID, userID)
	id := strconv.FormatUint(uint64(customerID), 10)
	c.Request = httptest.NewRequest(http.MethodPut, "/crm/customer/"+id, strings.NewReader(string(raw)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}
	h := &Handler{DB: testDB, CustomerSvc: customerService.NewCustomerService(testDB)}
	h.UpdateCustomer(c)
	return w
}

func TestS41GetCustomer_CrossBranchByID_KeThuaHEAD(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	testDB.Exec("DELETE FROM user_option WHERE id = 5") // OPTION_VIEW_LEAD_OF_TELE OFF
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	custB := s41SeedCustomerInBranch(t, tn, branchB, nil)

	// User thường ở chi nhánh A (không thuộc Telesale) đọc khách chi nhánh B theo id ⇒ 200.
	w := s41Get(t, tn.BranchID, 999999, custB)
	require.Equal(t, http.StatusOK, w.Code,
		"hành vi kế thừa: GetCustomer skipBranch=true ⇒ đọc được khách chi nhánh khác. body=%s", w.Body.String())

	// Scope telesale vẫn áp: telesale chi nhánh A đọc khách chi nhánh B KHÔNG phải của mình ⇒ 404,
	// khách chi nhánh B CỦA MÌNH ⇒ 200.
	tele := s41SeedTelesaleUser(t)
	require.Equal(t, http.StatusNotFound, s41Get(t, tn.BranchID, tele, custB).Code,
		"telesale scope phải chặn khách không phải của mình (kể cả cross-branch)")
	mine := s41SeedCustomerInBranch(t, tn, branchB, &tele)
	require.Equal(t, http.StatusOK, s41Get(t, tn.BranchID, tele, mine).Code,
		"telesale đọc khách của mình ở chi nhánh khác ⇒ 200")
}

func TestS41UpdateCustomer_CrossBranchByID_KeThuaHEAD(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	custB := s41SeedCustomerInBranch(t, tn, branchB, nil)

	var before int64
	require.NoError(t, testDB.Table("customer").Where("id = ?", custB).Pluck("updated_at", &before).Error)

	// User chi nhánh A sửa khách chi nhánh B theo id từ URL ⇒ 200 + DB đổi thật.
	w := s41Update(t, tn.BranchID, 999999, custB, map[string]any{
		"updated_at": before,
		"nickname":   "s41-cross-branch",
	})
	require.Equal(t, http.StatusOK, w.Code,
		"hành vi kế thừa: UpdateCustomer SkipBranch ⇒ sửa được khách chi nhánh khác. body=%s", w.Body.String())
	var nick string
	require.NoError(t, testDB.Table("customer").Where("id = ?", custB).Pluck("nickname", &nick).Error)
	require.Equal(t, "s41-cross-branch", nick)
	var branchAfter uint
	require.NoError(t, testDB.Table("customer").Where("id = ?", custB).Pluck("branch_id", &branchAfter).Error)
	require.Equal(t, branchB, branchAfter, "Update không được tự đổi chi nhánh của khách")

	// Id không tồn tại vẫn 404 (không phải "sửa gì cũng được").
	require.Equal(t, http.StatusNotFound, s41Update(t, tn.BranchID, 999999, 987654321,
		map[string]any{"updated_at": 1, "nickname": "x"}).Code)
}

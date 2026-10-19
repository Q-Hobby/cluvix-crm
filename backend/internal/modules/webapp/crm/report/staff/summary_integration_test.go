//go:build integration

package staff

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
)

func seedRevenue(t *testing.T, tn *testhelpers.TestTenant, branch any, revenueID uint, revenue string, confirm int64) {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{
		"customer_id": tn.CustomerID, "order_plan_id": 1, "order_service_used_id": 1,
		"revenue_id": revenueID, "amount": revenue, "branch_id": branch, "revenue": revenue,
		"created_at": now, "updated_at": now, "confirm_time": confirm,
	}
	require.NoError(t, testDB.Table("order_revenue_telesale").Create(&row).Error)
	id := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM order_revenue_telesale WHERE id = ?", id) })
}

type summaryBody struct {
	Items []struct {
		RevenueID    int64           `json:"revenue_id"`
		Count        int64           `json:"count"`
		TotalRevenue decimal.Decimal `json:"totalRevenue"`
		Telesale     *struct {
			Fullname string `json:"fullname"`
		} `json:"telesale"`
	} `json:"items"`
	TotalRevenue decimal.Decimal `json:"totalRevenue"`
	Meta         struct {
		TotalCount int64 `json:"totalCount"`
	} `json:"_meta"`
}

func callSummary(t *testing.T, branchActive uint, from, to int64) summaryBody {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", fmt.Sprintf(
		"/crm/report/staff/revenue-telesale/summary?filter[confirm_time][gte]=%d&filter[confirm_time][lte]=%d", from, to), nil)
	c.Set("branchActiveId", branchActive)
	c.Set("branchIds", []uint{branchActive})
	(&Handler{DB: testDB}).RevenueTelesaleSummary(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var b summaryBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b), "body: %s", w.Body.String())
	return b
}

// Tổng hợp doanh số: GROUP BY tư vấn viên, sắp theo doanh số giảm dần, tổng = decimal chính xác,
// scope chi nhánh (A + chưa gán, KHÔNG lấy B), lọc khoảng ngày, gắn tên tư vấn viên.
func TestRevenueTelesaleSummary(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	u1 := testhelpers.CreateScopedUser(t, testDB, "ts1", tn, tn.BranchID)
	u2 := testhelpers.CreateScopedUser(t, testDB, "ts2", tn, tn.BranchID)

	day := time.Date(2032, 5, 10, 5, 0, 0, 0, time.UTC).Unix()
	from, to := day-86400, day+86400
	seedRevenue(t, tn, tn.BranchID, u1, "100.10", day)
	seedRevenue(t, tn, nil, u1, "0.20", day) // chưa gán chi nhánh ⇒ vẫn tính
	seedRevenue(t, tn, tn.BranchID, u2, "500.00", day)
	seedRevenue(t, tn, branchB, u2, "999.00", day)              // chi nhánh khác ⇒ loại
	seedRevenue(t, tn, tn.BranchID, u1, "777.00", day+30*86400) // ngoài khoảng ngày ⇒ loại

	b := callSummary(t, tn.BranchID, from, to)
	require.EqualValues(t, 2, b.Meta.TotalCount)
	require.Len(t, b.Items, 2)
	require.EqualValues(t, u2, b.Items[0].RevenueID, "doanh số cao hơn đứng trước")
	require.True(t, decimal.RequireFromString("500").Equal(b.Items[0].TotalRevenue), "u2: %s", b.Items[0].TotalRevenue)
	require.EqualValues(t, u1, b.Items[1].RevenueID)
	require.EqualValues(t, 2, b.Items[1].Count)
	require.True(t, decimal.RequireFromString("100.30").Equal(b.Items[1].TotalRevenue), "u1: %s", b.Items[1].TotalRevenue)
	require.True(t, decimal.RequireFromString("600.30").Equal(b.TotalRevenue), "tổng: %s", b.TotalRevenue)
	require.NotNil(t, b.Items[0].Telesale)
	require.Equal(t, fmt.Sprintf("NV %d", u2), b.Items[0].Telesale.Fullname)

	// Đối chứng: chi nhánh B chỉ thấy dòng của B + dòng chưa gán.
	bb := callSummary(t, branchB, from, to)
	require.True(t, decimal.RequireFromString("999.20").Equal(bb.TotalRevenue), "tổng B: %s", bb.TotalRevenue)
}

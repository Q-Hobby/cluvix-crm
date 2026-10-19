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

// Màn chi tiết hoa hồng: dòng tổng + phân trang + từ khoá (LEFT JOIN customer / order_service_used) chạy đúng SQL.
func TestCommissionTelesaleList_Keyword(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	u := testhelpers.CreateScopedUser(t, testDB, "ct", tn, tn.BranchID)
	name := fmt.Sprintf("KhachStaff%d", time.Now().UnixNano())
	require.NoError(t, testDB.Exec("UPDATE customer SET fullname = ? WHERE id = ?", name, tn.CustomerID).Error)
	other := testhelpers.FreshTenant(t, testDB) // khách khác, cùng chi nhánh A ⇒ không khớp từ khoá
	day := time.Date(2032, 6, 1, 5, 0, 0, 0, time.UTC).Unix()
	for _, r := range []struct {
		cust uint
		com  string
	}{{tn.CustomerID, "10.50"}, {tn.CustomerID, "4.25"}, {other.CustomerID, "99"}} {
		row := map[string]any{"customer_id": r.cust, "order_plan_id": 1, "order_service_used_id": 1,
			"revenue_id": u, "branch_id": tn.BranchID, "amount": "100", "commissionTelesale": r.com, "confirm_time": day}
		require.NoError(t, testDB.Table("order_commission_telesale").Create(&row).Error)
		id := testhelpers.LastInsertID(testDB)
		t.Cleanup(func() { testDB.Exec("DELETE FROM order_commission_telesale WHERE id = ?", id) })
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", fmt.Sprintf(
		"/crm/report/staff/commission-telesale?_keyword=%s&filter[confirm_time][gte]=%d&per-page=1", name, day-10), nil)
	c.Set("branchActiveId", tn.BranchID)
	c.Set("branchIds", []uint{tn.BranchID})
	(&Handler{DB: testDB}).CommissionTelesale(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var b struct {
		Items []struct {
			CustomerID int64 `json:"customer_id"`
			Customer   *struct {
				Fullname string `json:"fullname"`
			} `json:"customer"`
		} `json:"items"`
		Meta struct {
			TotalCount int64 `json:"totalCount"`
		} `json:"_meta"`
		TotalCommission decimal.Decimal `json:"totalCommission"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b), "body: %s", w.Body.String())
	require.EqualValues(t, 2, b.Meta.TotalCount)
	require.Len(t, b.Items, 1, "per-page=1")
	require.NotNil(t, b.Items[0].Customer)
	require.Equal(t, name, b.Items[0].Customer.Fullname)
	require.True(t, decimal.RequireFromString("14.75").Equal(b.TotalCommission), "tổng: %s", b.TotalCommission)
}

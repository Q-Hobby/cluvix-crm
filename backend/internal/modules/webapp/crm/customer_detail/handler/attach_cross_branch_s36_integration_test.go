//go:build integration

package handler

// Story-36 (patch QA-C m2, soát 15 chỗ CrossBranchLookup): danh sách thanh toán của khách attach
// dòng dịch vụ (`orderServiceUsed`) CỐ Ý không lọc chi nhánh — phiếu thu ở chi nhánh A cho dịch vụ
// mua ở chi nhánh B vẫn hiện mã dịch vụ. Đổi về scope có lọc branch ⇒ `orderServiceUsed` rỗng.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
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
	appdb.DB = s.DB // pagination.Paginate đọc db.DB global
	helpers.InitJSONColumnMap(testDB)
	helpers.InitDecimalColumnMap(testDB)
	os.Exit(m.Run())
}

func TestGetPaymentDetailForCustomer_AttachDichVuChiNhanhKhac(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)
	now := time.Now().Unix()
	osu := map[string]any{
		"code": fmt.Sprintf("SV%d", time.Now().UnixNano()), "customer_id": tn.CustomerID,
		"order_plan_id": 1, "service_id": 1, "branch_id": branchB,
		"quantity": 1, "created_at": now, "updated_at": now,
	}
	require.NoError(t, testDB.Table("order_service_used").Create(&osu).Error)
	osuID := testhelpers.LastInsertID(testDB)
	t.Cleanup(func() { testDB.Exec("DELETE FROM order_service_used WHERE id = ?", osuID) })
	payID, err := testhelpers.CreatePayment(testDB, tn, decimal.NewFromInt(100000), define.PaymentForOrder)
	require.NoError(t, err)
	require.NoError(t, testDB.Exec("UPDATE payment SET serviceUsed_id = ? WHERE id = ?", osuID, payID).Error)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", fmt.Sprintf("/crm/payment/detail-for-customer?customer_id=%d", tn.CustomerID), nil)
	c.Set("branchActiveId", tn.BranchID)
	c.Set("branchIds", []uint{tn.BranchID})
	c.Set("userID", uint(1))
	(&Handler{DB: testDB}).GetPaymentDetailForCustomer(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	var found map[string]any
	for _, it := range body.Items {
		if v, ok := it["id"].(float64); ok && uint(v) == payID {
			found = it
		}
	}
	require.NotNil(t, found, "đối chứng: phiếu thu chi nhánh A phải có trong list (body: %s)", w.Body.String())
	sv, ok := found["orderServiceUsed"].(map[string]any)
	require.True(t, ok && sv != nil, "dịch vụ chi nhánh khác phải được attach, nhận: %v", found["orderServiceUsed"])
	require.EqualValues(t, osuID, sv["id"])
}

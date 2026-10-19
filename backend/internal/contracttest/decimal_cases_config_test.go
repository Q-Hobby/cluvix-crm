//go:build integration

package contracttest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	serviceHandler "erpCluvix/internal/modules/webapp/config/service/handler"
	"erpCluvix/internal/testhelpers"
)

// configDecimalCases — case của story-06 pass 2 (QA finding G1): lưới chỉ phủ 1/4 endpoint
// đã vá của cụm ("khác"). #40 `LoadServiceByKeyword` và #41 `LoadService`
// (`webapp/config/service/handler.go`) thêm `helpers.ParseDecimalFieldsForTables(items,
// "service")` ở pass 1 nhưng KHÔNG có case nào giữ — ai xoá dòng đó thì `min_price`/`max_price`/
// `commission_telesale` quay lại string mà suite vẫn xanh.
//
// `service` có 16 cột decimal, JOIN `service_type` có 0 (đối chiếu INFORMATION_SCHEMA thật,
// QA pass 2) ⇒ 1 tên bảng `"service"` là đủ, không có lớp lỗi "thiếu bảng JOIN".
// Cả hai handler cùng shape response (bare array, không bọc `data`) và cùng logic vá — 2 case
// riêng (không gộp làm 1 Call) vì là 2 route độc lập, mutation ở #41 sẽ không bị #40 che.
var configDecimalCases = []DecimalCase{
	{
		Name:     "config_service_load_by_keyword",
		Endpoint: "GET /config/service/load-service-by-keyword",
		Handler:  "webapp/config/service/handler.LoadServiceByKeyword",
		Call:     callConfigLoadServiceByKeyword,
		NumberFields: []string{
			"*.min_price",
			"*.max_price",
			"*.commission_telesale",
		},
	},
	{
		Name:     "config_service_load_service",
		Endpoint: "GET /config/service/load-service",
		Handler:  "webapp/config/service/handler.LoadService",
		Call:     callConfigLoadService,
		NumberFields: []string{
			"*.min_price",
			"*.max_price",
			"*.commission_telesale",
		},
	},
}

func init() { decimalCases = append(decimalCases, configDecimalCases...) }

// seedConfigService seed 1 service_type + 1 service (min_price/max_price/commission_telesale
// khác NULL, khác 0) và trả tenant dùng chung cho cả hai case.
// Tái dùng seedServiceType/seedService (decimal_cases_crm_test.go) — cùng package, không tạo trùng.
func seedConfigService(t *testing.T, db *gorm.DB) (*testhelpers.TestTenant, uint) {
	t.Helper()
	tenant := testhelpers.FreshTenant(t, db)
	serviceTypeID := seedServiceType(t, db, tenant)
	serviceID := seedService(t, db, tenant, serviceTypeID)
	// seedService không set commission_telesale — seed thêm để case chứng minh được cả 3 cột.
	if err := db.Table("service").Where("id = ?", serviceID).
		Update("commission_telesale", "150000.00").Error; err != nil {
		t.Fatalf("seed service.commission_telesale: %v", err)
	}
	return tenant, serviceID
}

func callConfigLoadServiceByKeyword(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()
	tenant, _ := seedConfigService(t, db)

	h := serviceHandler.New(serviceHandler.Deps{DB: db})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/config/service/load-service-by-keyword", nil)
	setCRMScopeContext(c, tenant)

	h.LoadServiceByKeyword(c)
	return w
}

func callConfigLoadService(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()
	tenant, _ := seedConfigService(t, db)

	h := serviceHandler.New(serviceHandler.Deps{DB: db})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/config/service/load-service", nil)
	setCRMScopeContext(c, tenant)

	h.LoadService(c)
	return w
}

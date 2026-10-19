//go:build integration

// story-30 AC3 — LoadServiceByKeyword/LoadService JOIN `service_type` (activeServiceQuery) mà không
// có Select tường minh trước đây → SELECT * scan vào map, cột `name` trùng tên bị bảng JOIN sau
// (`service_type`) ghi đè: response trả `service_type.name` thay vì `service.name`. Lưới này seed 2
// tên PHÂN BIỆT RÕ giữa service/service_type rồi xác nhận response trả đúng tên của `service`.
package contracttest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	serviceHandler "erpCluvix/internal/modules/webapp/config/service/handler"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/helpers"
)

func TestS30LoadService_ReturnsServiceNameNotServiceTypeName(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	serviceTypeID := seedServiceType(t, testDB, tenant)
	serviceID := seedService(t, testDB, tenant, serviceTypeID)

	// seedService/seedServiceType đặt tên khác nhau rõ ràng theo prefix ("Test Service " vs
	// "Test Service Type ") — đủ để phát hiện collision: nếu response trả tên có prefix
	// "Test Service Type" nghĩa là cột bị ghi đè bởi bảng JOIN.
	var wantName string
	if err := testDB.Table("service").Select("name").Where("id = ?", serviceID).Scan(&wantName).Error; err != nil {
		t.Fatalf("đọc lại service.name vừa seed: %v", err)
	}

	h := serviceHandler.New(serviceHandler.Deps{DB: testDB})
	gin.SetMode(gin.TestMode)

	assertRespName := func(endpoint string, call func(c *gin.Context)) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, endpoint, nil)
		setCRMScopeContext(c, tenant)
		call(c)

		if w.Code != http.StatusOK {
			t.Fatalf("%s phải trả 200, nhận HTTP %d\nbody: %s", endpoint, w.Code, w.Body.String())
		}
		var items []map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatalf("%s: parse body lỗi: %v\nbody: %s", endpoint, err, w.Body.String())
		}
		found := false
		for _, item := range items {
			if helpers.ToInt64(item["id"]) == int64(serviceID) {
				found = true
				if item["name"] != wantName {
					t.Fatalf("%s trả nhầm name — muốn %q (service.name), được %q (nghi bị "+
						"service_type.name ghi đè do thiếu Select tường minh)", endpoint, wantName, item["name"])
				}
			}
		}
		if !found {
			t.Fatalf("%s không thấy service vừa seed (id=%d) trong response: %v", endpoint, serviceID, items)
		}
	}

	assertRespName("/config/service/load-by-keyword", func(c *gin.Context) { h.LoadServiceByKeyword(c) })
	assertRespName("/config/service/load-service", func(c *gin.Context) { h.LoadService(c) })
}

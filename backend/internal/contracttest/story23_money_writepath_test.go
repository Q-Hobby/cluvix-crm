//go:build integration

// story-23 AC8 — LockServiceUsed là mục duy nhất của story-23 GHI NGƯỢC vào DB (mở khoá dịch vụ
// cộng discountServiceLock vào totalEnd). Trước khi vá, phép cộng này chạy bằng closure
// `toFloat`/float64 — lưới này seed 2 giá trị lẻ xu (8100000.11 + 50000.22) mà float64 cộng qua
// biểu diễn nhị phân dễ lệch, rồi đọc lại DB xác nhận tổng đúng CHÍNH XÁC tới xu bằng
// decimal.Decimal, không qua trung gian float64.
package contracttest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	treatmentServiceHandler "erpCluvix/internal/modules/webapp/crm/order_service_used/handler"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
)

func TestS23LockServiceUsed_UnlockWritesExactDecimal(t *testing.T) {
	if testDB == nil {
		t.Skip("testDB nil — TEST_MYSQL_DSN chưa set")
	}

	tenant := testhelpers.FreshTenant(t, testDB)
	serviceID := seedService(t, testDB, tenant, 0)
	osuID := seedOrderServiceUsed(t, testDB, tenant, map[string]any{
		"service_id":          serviceID,
		"isLock":              define.ServiceUsedIsLock, // 2 — đang khoá
		"totalEnd":            "8100000.11",
		"discountServiceLock": "50000.22",
		"payment":             "3000000.00",
	})
	id := strconv.FormatUint(uint64(osuID), 10)

	h := treatmentServiceHandler.New(treatmentServiceHandler.Deps{DB: testDB})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/_s23/lock/:id", func(c *gin.Context) {
		c.Set("branchIds", []uint{tenant.BranchID})
		c.Set("branchActiveId", tenant.BranchID)
		c.Set("userID", uint(1))
		h.LockServiceUsed(c)
	})

	// body.isLock=2 (ServiceUsedIsLock) = "đang khoá" → BE làm ngược lại = mở khoá.
	// Khoá lạc quan D6: gửi đúng updated_at đang lưu.
	var updatedAt int64
	if err := testDB.Raw("SELECT updated_at FROM order_service_used WHERE id = ?", osuID).Scan(&updatedAt).Error; err != nil {
		t.Fatalf("đọc updated_at: %v", err)
	}
	body := fmt.Sprintf(`{"isLock": 2, "updated_at": %d}`, updatedAt)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/_s23/lock/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("LockServiceUsed phải trả 200, nhận HTTP %d\nbody: %s", w.Code, w.Body.String())
	}

	var row struct {
		TotalEnd            decimal.Decimal `gorm:"column:totalEnd"`
		DiscountServiceLock decimal.Decimal `gorm:"column:discountServiceLock"`
		IsLock              int             `gorm:"column:isLock"`
	}
	if err := testDB.Table("order_service_used").Select("totalEnd, discountServiceLock, isLock").
		Where("id = ?", osuID).Scan(&row).Error; err != nil {
		t.Fatalf("query lại order_service_used: %v", err)
	}

	wantTotalEnd := decimal.RequireFromString("8150000.33") // 8100000.11 + 50000.22, đúng tới xu
	if !row.TotalEnd.Equal(wantTotalEnd) {
		t.Fatalf("totalEnd ghi sai — muốn %s, được %s (lệch do float64 rounding nếu không dùng decimal.Decimal)",
			wantTotalEnd, row.TotalEnd)
	}
	if !row.DiscountServiceLock.Equal(decimal.Zero) {
		t.Fatalf("discountServiceLock phải về 0 sau mở khoá, được %s", row.DiscountServiceLock)
	}
	if row.IsLock != define.ServiceUsedNotLock {
		t.Fatalf("isLock phải = %d (mở khoá), được %d", define.ServiceUsedNotLock, row.IsLock)
	}
}

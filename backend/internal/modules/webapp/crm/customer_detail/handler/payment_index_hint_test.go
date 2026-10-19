package handler

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Neo cổng của index hint trên GET /crm/payment/get-payment-detail-for-customer (2026-09-09).
// Số đo đầy đủ nằm ở doc của `paymentForceCustomerIdx`. Tóm tắt @605.319 payment, khách đối kháng
// 5.000 payment id CŨ nhất: 600.677 dòng InnoDB -> 358 (1 chi nhánh) và 600.339 -> 20 (toàn chi
// nhánh, ca mà chỉ mục MỘT MÌNH không cứu được). Nhưng có `appointment_id` thì FORCE lại HẠI:
// nhánh đó `idx-payment-appointment_id` chọn lọc hơn (20 dòng), FORCE ép bỏ nó ⇒ 4.072 dòng.
func TestPaymentForceCustomerIdx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		ten  string
		qs   string
		muon bool
	}{
		{"chỉ customer_id (đường FE đi)", "customer_id=51696", true},
		{"có appointment_id ⇒ KHÔNG ép", "customer_id=51696&appointment_id=12345", false},
		{"appointment_id rỗng ⇒ vẫn ép", "customer_id=51696&appointment_id=", true},
		{"có phân trang, không appointment_id", "customer_id=51696&page=100", true},
	}
	for _, tc := range cases {
		t.Run(tc.ten, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/crm/payment/get-payment-detail-for-customer?"+tc.qs, nil)
			if got := paymentForceCustomerIdx(c); got != tc.muon {
				t.Fatalf("paymentForceCustomerIdx = %v, muốn %v (query: %s)", got, tc.muon, tc.qs)
			}
		})
	}
}

// Tên chỉ mục nằm TRONG SQL: đổi tên trong migration mà quên hằng số ⇒ MySQL lỗi 1176 lúc chạy.
// Bắt ở đây rẻ hơn nhiều.
func TestPaymentCustomerIdxKhopVoiMigration(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("đọc thư mục migrations (%s): %v", dir, err)
	}
	found, scanned := false, 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		scanned++
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("đọc %s: %v", e.Name(), err)
		}
		if strings.Contains(string(b), paymentCustomerIdx) {
			found = true
		}
	}
	if scanned < 10 {
		t.Fatalf("chỉ quét được %d file migration — đường dẫn sai, phép kiểm vô nghĩa", scanned)
	}
	if !found {
		t.Fatalf("không migration nào tạo chỉ mục %q mà code đang FORCE tới ⇒ endpoint sẽ lỗi 1176", paymentCustomerIdx)
	}
}

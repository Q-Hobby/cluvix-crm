package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Neo Hard Rule "param id PHẢI parse ra số trước khi vào query".
//
// Trước bản vá, `?idCustomer=777abc` bind dạng CHUỖI ⇒ MySQL ép về 777 (warning 1292 bị nuốt)
// ⇒ trả về lời nhắc của khách 777. Test chạy với DB = nil: đường ĐÚNG phải chặn ở khâu parse và
// trả 422 mà KHÔNG bao giờ chạm tới DB; bỏ khâu parse ⇒ handler đi tiếp vào query trên nil DB
// ⇒ panic ⇒ test đỏ. Đã kiểm bằng mutation (bỏ hẳn lời gọi ParseUint, không phải `if false &&`
// — neutralize kiểu đó vẫn để lại lời gọi parse nên lưới xanh giả).
func TestReminderOfCustomer_NonNumericIDIsRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, raw := range []string{"777abc", "abc", "", "0", "-5", "%20777", "7.7"} {
		t.Run("idCustomer="+raw, func(t *testing.T) {
			h := New(Deps{DB: nil})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/crm/reminder/reminder-of-customer?idCustomer="+raw, nil)

			h.ReminderOfCustomer(c)

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("idCustomer=%q: muốn 422 (chặn trước khi vào query), nhận %d — body: %s",
					raw, w.Code, w.Body.String())
			}
		})
	}
}

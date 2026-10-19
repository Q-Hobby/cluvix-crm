package scope

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// failConnector — mọi lần mở kết nối đều lỗi ⇒ mọi truy vấn của db đều trả error (giả lập DB sập).
type failConnector struct{}

func (failConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("db down")
}
func (failConnector) Driver() driver.Driver { return nil }

// D2 (retro lần 2): đọc user_option/user_profile lỗi ⇒ Apply PHẢI trả error (fail-closed),
// không được trả query không lọc như "không phải Telesale".
func TestApply_DBError_FailClosed(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(failConnector{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("mở DB giả: %v", err)
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/crm/customer", nil)

	q := db.Table("customer")
	got, err := Apply(c, db, q)
	if err == nil {
		t.Fatal("DB lỗi mà Apply không trả error — telesale sẽ thấy hết khách (fail-open)")
	}
	if got != q {
		t.Fatal("khi lỗi phải trả nguyên q, không phải query đã sửa")
	}
}

// S-10: builder không trả error (WhereCustomerID) — đọc phạm vi lỗi ⇒ lỗi PHẢI nằm sẵn trên query (kể cả sau
// khi chain thêm điều kiện) để Find/Scan/Count không chạy, thay vì trả query không lọc.
func TestWhereCustomerID_DBError_ErrorOnQuery(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(failConnector{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("mở DB giả: %v", err)
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/crm/order-plan", nil)

	q := WhereCustomerID(c, db, db.Table("order_plan"), "order_plan.customer_id").Where("order_plan.id > ?", 0)
	if q.Error == nil {
		t.Fatal("DB lỗi mà query không mang lỗi — builder sẽ trả dữ liệu ngoài phạm vi (fail-open)")
	}
	if _, _, err := CustomerIDCond(c, db, "x.customer_id"); err == nil {
		t.Fatal("CustomerIDCond phải trả error khi đọc phạm vi lỗi")
	}
}

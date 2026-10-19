//go:build integration

package contracttest

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	customerHandler "erpCluvix/internal/modules/webapp/crm/customer/handler"
	paymentInDayHandler "erpCluvix/internal/modules/webapp/print/payment_in_day/handler"
	receptionPaymentHandler "erpCluvix/internal/modules/webapp/print/reception_payment/handler"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/response"
)

// decimalCases — BẢNG ĐĂNG KÝ. Story-04..10 APPEND case vào đây khi vá thêm endpoint;
// KHÔNG copy test, KHÔNG sửa harness.
//
// Quy ước seed: mọi cột DECIMAL được assert phải được seed giá trị khác NULL và khác 0
// (harness fail khi gặp null) — nếu không, case "xanh" mà chẳng chứng minh điều gì.
var decimalCases = []DecimalCase{
	{
		Name:     "crm_customer_get_by_id",
		Endpoint: "GET /crm/customer/:id",
		Handler:  "webapp/crm/customer/handler.GetCustomer",
		Call:     callCRMGetCustomer,
		NumberFields: []string{
			"data.totalRevenue",
			"data.totalPayment",
			"data.totalDebt",
			"data.totalDeposit",
			"data.totalBonus",
			"data.consignment",
			"data.refundDeposit",
			"data.refundService",
			"data.refundConsignment",
		},
	},
	{
		// story-08 — cụm export/print. `p.amount` là cột thật của `payment` → đã vá bằng
		// ParseDecimalFieldsForTables(items, "payment") (json_schema.go:95).
		//
		// story-25 (2026-07-30): SELECT sai cột (`p.payment_method`/`p.description` không tồn tại
		// trên bảng `payment` — tên thật `method`/`note`) đã vá + thêm check `.Error` sau `.Find()`.
		// Case này hết Skip, giờ chạy thật.
		Name:     "print_reception_payment",
		Endpoint: "GET /print/reception-payment",
		Handler:  "webapp/print/reception_payment/handler.PrintReceptionPayment",
		Call:     callPrintReceptionPayment,
		NumberFields: []string{
			"object.items.*.amount",
		},
	},
	{
		// story-08 — cụm export/print. `totalDoanhSo/totalThucThu/totalDeposit` là alias của
		// SUM(...) — KHÔNG khớp `ParseDecimalFieldsForTables` theo tên cột. Handler ĐÃ có sẵn
		// `helpers.NormalizeSliceDecimals(items)` (pkg/helpers/decimal.go:103) nhưng đó là NO-OP
		// thật: `.Raw(...).Scan(&items)` trả các alias UNION ALL này dạng Go `string` (không phải
		// `[]uint8`), trong khi NormalizeMapDecimals chỉ bắt nhánh `[]uint8` — case ĐỎ khi mới thêm
		// (đã verify bằng test thật trước khi kết luận, không đoán từ đọc code). Vá bằng
		// `helpers.ParseDecimalFields(item, "totalDoanhSo", "totalThucThu", "totalDeposit")`
		// (helpers.ToDecimal xử lý được cả string lẫn []uint8) — đúng nhánh alias theo architecture.md.
		Name:     "print_payment_in_day",
		Endpoint: "GET /print/payment-in-day",
		Handler:  "webapp/print/payment_in_day/handler.PaymentInDay",
		Call:     callPaymentInDay,
		NumberFields: []string{
			"*.totalThucThu",
		},
	},
}

// TestDecimalFieldsAreJSONNumber — chạy toàn bộ bảng đăng ký.
func TestDecimalFieldsAreJSONNumber(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB không sẵn sàng (set TEST_MYSQL_DSN trỏ vào DB dùng-một-lần)")
	}
	for _, tc := range decimalCases {
		t.Run(tc.Name, func(t *testing.T) {
			runDecimalCase(t, testDB, tc)
		})
	}
}

// TestDecimalHarnessControl — CHỨNG MINH lưới không phải "luôn đỏ".
//
// Dùng một handler giả lặp lại ĐÚNG cách GetCustomer đọc dữ liệu (`.Take` vào
// map[string]interface{} + response.Data), chỉ khác duy nhất ở chỗ CÓ gọi
// `helpers.ParseDecimalFieldsForTables`. Case này phải XANH.
//
// Cặp đỏ/xanh này khoanh chính xác nguyên nhân: khác biệt duy nhất giữa
// TestDecimalFieldsAreJSONNumber/crm_customer_get_by_id (đỏ) và test này (xanh)
// là một lời gọi helper — không phải cấu hình test, không phải seed, không phải
// `decimal.MarshalJSONWithoutQuotes`.
func TestDecimalHarnessControl(t *testing.T) {
	if testDB == nil {
		t.Skip("integration DB không sẵn sàng (set TEST_MYSQL_DSN trỏ vào DB dùng-một-lần)")
	}
	runDecimalCase(t, testDB, DecimalCase{
		Name:     "control_helper_applied",
		Endpoint: "GET (fake) /contracttest/customer/:id",
		Handler:  "contracttest.callPatchedGetCustomer (GetCustomer + ParseDecimalFieldsForTables)",
		Call:     callPatchedGetCustomer,
		NumberFields: []string{
			"data.totalRevenue",
			"data.totalDebt",
		},
	})
}

// callPatchedGetCustomer — bản sao GetCustomer CÓ gọi helper. KHÔNG sửa production code
// (việc vá handler thật là story-04); đây chỉ là đối chứng nằm trong file test.
func callPatchedGetCustomer(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()

	tenant := testhelpers.FreshTenant(t, db)
	seedCustomerMoney(t, db, tenant.CustomerID)
	id := strconv.FormatUint(uint64(tenant.CustomerID), 10)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/contracttest/customer/"+id, nil)

	var item map[string]interface{}
	if err := db.Table("customer").Select("customer.*").Where("customer.id = ?", id).Take(&item).Error; err != nil {
		t.Fatalf("control query: %v", err)
	}
	helpers.ParseJSONFieldForTable(item, "customer")
	helpers.ParseDecimalFieldsForTables([]map[string]interface{}{item}, "customer") // ← khác biệt DUY NHẤT
	response.Data(c, item)
	return w
}

// ---------------------------------------------------------------------------
// Case: GET /crm/customer/:id
// ---------------------------------------------------------------------------

// callCRMGetCustomer seed 1 customer có đủ giá trị tiền khác 0 rồi gọi thẳng
// handler.GetCustomer. GetCustomer scope theo phạm vi chi nhánh của người gọi.
// nên context test phải set key đó.
func callCRMGetCustomer(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()

	tenant := testhelpers.FreshTenant(t, db)
	seedCustomerMoney(t, db, tenant.CustomerID)

	h := customerHandler.New(customerHandler.Deps{DB: db})
	id := strconv.FormatUint(uint64(tenant.CustomerID), 10)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/crm/customer/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Set("branchIds", []uint{tenant.BranchID})
	c.Set("branchActiveId", tenant.BranchID)
	c.Set("userID", uint(1))

	h.GetCustomer(c)
	return w
}

// seedCustomerMoney đặt giá trị khác 0 cho mọi cột DECIMAL của `customer` mà case này assert.
// Giá trị đặt qua raw SQL literal (không qua decimal.Decimal) để chắc chắn đường đi kiểm chứng
// là DB → driver → handler, giống hệt runtime thật.
func seedCustomerMoney(t *testing.T, db *gorm.DB, customerID uint) {
	t.Helper()
	now := time.Now().Unix()
	if err := db.Table("customer").Where("id = ?", customerID).Updates(map[string]any{
		"totalRevenue":      "640000000.00",
		"totalPayment":      "500000000.00",
		"totalDebt":         "140000000.00",
		"totalDeposit":      "20000000.00",
		"totalBonus":        "1500000.00",
		"consignment":       "3000000.50",
		"refundDeposit":     "1000000.00",
		"refundService":     "250000.25",
		"refundConsignment": "125000.75",
		"updated_at":        now,
	}).Error; err != nil {
		t.Fatalf("seed customer money: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Case: GET /print/reception-payment  (story-08)
// ---------------------------------------------------------------------------

// callPrintReceptionPayment seed 1 payment rồi gọi thẳng handler.PrintReceptionPayment.
// Handler scope tenant qua ScopedDBNullBranch("payment") — context phải set branchActiveId
// + branchActiveId để khớp tenant vừa seed.
func callPrintReceptionPayment(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()

	tenant := testhelpers.FreshTenant(t, db)
	seedReceptionPayment(t, db, tenant)

	h := receptionPaymentHandler.New(receptionPaymentHandler.Deps{DB: db})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/print/reception-payment", nil)
	c.Set("branchIds", []uint{tenant.BranchID})
	c.Set("branchActiveId", tenant.BranchID)
	c.Set("userID", uint(1))

	h.PrintReceptionPayment(c)
	return w
}

// seedReceptionPayment tạo 1 payment với amount khác 0 khác NULL.
func seedReceptionPayment(t *testing.T, db *gorm.DB, tenant *testhelpers.TestTenant) {
	t.Helper()
	now := time.Now().Unix()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":         "RCP" + suffix[:10],
		"customer_id":  tenant.CustomerID,
		"method":       1,
		"amount":       "3500000.00",
		"payment_for":  define.PaymentForOrder,
		"payment_time": now,
		"created_at":   now,
		"updated_at":   now,
	})
	if err := db.Table("payment").Create(&row).Error; err != nil {
		t.Fatalf("seed reception payment: %v", err)
	}
	id := testhelpers.LastInsertID(db)
	t.Cleanup(func() { db.Exec("DELETE FROM payment WHERE id = ?", id) })
}

// ---------------------------------------------------------------------------
// Case: GET /print/payment-in-day  (story-08)
// ---------------------------------------------------------------------------

// callPaymentInDay seed 1 payment (payment_for=PaymentForOrder, deposit_type mặc định 0
// != PaymentTypeDepositKG) rồi gọi thẳng handler.PaymentInDay. Handler lọc theo
// tenantSQLCond nội bộ (story-41: không còn vế thương hiệu; không dùng ScopedDB*), không lọc branch.
func callPaymentInDay(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()

	tenant := testhelpers.FreshTenant(t, db)
	seedReceptionPayment(t, db, tenant) // amount 3.500.000, payment_for=Order → nuôi bucket totalThucThu

	h := paymentInDayHandler.New(paymentInDayHandler.Deps{DB: db})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/print/payment-in-day", nil)

	h.PaymentInDay(c)
	return w
}

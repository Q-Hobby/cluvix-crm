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

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/pagination"
)

// paginationDecimalCases — case của story-16 (lớp lỗi thứ 4: `pagination.extractTables`).
//
// File RIÊNG (append qua init()) thay vì sửa `decimal_cases_test.go`: file đó dùng chung nhiều
// story chạy song song — pattern đã dùng ở decimal_cases_crm_test.go / _loyalty_test.go.
//
// HAI lỗi được canh, cả hai đều nằm trong `extractTables` (pkg/pagination/pagination.go):
//
//  1. Bảng JOIN không được phủ: `extractTables` trích tên bảng JOIN từ `Statement.SQL`, nhưng
//     GORM v1.31.1 reset `stmt.SQL` sau execute (callbacks.go:150) ⇒ nhánh đó là DEAD CODE ⇒
//     `Paginate` chỉ parse decimal của BẢNG CHÍNH. Cột DECIMAL **thật** của bảng JOIN, chiếu
//     ĐÚNG TÊN (không alias), ra JSON string.
//
//  2. Bảng chính đặt ALIAS không được phủ: `Table("payment p")` ⇒ GORM `tableRegexp` đặt
//     `Statement.Table = "p"` (alias, không phải tên bảng) ⇒ ngay cả cột của bảng CHÍNH cũng
//     không được parse.
//
// Vì sao dùng handler dựng trong test thay vì endpoint thật:
// endpoint thật duy nhất dính lỗi (1) là `GET /accountant/payment/report-telesale`, nhưng nó
// đang HỎNG SẴN từ trước story này (SELECT `order_commission_telesale.telesale_id` +
// `.source_id` — 2 cột đã bị rename/drop ở migration 20260621161526 ⇒ MySQL 1054 ⇒ list luôn
// rỗng). Case cho endpoint đó vẫn được đăng ký nhưng Skip, để khi bug kia được vá thì bật lên
// là có lưới ngay (xem `paginationSkippedCases`).
var paginationDecimalCases = []DecimalCase{
	{
		Name:     "pagination_join_table_decimal_column",
		Endpoint: "GET /_contracttest/pagination/join-decimal (handler dựng trong test)",
		Handler:  "pkg/pagination.Paginate → extractTables (nhánh bảng JOIN)",
		Call:     callPaginateJoinDecimal,
		NumberFields: []string{
			// payment.deposit_after — cột THẬT của bảng JOIN, chiếu đúng tên. Đây là field
			// DUY NHẤT thực sự đỏ-trước-xanh-sau: `amount` cũng là tên cột DECIMAL của bảng
			// CHÍNH (order_commission_telesale.amount) nên đã được phủ sẵn nhờ VA CHẠM TÊN —
			// giữ lại để canh regression, KHÔNG dùng làm bằng chứng.
			"items.*.amount",
			"items.*.deposit_after",
			// ĐỐI CHỨNG: cột của bảng CHÍNH — luôn number kể cả trước khi vá. Nếu field này
			// cũng đỏ thì lỗi ở cấu hình test (decimal.MarshalJSONWithoutQuotes), không phải
			// ở extractTables.
			"items.*.commissionTelesale",
		},
	},
	{
		Name:     "pagination_main_table_alias",
		Endpoint: "GET /_contracttest/pagination/main-alias (handler dựng trong test)",
		Handler:  "pkg/pagination.Paginate → extractTables (nhánh bảng chính có alias)",
		Call:     callPaginateMainTableAlias,
		NumberFields: []string{
			"items.*.amount",
			"items.*.deposit_after",
		},
	},
}

// paginationSkippedCases (case cho endpoint THẬT `GET /accountant/payment/report-telesale`) XOÁ ở
// story-07: handler thuộc module `accountant` (đã xoá cùng module). Hai case generic bên trên vẫn
// canh đúng lớp lỗi `extractTables` qua raw SQL (không phụ thuộc module đã xoá).
func init() {
	decimalCases = append(decimalCases, paginationDecimalCases...)
}

// ---------------------------------------------------------------------------
// (1) bảng JOIN — lặp đúng hình dạng query của ReportTelesale
// ---------------------------------------------------------------------------

func callPaginateJoinDecimal(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()

	tenant := testhelpers.FreshTenant(t, db)
	paymentID := seedTelesalePayment(t, db, tenant)
	seedOrderCommissionTelesale(t, db, tenant, paymentID)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/_contracttest/pagination/join-decimal", nil)
	setCRMScopeContext(c, tenant)

	q := db.Table("order_commission_telesale").
		Joins("LEFT JOIN payment ON payment.id = order_commission_telesale.payment_id").
		Where("order_commission_telesale.payment_id = ?", paymentID).
		Select(`order_commission_telesale.id, order_commission_telesale.commissionTelesale,
			payment.amount, payment.deposit_after`)

	var items []map[string]interface{}
	result := pagination.Paginate(c, q, &items)
	c.JSON(http.StatusOK, result)
	return w
}

// ---------------------------------------------------------------------------
// (2) bảng chính có ALIAS — `Table("payment p")` ⇒ Statement.Table = "p"
// ---------------------------------------------------------------------------

func callPaginateMainTableAlias(t *testing.T, db *gorm.DB) *httptest.ResponseRecorder {
	t.Helper()

	tenant := testhelpers.FreshTenant(t, db)
	paymentID := seedTelesalePayment(t, db, tenant)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/_contracttest/pagination/main-alias", nil)
	setCRMScopeContext(c, tenant)

	q := db.Table("payment p").
		Where("p.id = ?", paymentID).
		Select("p.id, p.amount, p.deposit_after")

	var items []map[string]interface{}
	result := pagination.Paginate(c, q, &items)
	c.JSON(http.StatusOK, result)
	return w
}

// ---------------------------------------------------------------------------
// Seed
// ---------------------------------------------------------------------------

// seedTelesalePayment tạo 1 payment với amount + deposit_after khác NULL và khác 0.
// Giá trị đặt bằng string literal để đường đi đúng runtime: DB → driver ([]uint8) → handler.
func seedTelesalePayment(t *testing.T, db *gorm.DB, tenant *testhelpers.TestTenant) uint {
	t.Helper()
	now := time.Now().Unix()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"code":            "PMT" + suffix[:10],
		"customer_id":     tenant.CustomerID,
		"method":          1,
		"payment_for":     define.PaymentForOrder,
		"amount":          "7400000.00",
		"deposit_after":   "1250000.00",
		"payment_time":    now,
		"payment_of_date": now,
		"created_at":      now,
		"updated_at":      now,
	})
	if err := db.Table("payment").Create(&row).Error; err != nil {
		t.Fatalf("seed payment: %v", err)
	}
	id := testhelpers.LastInsertID(db)
	t.Cleanup(func() { db.Exec("DELETE FROM payment WHERE id = ?", id) })
	return id
}

func seedOrderCommissionTelesale(t *testing.T, db *gorm.DB, tenant *testhelpers.TestTenant, paymentID uint) uint {
	t.Helper()

	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"customer_id":           tenant.CustomerID,
		"order_plan_id":         1,
		"order_service_used_id": 1,
		"payment_id":            paymentID,
		"revenue_id":            1,
		"amount":                "7400000.00",
		"commissionTelesale":    "370000.00",
	})
	if err := db.Table("order_commission_telesale").Create(&row).Error; err != nil {
		t.Fatalf("seed order_commission_telesale: %v", err)
	}
	id := testhelpers.LastInsertID(db)
	t.Cleanup(func() { db.Exec("DELETE FROM order_commission_telesale WHERE id = ?", id) })
	return id
}

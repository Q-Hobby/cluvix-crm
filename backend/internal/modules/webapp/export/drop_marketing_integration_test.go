//go:build integration

package export

// User chốt 2026-10-18: bỏ hẳn field Marketing (`customer.marketing_id`, `appointment.marketing_id`)
// — migration 20261018152846 xoá 2 cột. Export lịch hẹn / khách hàng (convert) / phiếu thu từng
// JOIN `user_profile mkt ON mkt.user_id = <bảng>.marketing_id`: còn sót 1 chỗ là export lỗi 1054
// "Unknown column" trên DB đã migrate. Test chạy THẬT từng export trên schema đủ migration (có dữ
// liệu ⇒ đi qua COUNT + đọc dòng), rồi mở file xlsx kiểm dòng header không còn cột "Marketing".

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"erpCluvix/internal/testhelpers"
	pkgExport "erpCluvix/pkg/export"
)

// exportHeaders chạy fn (1 hàm Export*) trong thư mục tạm, trả dòng header (dòng 1) của file xlsx.
func exportHeaders(t *testing.T, fn func(c *gin.Context)) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/x?_export=1", nil)
	c.Set("userID", uint(1))
	fn(c)
	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var url string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &url), "body: %s", w.Body.String())

	// File nằm trong thư mục của người tạo (userID 1 ở trên) — xem pkg/export.Config.OwnerID.
	path, ok := pkgExport.OwnerFilePath(pkgExport.DefaultDir, 1, filepath.Base(url))
	require.True(t, ok, "url=%s", url)
	f, err := excelize.OpenFile(path)
	require.NoError(t, err, "url=%s", url)
	defer func() { _ = f.Close() }()
	rows, err := f.GetRows("Sheet1")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	return rows[0]
}

func TestExport_BoMarketing_KhongConCot(t *testing.T) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		t.Fatalf("TEST_MYSQL_DSN chưa set/không dựng được schema — integration test này KHÔNG được skip: %v", err)
	}
	defer s.Close()
	db := s.DB
	SetDB(db)
	t.Cleanup(func() { SetDB(nil) })
	t.Chdir(t.TempDir()) // ExportDir "./storage/export/" — không ghi file vào cây source

	var cols int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME IN ('customer','appointment') AND COLUMN_NAME = 'marketing_id'`).Scan(&cols).Error)
	require.Zero(t, cols, "migration phải đã xoá customer/appointment.marketing_id")

	tn := testhelpers.FreshTenant(t, db)
	now := time.Now().Unix()
	appt := testhelpers.WithTenantCols(tn, map[string]any{
		"code": fmt.Sprintf("LH%d", time.Now().UnixNano()), "customer_id": tn.CustomerID,
		"appointment_date": now, "status": 1, "created_at": now, "updated_at": now,
	})
	require.NoError(t, db.Table("appointment").Create(&appt).Error)
	apptID := testhelpers.LastInsertID(db)
	t.Cleanup(func() { db.Exec("DELETE FROM appointment WHERE id = ?", apptID) })
	payID, err := testhelpers.CreatePayment(db, tn, decimal.NewFromInt(100000), 4)
	require.NoError(t, err)

	noMarketing := func(name string, headers []string) {
		require.NotEmpty(t, headers, name)
		for _, h := range headers {
			require.NotEqual(t, "Marketing", h, "%s: header còn cột Marketing: %v", name, headers)
		}
	}

	noMarketing("appointment", exportHeaders(t, func(c *gin.Context) {
		ExportAppointment(c, db.Table("appointment").
			Joins("LEFT JOIN customer ON customer.id = appointment.customer_id").
			Where("appointment.id = ?", apptID))
	}))
	noMarketing("customer convert", exportHeaders(t, func(c *gin.Context) {
		ExportCustomerConvert(c, db.Table("customer").Where("customer.id = ?", tn.CustomerID))
	}))
	paymentQ := func() *gorm.DB {
		return db.Table("payment").
			Joins("LEFT JOIN customer ON customer.id = payment.customer_id").
			Joins("LEFT JOIN order_plan ON order_plan.id = payment.parent_id").
			Where("payment.id = ?", payID)
	}
	noMarketing("payment receipts", exportHeaders(t, func(c *gin.Context) { ExportPaymentReceipts(c, paymentQ()) }))
	noMarketing("payment", exportHeaders(t, func(c *gin.Context) { ExportPayment(c, paymentQ()) }))

}

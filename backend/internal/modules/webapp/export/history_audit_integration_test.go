//go:build integration

package export

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	pkgExport "erpCluvix/pkg/export"
)

// AC8 (story-05) — `history_export.quantityRecord` phải bằng ĐÚNG số dòng dữ liệu trong file, không
// phải giá trị câu COUNT.
//
// Test nằm trong CÙNG package nên gọi được `doExport` (unexported) — đúng cái seam nối
// `pkg/export.Export` với việc ghi audit. Test ở tầng `pkg/export` chỉ chứng minh `OnSuccess` nhận
// đúng số; nó KHÔNG chứng minh số đó chảy tới bảng audit.
//
// Chạy khi có `EXPORT_BENCH_DSN` trỏ DB dùng-một-lần (đuôi `_test`/`_tmp`).
func TestHistoryExport_QuantityRecord_KhopSoDongTrongFile(t *testing.T) {
	dsn := os.Getenv("EXPORT_BENCH_DSN")
	if dsn == "" {
		t.Skip("không có EXPORT_BENCH_DSN — bỏ qua")
	}
	if !strings.Contains(dsn, "_test") && !strings.Contains(dsn, "_tmp") {
		t.Fatalf("DSN phải trỏ DB dùng-một-lần: %q", dsn)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("mở DB: %v", err)
	}
	sqlDB, _ := db.DB()
	defer func() { _ = sqlDB.Close() }()

	SetDB(db)
	t.Cleanup(func() { SetDB(nil) })

	const position = "AC8_audit_probe"
	if err := db.Exec("DELETE FROM history_export WHERE positionExport = ?", position).Error; err != nil {
		t.Fatalf("dọn bản ghi cũ: %v", err)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM history_export WHERE positionExport = ?", position) })

	// Query nhỏ, số dòng biết trước và KHÁC hẳn mọi số khác trong bài để không nhầm.
	const wantRows = 137
	q := db.Table("order_service_used").
		Select("order_service_used.id").
		Where("order_service_used.isLock = 1").
		Where("order_service_used.id <= ?", wantRows)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/export?_export=1", nil)
	c.Set("userID", uint(1))

	dir := t.TempDir()
	url, err := doExport(c, q, pkgExport.Config{
		FileName:    position,
		Headers:     []string{"ID"},
		ExportDir:   dir + "/",
		DownloadURL: "https://example.test",
	}, func(r map[string]interface{}) []interface{} {
		return []interface{}{r["id"]}
	})
	if err != nil {
		t.Fatalf("doExport lỗi: %v", err)
	}
	if url == "" {
		t.Fatal("doExport không trả URL")
	}

	var got struct {
		QuantityRecord int `gorm:"column:quantityRecord"`
	}
	if err := db.Table("history_export").
		Select("quantityRecord").
		Where("positionExport = ?", position).
		Order("id DESC").Limit(1).Scan(&got).Error; err != nil {
		t.Fatalf("đọc history_export: %v", err)
	}
	if got.QuantityRecord != wantRows {
		t.Fatalf("history_export.quantityRecord = %d, muốn %d (số dòng THẬT trong file)",
			got.QuantityRecord, wantRows)
	}
	t.Logf("AC8 OK: history_export.quantityRecord = %d, khớp số dòng dữ liệu trong file", got.QuantityRecord)
}

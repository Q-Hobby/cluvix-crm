//go:build integration

// Story-40 (E6/D15): múi giờ lấy từ `pkg/brand` (nguồn duy nhất; story-62: bảng `business_profile`).
//
// Thay 2 test cũ (story-09) `TestLocation_QueryError_DoesNotPoisonCache` /
// `TestLocation_EmptyTimezone_IsCached`: chúng đo cache THEO THƯƠNG HIỆU riêng của package này — cache đó
// đã bỏ. Bất biến "lỗi DB KHÔNG cache, giá trị thật được cache" nay nằm ở `pkg/brand` và giữ bằng
// `pkg/brand` TestLoiDB_KhongCache + TestDocDuField_VaCache; ở đây kiểm lại CHÍNH đường tz (lỗi ⇒ mặc
// định cho lời gọi đó, lần sau đọc được giá trị thật; timezone rỗng ⇒ mặc định).
//
// ⛔ Chỉ chạy trên DB DÙNG-MỘT-LẦN (`TEST_MYSQL_DSN`); testhelpers.New DROP mọi bảng.
package tz_test

import (
	"log"
	"os"
	"testing"
	"time"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/tz"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	os.Exit(m.Run())
}

func setBrandTZ(t *testing.T, timezone string) {
	t.Helper()
	if testDB == nil {
		t.Skip("no test DB")
	}
	require.NoError(t, testDB.Exec("DELETE FROM business_profile").Error)
	require.NoError(t, testDB.Exec("INSERT INTO business_profile (name, timezone) VALUES ('S40', ?)", timezone).Error)
	tz.InvalidateAll()
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM business_profile")
		tz.InvalidateAll()
	})
}

func openBroken(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.Open(os.Getenv("TEST_MYSQL_DSN")), &gorm.Config{Logger: gormlogger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

func TestHamTz_TheoMuiGioThuongHieu(t *testing.T) {
	setBrandTZ(t, "Asia/Tokyo")
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)

	require.Equal(t, "Asia/Tokyo", tz.Location(testDB).String())
	require.Equal(t, "Asia/Tokyo", tz.Now(testDB).Location().String())
	require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, tokyo).Unix(), tz.MonthStart(testDB, 2026, 3))
	require.Equal(t, time.Date(2026, 3, 31, 23, 59, 59, 0, tokyo).Unix(), tz.MonthEnd(testDB, 2026, 3))
	// 2026-03-31 15:30 UTC = 2026-04-01 00:30 Tokyo — lệch ngày nếu đọc sai múi giờ.
	u := time.Date(2026, 3, 31, 15, 30, 0, 0, time.UTC).Unix()
	require.Equal(t, "2026-04-01", tz.Format(testDB, u, "2006-01-02"))
	require.Equal(t, time.Now().In(tokyo).Format("0601"), tz.YearMonth(testDB))
	require.Equal(t, brand.Location(testDB).String(), tz.Location(testDB).String(), "tz và brand phải cùng nguồn")
}

func TestLocation_LoiDB_MacDinhKhongCache(t *testing.T) {
	setBrandTZ(t, "Asia/Tokyo")
	broken := openBroken(t)

	require.Equal(t, tz.DefaultTZ, tz.Location(broken).String(), "lỗi DB ⇒ mặc định cho lời gọi này")
	require.Equal(t, "Asia/Tokyo", tz.Location(testDB).String(), "lần lỗi trước KHÔNG được cache")
	require.Equal(t, "Asia/Tokyo", tz.Location(broken).String(), "giá trị thật đã vào cache")
}

func TestLocation_TimezoneRong_MacDinh(t *testing.T) {
	setBrandTZ(t, "")
	require.Equal(t, tz.DefaultTZ, tz.Location(testDB).String())
}

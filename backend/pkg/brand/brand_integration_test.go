//go:build integration

// pkg/brand trên schema thật (`backend/migrations`, bảng 1 dòng `business_profile` — story-62; trước
// đó story-35 đọc bảng thương hiệu cũ).
// ⛔ Chỉ chạy trên DB DÙNG-MỘT-LẦN (`TEST_MYSQL_DSN`); testhelpers.New DROP mọi bảng.
package brand_test

import (
	"errors"
	"log"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/tz"
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

func needDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("no test DB")
	}
	require.NoError(t, testDB.Exec("DELETE FROM business_profile").Error)
	brand.Invalidate()
	t.Cleanup(func() {
		testDB.Exec("DELETE FROM business_profile")
		brand.Invalidate()
	})
}

func insertProfile(t *testing.T, cols map[string]any) uint {
	t.Helper()
	require.NoError(t, testDB.Table("business_profile").Create(&cols).Error)
	var id uint
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error)
	require.NotZero(t, id)
	return id
}

func TestChuaCoDong_TraMacDinh(t *testing.T) {
	needDB(t)

	require.Equal(t, brand.Defaults(), brand.Profile(testDB))
	require.Equal(t, "Asia/Ho_Chi_Minh", brand.Location(testDB).String())
	require.Equal(t, "", brand.Code(testDB))
	p := brand.Profile(testDB)
	require.Equal(t, "VND", p.Currency)
	require.Equal(t, "₫", p.CurrencySymbol)
	require.Equal(t, "vi-VN", p.CurrencyLocale)
	require.Equal(t, 30, p.TotpTrustDays)

	require.False(t, p.Require2FA, "chưa có dòng ⇒ không bắt 2FA")
	require.Empty(t, p.AllowedIPs, "chưa có dòng ⇒ không giới hạn IP")
	strict, err := brand.ProfileStrict(testDB)
	require.NoError(t, err, "chưa có dòng KHÔNG phải lỗi (policy = mặc định)")
	require.Equal(t, brand.Defaults(), strict)

	// "Chưa có dòng" KHÔNG được cache: onboarding tạo dòng xong phải thấy ngay.
	insertProfile(t, map[string]any{"name": "Brand S62", "code": "S62", "timezone": "Asia/Tokyo"})
	require.Equal(t, "S62", brand.Code(testDB))
	require.Equal(t, "Asia/Tokyo", brand.Location(testDB).String())
}

func TestDocDuField_VaCache(t *testing.T) {
	needDB(t)
	id := insertProfile(t, map[string]any{
		"name": "Brand S35", "code": "BS35", "tax_code": "0101234567",
		"hotline": "1900", "address": "1 Lê Lợi", "email": "a@b.vn", "website": "b.vn",
		"logo_image": "logo.png", "timezone": "Asia/Tokyo",
		"currency": "USD", "currency_symbol": "$", "currency_locale": "en-US",
		"weekly_rest_days": "0,6", "allowed_ips": `["10.0.0.0/8"]`, "require_2fa": 1,
		"require_2fa_roles": `[1,2]`, "totp_trust_days": 0,
	})

	p := brand.Profile(testDB)
	require.Equal(t, "Brand S35", p.Name)
	require.Equal(t, "BS35", p.Code)
	require.Equal(t, "1900", p.Hotline)
	require.Equal(t, "1 Lê Lợi", p.Address)
	require.Equal(t, "a@b.vn", p.Email)
	require.Equal(t, "b.vn", p.Website)
	require.Equal(t, "logo.png", p.LogoImage)
	require.Equal(t, "0101234567", p.TaxCode)
	require.Equal(t, "Asia/Tokyo", p.Timezone)
	require.Equal(t, "USD", p.Currency)
	require.Equal(t, "$", p.CurrencySymbol)
	require.Equal(t, "en-US", p.CurrencyLocale)
	require.Equal(t, "0,6", p.WeeklyRestDays)
	require.JSONEq(t, `["10.0.0.0/8"]`, string(p.AllowedIPs))
	require.True(t, p.Require2FA)
	require.JSONEq(t, `[1,2]`, string(p.Require2FARoles))
	require.Equal(t, 0, p.TotpTrustDays, "0 = luôn hỏi 2FA — giá trị hợp lệ, không được thay bằng mặc định 30")
	require.Equal(t, "BS35", brand.Code(testDB))
	require.Equal(t, "Asia/Tokyo", brand.Location(testDB).String())

	// Cache: đổi DB ⇒ vẫn giá trị cũ cho tới Invalidate.
	require.NoError(t, testDB.Exec("UPDATE business_profile SET code = 'NEW', timezone = 'Europe/Paris' WHERE id = ?", id).Error)
	require.Equal(t, "BS35", brand.Code(testDB), "giá trị đọc được phải vào cache")
	brand.Invalidate()
	require.Equal(t, "NEW", brand.Code(testDB))
	require.Equal(t, "Europe/Paris", brand.Location(testDB).String())

	// tz.InvalidateAll cũng xoá cache brand.
	require.NoError(t, testDB.Exec("UPDATE business_profile SET timezone = 'Asia/Seoul' WHERE id = ?", id).Error)
	tz.InvalidateAll()
	require.Equal(t, "Asia/Seoul", brand.Location(testDB).String())
}

func TestNhieuDong_LayIDNhoNhat(t *testing.T) {
	needDB(t)
	insertProfile(t, map[string]any{"name": "Brand A", "code": "A"})
	insertProfile(t, map[string]any{"name": "Brand B", "code": "B"})
	require.Equal(t, "A", brand.Code(testDB), "lỡ có nhiều dòng ⇒ luôn lấy id nhỏ nhất (tất định)")
}

// Lỗi DB ⇒ mặc định cho lời gọi đó (Profile/Location), KHÔNG cache; ProfileStrict trả lỗi.
func TestLoiDB_KhongCache(t *testing.T) {
	needDB(t)
	insertProfile(t, map[string]any{"name": "Brand S35", "timezone": "Asia/Tokyo", "code": "S35"})

	broken, err := gorm.Open(mysql.Open(os.Getenv("TEST_MYSQL_DSN")), &gorm.Config{Logger: gormlogger.Discard})
	require.NoError(t, err)
	sqlDB, err := broken.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.Equal(t, brand.Defaults(), brand.Profile(broken))
	require.Equal(t, "Asia/Ho_Chi_Minh", brand.Location(broken).String())
	_, lerr := brand.ProfileStrict(broken)
	require.Error(t, lerr, "lỗi DB ⇒ ProfileStrict PHẢI trả lỗi (policy bảo mật không được fail-open)")
	require.False(t, errors.Is(lerr, brand.ErrNoProfile))

	require.Equal(t, "Asia/Tokyo", brand.Location(testDB).String(), "lỗi lần trước KHÔNG được cache")
	// Giờ đã cache ⇒ kết nối hỏng vẫn trả giá trị thật.
	require.Equal(t, "S35", brand.Code(broken))
}

// tz.LocFromContext + helpers.FormatAmount/FormatDecimal đọc qua brand (hồ sơ doanh nghiệp).
func TestTzVaHelpers_DocQuaBrand(t *testing.T) {
	needDB(t)
	id := insertProfile(t, map[string]any{"name": "Brand S35", "timezone": "Asia/Tokyo",
		"currency_locale": "en-US", "currency_symbol": "$"})

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.Equal(t, "Asia/Tokyo", tz.LocFromContext(c, testDB).String())

	require.Equal(t, "$ 1,500,000", helpers.FormatAmount(testDB, 1500000))
	require.Equal(t, "$ 1,500,000", helpers.FormatDecimal(testDB, decimal.NewFromInt(1500000)))

	require.NoError(t, testDB.Exec("UPDATE business_profile SET currency_locale = 'vi-VN', currency_symbol = '₫' WHERE id = ?", id).Error)
	brand.Invalidate()
	require.Equal(t, "1.500.000 ₫", helpers.FormatAmount(testDB, 1500000))
}

// Cache an toàn đồng thời (chạy kèm -race ở story-35: `go test -race -p 1 -tags integration ./pkg/brand/`).
func TestDongThoi(t *testing.T) {
	needDB(t)
	insertProfile(t, map[string]any{"name": "Brand S35", "code": "S35"})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if (i+j)%7 == 0 {
					brand.Invalidate()
				}
				if got := brand.Code(testDB); got != "S35" {
					t.Errorf("Code = %q", got)
					return
				}
				_ = brand.Location(testDB)
			}
		}(i)
	}
	wg.Wait()
}

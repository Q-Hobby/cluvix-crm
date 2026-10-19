package brand

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type nopConn struct{}

func (nopConn) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dry run: không mở kết nối")
}
func (nopConn) Driver() driver.Driver { return nil }

// DryRun: query không chạy ⇒ không có dòng ⇒ mặc định (không cache).
func TestDryRun_KhongCoDong(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sql.OpenDB(nopConn{}), SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	Invalidate()
	if got := Profile(db); got.Timezone != DefaultTimezone || got.Currency != DefaultCurrency ||
		got.CurrencySymbol != DefaultCurrencySymbol || got.CurrencyLocale != DefaultCurrencyLocale ||
		got.TotpTrustDays != DefaultTotpTrustDays {
		t.Errorf("Profile mặc định sai: %+v", got)
	}
	if got := Location(db).String(); got != DefaultTimezone {
		t.Errorf("Location = %s", got)
	}
	if got, err := ProfileStrict(db); err != nil || got.Require2FA || len(got.AllowedIPs) != 0 ||
		got.TotpTrustDays != DefaultTotpTrustDays {
		t.Errorf("ProfileStrict không có dòng phải ra mặc định không lỗi: %+v, %v", got, err)
	}
	mu.RLock()
	defer mu.RUnlock()
	if cached != nil {
		t.Error("không có dòng thì KHÔNG được cache")
	}
}

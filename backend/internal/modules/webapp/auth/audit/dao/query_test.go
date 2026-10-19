package dao

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// nopConnector — DryRun: GORM dựng SQL rồi dừng, không bao giờ mở kết nối thật.
type nopConnector struct{}

func (nopConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dry run: test này không được phép mở kết nối")
}
func (nopConnector) Driver() driver.Driver { return nil }

// Story-58/62: nhật ký bảo mật không có phạm vi thương hiệu (bảng không còn cột đó); bộ lọc
// (user/event/khoảng thời gian) + tiebreaker ORDER BY giữ nguyên.
func TestAuditQuery_S58_BoLocUserEventThoiGian(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sql.OpenDB(nopConnector{}), SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	stmt := (&Dao{DB: db}).Query(AuditFilter{UserID: 7, Events: []string{"login_success"}, From: 1, To: 2}).
		Find(&rows).Statement
	sqlStr := stmt.SQL.String()
	for _, want := range []string{"user_id = ?", "event IN (?)", "created_at >= ?", "created_at <= ?", "ORDER BY created_at DESC, id DESC"} {
		if !strings.Contains(sqlStr, want) {
			t.Errorf("SQL thiếu %q: %s", want, sqlStr)
		}
	}
}

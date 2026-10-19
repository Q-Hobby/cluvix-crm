package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Neo phạm vi tenant của GET /crm/report/service-type-statistics (2026-09-09).
//
// Bug đã vá: `ScopedDBNullBranch(c, db, "osu")` truyền ALIAS làm tên bảng ⇒
// `tenantTables["osu"]` = false ⇒ helper trả nguyên db, KHÔNG thêm vế nào, và KHÔNG báo lỗi.
// Báo cáo vì thế trả dữ liệu mọi chi nhánh bất kể quyền người xem — build xanh, test cũ xanh,
// màn hình vẫn ra số. Test này chạy CHÍNH `buildServiceTypeStatsQuery` (không dựng lại query
// trong test — bản dựng lại sẽ xanh kể cả khi handler mất scope).
//
// GORM DryRun: dựng SQL rồi dừng ⇒ không đụng database nào.

type nopConnStats struct{}

func (nopConnStats) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dry run: test này không được phép mở kết nối")
}
func (nopConnStats) Driver() driver.Driver { return nil }

func dryRunDBStats(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(nopConnStats{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("mở DryRun DB: %v", err)
	}
	return db
}

func statsCtx(t *testing.T, branchActive uint) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET",
		"/crm/report/service-type-statistics?filter[created_day][gte]=1788195600&filter[created_day][lte]=1788973199", nil)
	c.Set("branchActiveId", branchActive)
	c.Set("branchIds", []uint{1, 2, 3})
	return c
}

func statsSQL(t *testing.T, h *Handler, c *gin.Context) string {
	t.Helper()
	var rows []map[string]interface{}
	return h.buildServiceTypeStatsQuery(c).Select("st.id").Find(&rows).Statement.SQL.String()
}

func TestServiceTypeStats_QueryCoScopeTenant(t *testing.T) {
	h := New(Deps{DB: dryRunDBStats(t)})

	sqlText := statsSQL(t, h, statsCtx(t, 1))
	// Story-35 (D15 — 1 thương hiệu/DB): thân ScopedDBNullBranch chỉ còn vế branch.
	for _, want := range []string{"osu.branch_id", "osu.created_day"} {
		if !strings.Contains(sqlText, want) {
			t.Fatalf("SQL thiếu %q — scope/bộ lọc bị bỏ im lặng.\nSQL: %s", want, sqlText)
		}
	}
}

// Đối chứng: chứng minh phép kiểm CÓ hiệu lực, và ghi lại đúng cơ chế đã gây bug —
// truyền alias làm tên bảng thì helper im lặng không thêm vế nào.
func TestServiceTypeStats_AliasLamTenBangThiMatScope(t *testing.T) {
	db := dryRunDBStats(t)
	c := statsCtx(t, 1)

	var rows []map[string]interface{}
	sai := tenant.ScopedDBNullBranch(c, db, "osu").
		Table("order_service_used osu").Select("osu.id").Find(&rows).Statement.SQL.String()
	if strings.Contains(sai, "branch_id") {
		t.Fatalf("bản SAI đáng lẽ KHÔNG có vế scope nào — phép kiểm vô hiệu.\nSQL: %s", sai)
	}

	rows = nil
	dung := tenant.ScopedDBNullBranch(c, db, "order_service_used", "osu").
		Table("order_service_used osu").Select("osu.id").Find(&rows).Statement.SQL.String()
	// Story-35 (D15): bản ĐÚNG chỉ còn vế branch.
	if !strings.Contains(dung, "osu.branch_id") {
		t.Fatalf("bản ĐÚNG phải có vế scope branch.\nSQL: %s", dung)
	}
}

// branchActiveId = 0 (xem toàn bộ chi nhánh) vẫn phải bị chặn theo danh sách branchIds được cấp,
// không phải bỏ trắng bộ lọc.
func TestServiceTypeStats_ToanChiNhanhVanTheoQuyen(t *testing.T) {
	h := New(Deps{DB: dryRunDBStats(t)})
	sqlText := statsSQL(t, h, statsCtx(t, 0))
	if !strings.Contains(sqlText, "osu.branch_id IN") {
		t.Fatalf("branchActive=0 phải lọc theo branchIds được cấp.\nSQL: %s", sqlText)
	}
}

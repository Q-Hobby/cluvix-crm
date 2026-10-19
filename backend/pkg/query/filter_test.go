package query

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"erpCluvix/pkg/logger"
)

// newDryRunDB tạo *gorm.DB ở chế độ DryRun — build SQL không cần DB thật.
// SkipInitializeWithVersion=true để driver không gọi "SELECT VERSION()" lúc Open
// (tránh phải dial 1 kết nối MySQL thật).
// TestMain gắn nop logger MỘT LẦN rồi trả lại giá trị cũ: `logSortRejected` ghi qua
// `logger.L`, biến này nil trong unit test (chưa qua logger.Init) nên nhánh sort bị reject sẽ
// panic. Set trong helper rồi bỏ đó sẽ để lại global đã bị ghi đè cho mọi test chạy sau, che
// mất lỗi nil-logger thật.
func TestMain(m *testing.M) {
	prev := logger.L
	logger.L = zap.NewNop()
	code := m.Run()
	logger.L = prev
	os.Exit(code)
}

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DriverName:                "mysql",
		DSN:                       "user:pass@tcp(127.0.0.1:3306)/testdb?parseTime=true",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("newDryRunDB: %v", err)
	}
	return db
}

// newTestContext tạo *gin.Context với query params cho URL truyền vào.
func newTestContext(rawQuery string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?"+rawQuery, nil)
	return c
}

func buildSQL(t *testing.T, db *gorm.DB, table string, apply func(q *gorm.DB) *gorm.DB) (string, []interface{}) {
	t.Helper()
	var rows []struct {
		ID int `gorm:"column:id"`
	}
	q := db.Table(table)
	q = apply(q)
	stmt := q.Find(&rows).Statement
	return stmt.SQL.String(), stmt.Vars
}

// AC3(a): neq sinh đúng chuỗi có "OR ... IS NULL" + đúng bind args.
func TestApplyFilter_Neq_GeneratesOrIsNull(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[status][neq]=3")

	sql, vars := buildSQL(t, db, "orders", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "status", "status")
	})

	if !strings.Contains(sql, "(status != ? OR status IS NULL)") {
		t.Fatalf("expected neq SQL to contain '(status != ? OR status IS NULL)', got: %s", sql)
	}
	if len(vars) != 1 || vars[0] != "3" {
		t.Fatalf("expected bind args [3], got: %v", vars)
	}
}

// AC3(a): neq với column có prefix bảng (alias) vẫn bọc ngoặc đúng, không phá cú pháp.
func TestApplyFilter_Neq_WithTablePrefix(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[status][neq]=3")

	sql, vars := buildSQL(t, db, "orders o", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c, q, "status", "o.status")
	})

	if !strings.Contains(sql, "(o.status != ? OR o.status IS NULL)") {
		t.Fatalf("expected neq SQL to contain '(o.status != ? OR o.status IS NULL)', got: %s", sql)
	}
	if len(vars) != 1 || vars[0] != "3" {
		t.Fatalf("expected bind args [3], got: %v", vars)
	}
}

// AC3(b): eq/gt không đổi hành vi — vẫn sinh "column = ?" / "column > ?" đơn giản, không bọc ngoặc OR IS NULL.
func TestApplyFilter_EqGt_Unchanged(t *testing.T) {
	db := newDryRunDB(t)

	c1 := newTestContext("filter[status][eq]=1")
	sql1, vars1 := buildSQL(t, db, "orders", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c1, q, "status", "status")
	})
	if !strings.Contains(sql1, "status = ?") || strings.Contains(sql1, "IS NULL") {
		t.Fatalf("expected eq SQL 'status = ?' without IS NULL, got: %s", sql1)
	}
	if len(vars1) != 1 || vars1[0] != "1" {
		t.Fatalf("expected bind args [1], got: %v", vars1)
	}

	c2 := newTestContext("filter[amount][gt]=100")
	sql2, vars2 := buildSQL(t, db, "orders", func(q *gorm.DB) *gorm.DB {
		return ApplyFilter(c2, q, "amount", "amount")
	})
	if !strings.Contains(sql2, "amount > ?") || strings.Contains(sql2, "IS NULL") {
		t.Fatalf("expected gt SQL 'amount > ?' without IS NULL, got: %s", sql2)
	}
	if len(vars2) != 1 || vars2[0] != "100" {
		t.Fatalf("expected bind args [100], got: %v", vars2)
	}
}

// AC3(c): neq kết hợp nhiều field (neq + eq) trên cùng query vẫn đúng, không lẫn lộn.
func TestApplyFilter_Neq_CombinedWithOtherField(t *testing.T) {
	db := newDryRunDB(t)
	c := newTestContext("filter[status][neq]=3&filter[branchId][eq]=5")

	sql, vars := buildSQL(t, db, "orders", func(q *gorm.DB) *gorm.DB {
		q = ApplyFilter(c, q, "status", "status")
		q = ApplyFilter(c, q, "branchId", "branchId")
		return q
	})

	if !strings.Contains(sql, "(status != ? OR status IS NULL)") {
		t.Fatalf("expected neq fragment present, got: %s", sql)
	}
	if !strings.Contains(sql, "branchId = ?") {
		t.Fatalf("expected eq fragment present, got: %s", sql)
	}
	if len(vars) != 2 {
		t.Fatalf("expected 2 bind args, got: %v", vars)
	}
}

// --- ApplySort: tiebreaker ổn định cho LIMIT/OFFSET ---
// Sort theo cột không unique mà không có tiebreaker → thứ tự không xác định giữa các row
// bằng nhau → export (LIMIT/OFFSET theo batch 5000) lặp/mất dòng ở ranh giới batch.

func sortSQL(t *testing.T, rawQuery string, apply func(c *gin.Context, q *gorm.DB) *gorm.DB) string {
	t.Helper()
	db := newDryRunDB(t)
	c := newTestContext(rawQuery)
	sql, _ := buildSQL(t, db, "appointment", func(q *gorm.DB) *gorm.DB { return apply(c, q) })
	return sql
}

func assertOrderBy(t *testing.T, sql, want string) {
	t.Helper()
	idx := strings.Index(sql, "ORDER BY ")
	if idx < 0 {
		t.Fatalf("SQL không có ORDER BY: %s", sql)
	}
	got := strings.TrimSpace(sql[idx+len("ORDER BY "):])
	if got != want {
		t.Errorf("ORDER BY = %q, want %q\nSQL: %s", got, want, sql)
	}
}

// Cột không unique → phải kèm pk bảng gốc.
func TestApplySort_AppendsTiebreaker(t *testing.T) {
	sql := sortSQL(t, "sort=appointment_date", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.id DESC", "appointment.appointment_date")
	})
	assertOrderBy(t, sql, "appointment.appointment_date ASC, appointment.id DESC")
}

// Field đã LÀ pk bảng gốc → không nhân đôi.
func TestApplySort_NoDuplicateTiebreakerWhenSortingByPK(t *testing.T) {
	sql := sortSQL(t, "sort=-id", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.id DESC", "appointment.id")
	})
	assertOrderBy(t, sql, "appointment.id DESC")
}

// Field trỏ bảng JOIN → tiebreaker vẫn phải là pk bảng GỐC, không phải customer.id.
func TestApplySort_TiebreakerUsesBaseTableNotJoinedTable(t *testing.T) {
	sql := sortSQL(t, "sort=customer.fullname", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.id DESC", "customer.fullname")
	})
	assertOrderBy(t, sql, "customer.fullname ASC, appointment.id DESC")
}

// defaultOrder theo alias tổng hợp (không suy được bảng) → không thêm gì.
func TestApplySort_NoTiebreakerWhenDefaultOrderHasNoTablePrefix(t *testing.T) {
	sql := sortSQL(t, "sort=-records", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "records DESC", "records")
	})
	assertOrderBy(t, sql, "records DESC")
}

// Sort bị reject (ngoài whitelist) → giữ NGUYÊN defaultOrder, không chèn tiebreaker.
func TestApplySort_RejectedKeepsDefaultOrderUntouched(t *testing.T) {
	sql := sortSQL(t, "sort=hacker_field", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.id DESC", "appointment.appointment_date")
	})
	assertOrderBy(t, sql, "appointment.id DESC")
}

// KHÔNG có sort param là ca THƯỜNG GẶP NHẤT (mở list lần đầu, mọi vòng export theo batch) —
// defaultOrder không unique ở đây cũng phải được phá hoà, y như đường có sort param.
// Trước 2026-09-06 test này khẳng định điều ngược lại ("defaultOrder nguyên vẹn") nên giữ suite
// xanh xuyên qua đúng cái bug mất-dòng-khi-phân-trang mà tiebreaker sinh ra để chặn.
func TestApplySort_NoSortParamAppendsTiebreaker(t *testing.T) {
	sql := sortSQL(t, "", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.appointment_date DESC", "appointment.appointment_date")
	})
	assertOrderBy(t, sql, "appointment.appointment_date DESC, appointment.id DESC")
}

// Không sort param + defaultOrder ĐÃ là pk → không nhân đôi.
func TestApplySort_NoSortParamNoDuplicateWhenDefaultIsPK(t *testing.T) {
	sql := sortSQL(t, "", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.id DESC", "appointment.id")
	})
	assertOrderBy(t, sql, "appointment.id DESC")
}

// Không sort param + pk nằm ở vế SAU của defaultOrder → vẫn không nhân đôi.
func TestApplySort_NoSortParamNoDuplicateWhenPKIsSecondTerm(t *testing.T) {
	sql := sortSQL(t, "", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.appointment_date DESC, appointment.id ASC", "appointment.id")
	})
	assertOrderBy(t, sql, "appointment.appointment_date DESC, appointment.id ASC")
}

// Vế trùng TIỀN TỐ chứ không trùng cột (`appointment.id_old` chứa chuỗi `appointment.id`)
// → vẫn PHẢI thêm tiebreaker. Đây là ca làm hỏng lưới nếu so khớp bằng substring.
func TestApplySort_NoSortParamPrefixCollisionStillAppends(t *testing.T) {
	sql := sortSQL(t, "", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.id_old DESC", "appointment.id_old")
	})
	assertOrderBy(t, sql, "appointment.id_old DESC, appointment.id DESC")
}

// Không sort param + defaultOrder theo alias tổng hợp → không suy được bảng ⇒ giữ nguyên.
// Đây là lưới chặn lỗi 1055 cho query GROUP BY không-theo-pk (emr/report).
func TestApplySort_NoSortParamNoTiebreakerWithoutTablePrefix(t *testing.T) {
	sql := sortSQL(t, "", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "records DESC", "records")
	})
	assertOrderBy(t, sql, "records DESC")
}

// Sort bị reject → rơi về defaultOrder, và defaultOrder đó cũng phải có tiebreaker.
func TestApplySort_RejectedDefaultOrderAlsoGetsTiebreaker(t *testing.T) {
	sql := sortSQL(t, "sort=hacker_field", func(c *gin.Context, q *gorm.DB) *gorm.DB {
		return ApplySort(c, q, "appointment.appointment_date DESC", "appointment.appointment_date")
	})
	assertOrderBy(t, sql, "appointment.appointment_date DESC, appointment.id DESC")
}

package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// nopConnector — connector không bao giờ mở kết nối thật. `sql.OpenDB` LƯỜI (không dial lúc gọi),
// GORM ở DryRun dựng SQL rồi dừng ⇒ test chạy mọi máy/CI mà KHÔNG đụng database nào.
type nopConnector struct{}

func (nopConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("dry run: test này không được phép mở kết nối")
}
func (nopConnector) Driver() driver.Driver { return nil }

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sql.OpenDB(nopConnector{}),
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("mở DryRun DB: %v", err)
	}
	// DryRun không đọc được phạm vi telesale (S-10) ⇒ coi như user không bị giới hạn; phạm vi telesale
	// được khoá ở test tích hợp crm/customer/integration.
	orig := reminderScopeCond
	reminderScopeCond = func(*gin.Context, *gorm.DB, string) (string, []any, error) { return "", nil, nil }
	t.Cleanup(func() { reminderScopeCond = orig })
	return db
}

func listCtx(t *testing.T, rawQuery string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/crm/reminder?"+rawQuery, nil)
	return c
}

// Các helper dưới đây chạy CHÍNH những hàm dựng query mà ListReminder dùng — không dựng lại query
// trong test. Bản đầu của file này tự dựng lại và vì thế KHÔNG bắt được mutation nào: cấy lại
// GROUP BY, ép join vô điều kiện, bỏ scope tenant — cả ba đều test vẫn XANH.
//
// ⚠ Ở DryRun, các truy vấn tra id của đường ngram (Pluck) KHÔNG chạy thật ⇒ luôn trả danh sách
// rỗng ⇒ đường nhanh kết thúc bằng `1 = 0`. Đó CHÍNH LÀ hành vi cần khoá (rỗng phải là 0 dòng,
// không phải "bỏ qua điều kiện"), nên DryRun là chỗ đo đúng, không phải hạn chế.
func countSQL(t *testing.T, h *Handler, c *gin.Context) string {
	t.Helper()
	var total int64
	return h.buildReminderCountQuery(c).Count(&total).Statement.SQL.String()
}

func pageIDSQL(t *testing.T, h *Handler, c *gin.Context) string {
	t.Helper()
	var ids []int64
	return h.buildReminderPageIDQuery(c).Limit(20).Pluck("reminder.id", &ids).Statement.SQL.String()
}

func rowSQL(t *testing.T, h *Handler, c *gin.Context) string {
	t.Helper()
	var items []map[string]interface{}
	return h.buildReminderRowQuery(c, []int64{3, 2, 1}).Find(&items).Statement.SQL.String()
}

// ⚠ ĐÃ THỬ VÀ BỎ — đừng thêm lại, cả hai XANH kể cả khi cấy lỗi thật:
//   - "COUNT không mang ORDER BY": GORM `.Count()` tự bỏ ORDER BY.
//   - "COUNT không mang danh sách cột của câu danh sách": `.Count()` cũng tự thay Select bằng
//     `count(*)`, nên cho buildReminderCountQuery trả thẳng query danh sách vẫn test xanh.
//
// Cả hai tính chất đó do GORM bảo đảm, không phải do code này — assert chúng chỉ tạo cảm giác an
// toàn giả.

// GROUP BY reminder.id là chi phí thuần: join `customer.id = reminder.customer_id` đi vào KHOÁ
// CHÍNH nên không thể nhân dòng. Nó ép MySQL dựng "Temporary table with deduplication" trên toàn
// bộ tập khớp (471.860 dòng ở bench 524k) trước khi sort + LIMIT 20 — 5,07-6,08s cho trang 1.
func TestListQueriesHaveNoGroupBy(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	for _, qs := range []string{"", "_keyword=Nguyen", "filter[status]=1"} {
		c := listCtx(t, qs)
		for name, sql := range map[string]string{"pageID": pageIDSQL(t, h, c), "row": rowSQL(t, h, c)} {
			if strings.Contains(strings.ToUpper(sql), "GROUP BY") {
				t.Errorf("query %s (%q) có GROUP BY — xem khối chú thích đầu find.go:\n%s", name, qs, sql)
			}
		}
	}
}

// COUNT chỉ được join `customer` khi có điều kiện thật sự ĐỌC cột customer. MySQL KHÔNG tự loại
// LEFT JOIN thừa, nên join vô điều kiện là trả giá join trên MỌI lần load.
func TestCountQueryJoinsCustomerOnlyWhenNeeded(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})

	for _, qs := range []string{"", "filter[status]=1", "filter[type]=2", "filter[customer_id]=9", "page=1000"} {
		sql := countSQL(t, h, listCtx(t, qs))
		if strings.Contains(sql, "JOIN customer") {
			t.Errorf("COUNT (%q) join customer mà không có điều kiện nào cần:\n%s", qs, sql)
		}
		if !strings.Contains(strings.ToUpper(sql), "COUNT(") {
			t.Errorf("COUNT (%q) không phải câu đếm:\n%s", qs, sql)
		}
	}
}

// Từ khoá ngram-AN TOÀN đi đường nhanh: lọc bằng danh sách id, KHÔNG còn `LIKE '%kw%'` xuyên join
// (nguyên nhân quét toàn bảng 4,4-7,7s ở 524k dòng).
func TestSafeKeywordUsesIDListNotLikeAcrossJoin(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	sql := countSQL(t, h, listCtx(t, "_keyword=Nguyen"))

	if strings.Contains(sql, "customer.fullname LIKE") || strings.Contains(sql, "reminder.note LIKE") {
		t.Errorf("từ khoá an toàn vẫn đi đường LIKE xuyên join:\n%s", sql)
	}
	if strings.Contains(sql, "JOIN customer") {
		t.Errorf("đường nhanh không cần join customer nhưng vẫn join:\n%s", sql)
	}
}

// ⚠ Ca nguy hiểm nhất của cả bản vá: đường nhanh không tìm được id nào thì PHẢI trả 0 dòng.
// Bỏ qua điều kiện = trả về NGUYÊN danh sách như thể không tìm gì — sai im lặng, không lỗi.
func TestEmptyKeywordMatchReturnsNoRowsNotEverything(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	sql := countSQL(t, h, listCtx(t, "_keyword=zzqqxx"))
	if !strings.Contains(sql, "1 = 0") {
		t.Errorf("từ khoá không khớp id nào mà không có `1 = 0` ⇒ sẽ trả toàn bộ danh sách:\n%s", sql)
	}
}

// Từ khoá ngram KHÔNG an toàn (có "từ" 1 ký tự) phải RƠI VỀ đường LIKE cũ. Đi ngram ở ca này
// cho MATCH = 0 trong khi LIKE trả hàng trăm dòng — sót sạch, không lỗi, không cảnh báo.
func TestUnsafeKeywordFallsBackToLikeWithJoin(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	sql := countSQL(t, h, listCtx(t, "_keyword=Van+A"))

	if !strings.Contains(sql, "JOIN customer") {
		t.Errorf("đường lui LIKE tham chiếu customer.* nhưng KHÔNG join ⇒ Unknown column:\n%s", sql)
	}
	if !strings.Contains(sql, "customer.fullname LIKE") {
		t.Errorf("từ khoá ngram-không-an-toàn phải rơi về LIKE:\n%s", sql)
	}
}

// `_groupCountry` trỏ tới nhóm rỗng ⇒ PHẢI trả 0 dòng. Bản cũ bỏ qua luôn điều kiện, tức người
// dùng lọc "nhóm X" lại nhận nguyên danh sách.
func TestEmptyGroupCountryReturnsNoRows(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	sql := countSQL(t, h, listCtx(t, "_groupCountry[]=7"))
	if !strings.Contains(sql, "1 = 0") {
		t.Errorf("nhóm quốc gia rỗng mà không có `1 = 0` ⇒ trả toàn bộ danh sách:\n%s", sql)
	}
}

// Bước 2 của deferred join lọc bằng `IN (danh sách id)` nên KHÔNG tự có thứ tự — MySQL không có
// nghĩa vụ giữ thứ tự của mệnh đề IN. Thiếu ORDER BY = trả đúng bộ dòng nhưng sai thứ tự.
func TestRowQueryOrdersExplicitly(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	sql := rowSQL(t, h, listCtx(t, ""))
	if !strings.Contains(strings.ToUpper(sql), "ORDER BY") {
		t.Errorf("query nạp dữ liệu thiếu ORDER BY:\n%s", sql)
	}
	if !strings.Contains(sql, "customer_fullname") {
		t.Errorf("query nạp dữ liệu thiếu cột customer mà FE cần:\n%s", sql)
	}
}

// Scope tenant ở CẢ BA đường sau khi tách phải NHẤT QUÁN.
//
// Story-35/62 (D15 — 1 thương hiệu/DB): `reminder` KHÔNG có cột branch_id (không nằm branchTables —
// canary TestWhitelist_NoDrift xác nhận trên schema) ⇒ scope của bảng này không sinh vế nào; test chỉ
// còn neo cả 3 câu cùng truy vấn bảng reminder (story-62 bỏ phép kiểm cột thương hiệu: cột đó không còn
// trong schema, tái xuất hiện sẽ nổ 1054 ở mọi integration test của module).
func TestAllQueriesKeepTenantScope(t *testing.T) {
	h := New(Deps{DB: dryRunDB(t)})
	c := listCtx(t, "")
	for name, sql := range map[string]string{
		"count":  countSQL(t, h, c),
		"pageID": pageIDSQL(t, h, c),
		"row":    rowSQL(t, h, c),
	} {
		if !strings.Contains(sql, "reminder") {
			t.Fatalf("câu %s không truy vấn bảng reminder — phép kiểm vô hiệu:\n%s", name, sql)
		}
	}
}

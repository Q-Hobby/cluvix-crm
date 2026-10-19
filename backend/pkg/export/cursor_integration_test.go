//go:build integration

package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sqlRecorder ghi lại MỌI câu SQL gửi xuống MySQL, và (tuỳ chọn) chạy `afterCount` ngay sau khi câu
// COUNT chạy xong — dùng để dựng ca "dữ liệu đổi giữa COUNT và lúc mở con trỏ".
type sqlRecorder struct {
	logger.Interface
	mu         sync.Mutex
	stmts      []string
	afterCount func()
	fired      bool
}

func newRecorder() *sqlRecorder { return &sqlRecorder{Interface: logger.Discard} }

func (r *sqlRecorder) LogMode(logger.LogLevel) logger.Interface { return r }

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.mu.Lock()
	r.stmts = append(r.stmts, sql)
	fire := false
	if r.afterCount != nil && !r.fired && strings.Contains(sql, "count_q") {
		r.fired, fire = true, true
	}
	r.mu.Unlock()
	if fire {
		r.afterCount()
	}
}

// dataStmts trả về các câu SQL LẤY DỮ LIỆU (bỏ câu COUNT và các câu dựng fixture).
func (r *sqlRecorder) dataStmts() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, s := range r.stmts {
		if strings.Contains(s, "count_q") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "SELECT") {
			out = append(out, s)
		}
	}
	return out
}

func dbWithLogger(t *testing.T, rec *sqlRecorder, maxConns int) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("testDB khác nil nghĩa là TEST_MYSQL_DSN đã được đặt")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: rec})
	if err != nil {
		t.Fatalf("mở DB: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("lấy sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(maxConns)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func plainMapper(r map[string]interface{}) []interface{} {
	return []interface{}{r["id"], r["code"], r["fullname"], r["amount"], r["created_at"], r["payload"]}
}

func baseCfg(dir, name string) Config {
	return Config{
		FileName:    name,
		Headers:     streamHeaders(),
		ExportDir:   dir + "/",
		DownloadURL: "https://example.test",
	}
}

// ---------------------------------------------------------------------------
// AC3 — pkg/export KHÔNG được thêm mệnh đề nào vào câu lấy dữ liệu
// ---------------------------------------------------------------------------

func TestExport_DataQuery_KhongThemLimitOffsetOrderBy(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	const table = "export_cursor_clause_test"
	mkStreamTable(t, testDB, table, 60)

	rec := newRecorder()
	db := dbWithLogger(t, rec, 4)

	dir := t.TempDir()
	if _, err := Export(streamQuery(db, table), baseCfg(dir, "clause"), plainMapper); err != nil {
		t.Fatalf("Export lỗi: %v", err)
	}

	stmts := rec.dataStmts()
	if len(stmts) == 0 {
		t.Fatal("không bắt được câu SQL lấy dữ liệu nào — recorder hỏng, kết luận vô nghĩa")
	}
	// Đúng MỘT câu lấy dữ liệu: cơ chế cũ sinh ⌈N/5000⌉ câu.
	if len(stmts) != 1 {
		t.Errorf("có %d câu lấy dữ liệu, muốn đúng 1 (một-lần-quét):\n%s", len(stmts), strings.Join(stmts, "\n"))
	}
	for _, s := range stmts {
		up := strings.ToUpper(s)
		for _, bad := range []string{"LIMIT", "OFFSET", "ORDER BY"} {
			if strings.Contains(up, bad) {
				t.Errorf("pkg/export đã thêm %q vào câu lấy dữ liệu — cấm tuyệt đối "+
					"(thứ tự dòng không được đổi; ORDER BY + JOIN làm mất early-exit). SQL:\n%s", bad, s)
			}
		}
		if strings.Contains(up, "BATCH_Q") {
			t.Errorf("còn bọc derived table batch_q — cơ chế cũ chưa bị gỡ hết. SQL:\n%s", s)
		}
	}
}

// ---------------------------------------------------------------------------
// AC4 — COUNT vẫn quyết định định dạng, đúng ở CẢ HAI ca biên
// ---------------------------------------------------------------------------

func TestExport_NguongDinhDang_CaBien(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	for _, tc := range []struct {
		rows int
		want string
	}{
		{9999, ".xlsx"}, // dưới CSVThreshold
		{10000, ".csv"}, // đúng ngưỡng
	} {
		t.Run(fmt.Sprintf("%d dòng → %s", tc.rows, tc.want), func(t *testing.T) {
			table := fmt.Sprintf("export_cursor_threshold_%d_test", tc.rows)
			mkStreamTable(t, testDB, table, tc.rows)
			dir := t.TempDir()
			url, err := Export(streamQuery(testDB, table), baseCfg(dir, "threshold"), plainMapper)
			if err != nil {
				t.Fatalf("Export lỗi: %v", err)
			}
			if got := filepath.Ext(url); got != tc.want {
				t.Fatalf("%d dòng ra %q, muốn %s — hành vi người dùng thấy đã đổi", tc.rows, got, tc.want)
			}
			if n := len(rowsFromFile(t, dir, url)); n != tc.rows {
				t.Fatalf("file có %d dòng, muốn %d", n, tc.rows)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC5 — 0 dòng: trả lỗi cũ, KHÔNG tạo file
// ---------------------------------------------------------------------------

func TestExport_KhongCoDuLieu(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	const table = "export_cursor_empty_test"
	mkStreamTable(t, testDB, table, 5)
	exec(t, testDB, "UPDATE "+table+" SET active = 0")

	dir := t.TempDir()
	called := false
	cfg := baseCfg(dir, "empty")
	cfg.OnSuccess = func(int64) { called = true }

	url, err := Export(streamQuery(testDB, table), cfg, plainMapper)
	if err == nil {
		t.Fatal("muốn lỗi khi không có dữ liệu")
	}
	if err.Error() != "Không có dữ liệu để export" {
		t.Errorf("thông báo lỗi đổi: %q — người dùng đang thấy chuỗi này", err.Error())
	}
	if url != "" {
		t.Errorf("trả URL %q dù lỗi", url)
	}
	if called {
		t.Error("OnSuccess bị gọi dù export thất bại")
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 0 {
		t.Errorf("tạo ra %d file dù không có dữ liệu", len(ents))
	}
}

// ---------------------------------------------------------------------------
// AC6 — OnSuccess nhận SỐ DÒNG THẬT ĐÃ GHI, không phải COUNT
// ---------------------------------------------------------------------------

// Dựng ca COUNT ≠ dữ liệu thật: xoá vài dòng NGAY SAU khi câu COUNT chạy, trước khi mở con trỏ.
// File phải theo dữ liệu thật, KHÔNG cắt bớt cũng không đệm thêm cho khớp COUNT — cắt/đệm chính là
// tái tạo lại lỗi bỏ-sót-dòng mà thay đổi này sinh ra để diệt.
func TestExport_OnSuccess_NhanSoDongThat(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	const (
		table   = "export_cursor_onsuccess_test"
		total   = 200
		removed = 7
	)
	mkStreamTable(t, testDB, table, total)
	db2 := openSecondPool(t)

	rec := newRecorder()
	rec.afterCount = func() {
		if err := db2.Exec(fmt.Sprintf(
			"UPDATE %s SET active = 0 WHERE id <= %d", table, removed)).Error; err != nil {
			t.Errorf("xoá sau COUNT thất bại: %v", err)
		}
	}
	db := dbWithLogger(t, rec, 4)

	dir := t.TempDir()
	var got int64 = -1
	cfg := baseCfg(dir, "onsuccess")
	cfg.OnSuccess = func(n int64) { got = n }

	url, err := Export(streamQuery(db, table), cfg, plainMapper)
	if err != nil {
		t.Fatalf("Export lỗi: %v", err)
	}
	if !rec.fired {
		t.Fatal("hook sau COUNT chưa chạy — test không dựng được ca COUNT ≠ thật")
	}

	inFile := len(rowsFromFile(t, dir, url))
	if inFile != total-removed {
		t.Fatalf("file có %d dòng, muốn %d (dữ liệu thật lúc mở con trỏ)", inFile, total-removed)
	}
	if got != int64(inFile) {
		t.Fatalf("OnSuccess nhận %d, muốn %d = số dòng THẬT trong file. "+
			"history_export.quantityRecord phải khớp file, không khớp COUNT (%d).", got, inFile, total)
	}
}

// ---------------------------------------------------------------------------
// AC7 — con trỏ đóng ở mọi nhánh, không rò kết nối
// ---------------------------------------------------------------------------

func TestExport_KhongRoKetNoi(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	const table = "export_cursor_leak_test"
	mkStreamTable(t, testDB, table, 40)

	// Pool CỐ Ý hẹp: con trỏ không đóng thì vài vòng đầu đã hết kết nối và test treo/lỗi.
	db := dbWithLogger(t, newRecorder(), 2)
	dir := t.TempDir()

	for i := 0; i < 100; i++ {
		if _, err := Export(streamQuery(db, table), baseCfg(dir, fmt.Sprintf("leak-%d", i)), plainMapper); err != nil {
			t.Fatalf("vòng %d lỗi (nghi rò kết nối): %v", i, err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("lấy sql.DB: %v", err)
	}
	if inUse := sqlDB.Stats().InUse; inUse != 0 {
		t.Fatalf("còn %d kết nối đang giữ sau 100 lần export — con trỏ chưa được đóng", inUse)
	}
}

// mapper panic ⇒ con trỏ vẫn phải được đóng (defer chạy khi unwind).
func TestExport_MapperPanic_VanDongConTro(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	const table = "export_cursor_panic_test"
	mkStreamTable(t, testDB, table, 30)

	db := dbWithLogger(t, newRecorder(), 2)
	dir := t.TempDir()

	func() {
		defer func() {
			if recover() == nil {
				t.Error("muốn panic lan ra ngoài, không bị nuốt")
			}
		}()
		_, _ = Export(streamQuery(db, table), baseCfg(dir, "panic"), func(map[string]interface{}) []interface{} {
			panic("mapper nổ giữa chừng")
		})
	}()

	// Con trỏ phải đã đóng ⇒ pool 2 kết nối vẫn dùng được bình thường.
	if _, err := Export(streamQuery(db, table), baseCfg(dir, "after-panic"), plainMapper); err != nil {
		t.Fatalf("export sau panic lỗi — nghi con trỏ chưa đóng: %v", err)
	}
	sqlDB, _ := db.DB()
	if inUse := sqlDB.Stats().InUse; inUse != 0 {
		t.Fatalf("còn %d kết nối đang giữ sau panic", inUse)
	}
}

// ---------------------------------------------------------------------------
// Query dựng bằng db.Raw — đường SỐNG (ExportPaymentByDate,
// internal/modules/webapp/export/report_revenue_by_date.go:98)
// ---------------------------------------------------------------------------

// Cơ chế cũ CỐ Ý bọc derived table vì ".Offset()/.Limit() bị GORM bỏ qua khi query gốc dựng bằng
// db.Raw". Cơ chế mới không dùng Offset/Limit nữa, nhưng vẫn phải chứng minh `Rows()` chạy được
// trên query Raw — nếu không, endpoint /accountant/report/payment-by-date/export sẽ chết.
func TestExport_QueryDungBangRaw(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	const table = "export_cursor_raw_test"
	const n = 120
	mkStreamTable(t, testDB, table, n)

	rec := newRecorder()
	db := dbWithLogger(t, rec, 4)
	raw := db.Raw("SELECT id, code, fullname, amount, created_at, payload FROM "+table+" WHERE active = ?", 1)

	dir := t.TempDir()
	url, err := Export(raw, baseCfg(dir, "raw"), plainMapper)
	if err != nil {
		t.Fatalf("Export trên query Raw lỗi: %v", err)
	}
	ids := idsFromFile(t, dir, url)
	if len(ids) != n {
		t.Fatalf("query Raw ra %d dòng, muốn %d", len(ids), n)
	}
	for i := 1; i <= n; i++ {
		if !ids[int64(i)] {
			t.Fatalf("query Raw thiếu id %d", i)
		}
	}
	if got := len(rec.dataStmts()); got != 1 {
		t.Errorf("query Raw sinh %d câu lấy dữ liệu, muốn 1", got)
	}
}

// ---------------------------------------------------------------------------
// AC9 + AC10 — ExtraSheets chỉ ở nhánh Excel, đúng 1 lần; header dùng chung
// ---------------------------------------------------------------------------

// ExtraSheets là HÀM chứ không phải dữ liệu sẵn vì nó tốn nhiều query, mà nhánh CSV không có khái
// niệm sheet nên sẽ vứt đi toàn bộ. Đếm số lần gọi là cách duy nhất bắt được hồi quy đó.
func TestExport_ExtraSheets_ChiONhanhExcel_DungMotLan(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	for _, tc := range []struct {
		name      string
		rows      int
		wantCalls int
		wantExt   string
	}{
		{"nhánh Excel → gọi đúng 1 lần", 50, 1, ".xlsx"},
		{"nhánh CSV → KHÔNG gọi", 10000, 0, ".csv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := fmt.Sprintf("export_cursor_extra_%d_test", tc.rows)
			mkStreamTable(t, testDB, table, tc.rows)

			calls := 0
			cfg := baseCfg(t.TempDir(), "extra")
			cfg.FieldKeys = []string{"id", "code", "fullname", "amount", "created_at", "payload"}
			cfg.ExtraSheets = func() []ExtraSheet {
				calls++
				return []ExtraSheet{{Name: "Danh mục", Headers: []string{"id", "tên"},
					Rows: [][]interface{}{{1, "Alpha"}, {2, "Beta"}}}}
			}

			url, err := Export(streamQuery(testDB, table), cfg, plainMapper)
			if err != nil {
				t.Fatalf("Export lỗi: %v", err)
			}
			if got := filepath.Ext(url); got != tc.wantExt {
				t.Fatalf("ra %q, muốn %s", got, tc.wantExt)
			}
			if calls != tc.wantCalls {
				t.Fatalf("ExtraSheets được gọi %d lần, muốn %d", calls, tc.wantCalls)
			}
		})
	}
}

// FieldKeys bật ⇒ file có 2 dòng header (nhãn + tên cột máy đọc) ở CẢ hai nhánh; dòng dữ liệu đầu
// tiên phải nằm ngay sau đó. Đây là hợp đồng với importer đọc ngược file export.
func TestExport_HeaderRows_ChungChoCaHaiNhanh(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	keys := []string{"id", "code", "fullname", "amount", "created_at", "payload"}
	for _, tc := range []struct {
		name string
		rows int
	}{{"Excel", 40}, {"CSV", 10000}} {
		t.Run(tc.name, func(t *testing.T) {
			table := fmt.Sprintf("export_cursor_hdr_%d_test", tc.rows)
			mkStreamTable(t, testDB, table, tc.rows)
			dir := t.TempDir()
			cfg := baseCfg(dir, "hdr")
			cfg.FieldKeys = keys

			url, err := Export(streamQuery(testDB, table), cfg, plainMapper)
			if err != nil {
				t.Fatalf("Export lỗi: %v", err)
			}
			// rowsFromFile bỏ ĐÚNG 1 dòng đầu ⇒ dòng đầu còn lại phải là dòng FieldKeys.
			got := rowsFromFile(t, dir, url)
			if len(got) != tc.rows+1 {
				t.Fatalf("có %d dòng sau header nhãn, muốn %d (1 dòng FieldKeys + %d dữ liệu)",
					len(got), tc.rows+1, tc.rows)
			}
			if strings.Join(got[0], ",") != strings.Join(keys, ",") {
				t.Fatalf("dòng 2 không phải FieldKeys: %v", got[0])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Story-03 — KHÔNG để lại file cụt khi lỗi giữa chừng
// ---------------------------------------------------------------------------

// listDir trả tên mọi entry trong thư mục (kể cả file tạm), để chứng minh không còn rác.
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("đọc thư mục: %v", err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

// Hai ĐIỂM LỖI khác nhau, cùng phải cho kết quả: không file cuối, không file tạm, có error.
func TestExport_LoiGiuaChung_KhongDeLaiFileCut(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	for _, tc := range []struct {
		name string
		rows int // quyết định nhánh: <10.000 → Excel, ≥10.000 → CSV
		fail func(call int) error
	}{
		{"nhánh CSV — mapper lỗi ở giữa", 10000, func(call int) error {
			if call == 4000 {
				return fmt.Errorf("mapper hỏng giữa chừng")
			}
			return nil
		}},
		{"nhánh CSV — mapper lỗi ngay dòng đầu", 10000, func(call int) error {
			if call == 1 {
				return fmt.Errorf("mapper hỏng ngay dòng đầu")
			}
			return nil
		}},
		{"nhánh Excel — mapper lỗi ở giữa", 500, func(call int) error {
			if call == 200 {
				return fmt.Errorf("mapper hỏng giữa chừng")
			}
			return nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := fmt.Sprintf("export_atomic_%d_test", tc.rows)
			mkStreamTable(t, testDB, table, tc.rows)
			dir := t.TempDir()

			// mapper không trả error được (chữ ký cố định) ⇒ dùng panic để mô phỏng lỗi giữa chừng,
			// đây cũng chính là đường unwind khắc nghiệt nhất cho việc dọn file tạm.
			calls := 0
			var caught interface{}
			func() {
				defer func() { caught = recover() }()
				_, _ = Export(streamQuery(testDB, table), baseCfg(dir, "atomic"),
					func(r map[string]interface{}) []interface{} {
						calls++
						if err := tc.fail(calls); err != nil {
							panic(err)
						}
						return plainMapper(r)
					})
			}()
			if caught == nil {
				t.Fatal("không có lỗi xảy ra — test không dựng được ca lỗi giữa chừng")
			}

			// KHÔNG được còn gì trong thư mục: không file cuối (người dùng tải nhầm file thiếu dữ
			// liệu), cũng không file tạm (rác nằm vĩnh viễn vì ScheduleFileDelete chỉ đặt khi thành công).
			if left := listDir(t, dir); len(left) != 0 {
				t.Fatalf("còn %d file sau khi lỗi: %v — phải sạch hoàn toàn", len(left), left)
			}
		})
	}
}

// Nhánh thành công: chỉ còn ĐÚNG file cuối, không sót file tạm; URL vẫn theo mẫu cũ.
func TestExport_ThanhCong_ChiConFileCuoi(t *testing.T) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	for _, rows := range []int{500, 10000} {
		t.Run(fmt.Sprintf("%d dòng", rows), func(t *testing.T) {
			table := fmt.Sprintf("export_atomic_ok_%d_test", rows)
			mkStreamTable(t, testDB, table, rows)
			dir := t.TempDir()

			url, err := Export(streamQuery(testDB, table), baseCfg(dir, "atomic-ok"), plainMapper)
			if err != nil {
				t.Fatalf("Export lỗi: %v", err)
			}
			left := listDir(t, dir)
			if len(left) != 1 {
				t.Fatalf("có %d file, muốn đúng 1 (file cuối): %v", len(left), left)
			}
			if strings.HasSuffix(left[0], ".part") || strings.HasPrefix(left[0], ".export-") {
				t.Fatalf("file còn lại là file TẠM chưa được rename: %s", left[0])
			}
			if !strings.HasPrefix(url, "https://example.test"+DownloadPath) {
				t.Fatalf("URL đổi mẫu: %s", url)
			}
			if !strings.HasSuffix(url, left[0]) {
				t.Fatalf("URL %q không trỏ tới file thật %q", url, left[0])
			}
			// File phải đọc được bởi tầng phục vụ tĩnh (trước đây os.Create cho 0644).
			st, err := os.Stat(filepath.Join(dir, left[0]))
			if err != nil {
				t.Fatalf("stat file cuối: %v", err)
			}
			if perm := st.Mode().Perm(); perm&0o044 == 0 {
				t.Fatalf("quyền file cuối %v — tầng phục vụ tĩnh có thể không đọc được", perm)
			}
		})
	}
}

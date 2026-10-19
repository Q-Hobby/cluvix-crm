//go:build integration

package export

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// updateGolden — chạy `go test -tags=integration ./pkg/export/... -update-golden` để SINH lại
// golden. Mặc định false: test ĐỐI CHIẾU với golden đã có.
//
// Golden phải được sinh trên bản CŨ (trước khi đổi cơ chế đọc trong export.go) — đó là toàn bộ giá
// trị của nó: mốc chứng minh bản vá KHÔNG đổi đầu ra.
var updateGolden = flag.Bool("update-golden", false, "ghi lại golden trong pkg/export/testdata")

const (
	// Vượt CSVThreshold (10.000) ⇒ đi nhánh CSV, và > 2 batch (BatchSize 5.000) ⇒ có biên batch để
	// cửa sổ OFFSET trượt qua. Dưới 5.000 thì chỉ có 1 batch và lỗi KHÔNG thể tái hiện.
	rowsCSVBranch = 12000
	// Dưới CSVThreshold ⇒ đi nhánh Excel. Vẫn > 1 batch để phủ cả vòng lặp.
	rowsXLSXBranch = 7000
)

// mkStreamTable dựng bảng fixture với phổ KIỂU CỘT giống dữ liệu thật (INT / VARCHAR / DECIMAL /
// BIGINT / JSON / TINYINT) — cần cho AC4: golden phải khoá được KIỂU Go mà mapper nhận, không chỉ
// giá trị. Đổi kiểu âm thầm (vd int64 → []uint8) làm mọi mapper dạng `return r["id"]` ghi ra ô
// Excel khác đi mà không có lỗi nào.
//
// CỐ Ý KHÔNG đánh index trên `active`: để MySQL quét bảng theo PK, thứ tự dòng tất định.
func mkStreamTable(t *testing.T, db *gorm.DB, name string, n int) {
	t.Helper()
	exec(t, db, "DROP TABLE IF EXISTS "+name)
	exec(t, db, `CREATE TABLE `+name+` (
		id INT PRIMARY KEY,
		code VARCHAR(32) NOT NULL,
		fullname VARCHAR(128) NULL,
		amount DECIMAL(20,2) NOT NULL DEFAULT 0,
		created_at BIGINT NULL,
		payload JSON NULL,
		active TINYINT NOT NULL DEFAULT 1
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	t.Cleanup(func() { exec(t, db, "DROP TABLE IF EXISTS "+name) })

	// Bảng số nhân đôi rồi INSERT ... SELECT — nhanh hơn hẳn insert từng dòng.
	// CỐ Ý dùng bảng THƯỜNG, không TEMPORARY: MySQL không cho mở lại một TEMPORARY table hai lần
	// trong cùng một câu lệnh (Error 1137 "Can't reopen table") mà `INSERT ... SELECT` từ chính nó
	// thì cần đúng như vậy.
	nums := name + "_nums"
	exec(t, db, "DROP TABLE IF EXISTS "+nums)
	exec(t, db, "CREATE TABLE "+nums+" (n INT PRIMARY KEY) ENGINE=InnoDB")
	t.Cleanup(func() { exec(t, db, "DROP TABLE IF EXISTS "+nums) })
	exec(t, db, "INSERT INTO "+nums+" VALUES (1)")
	for i := 0; i < 15; i++ { // 2^15 = 32.768 ≥ mọi n dùng ở đây
		exec(t, db, "INSERT INTO "+nums+" SELECT n + (SELECT MAX(n) FROM (SELECT n FROM "+nums+") x) FROM "+nums)
	}
	exec(t, db, fmt.Sprintf(`INSERT INTO %s (id, code, fullname, amount, created_at, payload, active)
		SELECT n, CONCAT('MA', LPAD(n, 8, '0')),
		       CONCAT('Khách hàng số ', n),
		       1000000 + (n %% 900000), 1735689600 + n,
		       CAST(CONCAT('{"pos":', n %% 7, '}') AS JSON), 1
		FROM `+nums+` WHERE n <= %d`, name, n))

	var got int64
	if err := db.Raw("SELECT COUNT(*) FROM " + name).Scan(&got).Error; err != nil {
		t.Fatalf("đếm bảng fixture: %v", err)
	}
	if got != int64(n) {
		t.Fatalf("fixture có %d dòng, muốn %d", got, n)
	}
}

func exec(t *testing.T, db *gorm.DB, sql string) {
	t.Helper()
	if err := db.Exec(sql).Error; err != nil {
		t.Fatalf("exec %.60s…: %v", sql, err)
	}
}

// streamQuery — query export mẫu. KHÔNG có ORDER BY, đúng như phần lớn hàm Export* thật trong
// internal/modules/webapp/export/ (đã kiểm: các export khối lượng lớn không tự đặt Order).
func streamQuery(db *gorm.DB, table string) *gorm.DB {
	return db.Table(table).
		Select("id, code, fullname, amount, created_at, payload, active").
		Where("active = ?", 1)
}

func streamHeaders() []string {
	return []string{"ID", "Mã", "Họ tên", "Số tiền", "Tạo lúc", "Payload"}
}

// openSecondPool — testhelpers ghim SetMaxOpenConns(1) cho ổn định session-setting; test này cần
// kết nối THỨ HAI để ghi trong lúc export đang đọc.
func openSecondPool(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("testDB khác nil nghĩa là TEST_MYSQL_DSN đã được đặt")
	}
	db2, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("mở pool thứ 2: %v", err)
	}
	sqlDB, err := db2.DB()
	if err != nil {
		t.Fatalf("lấy sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db2
}

// ---------------------------------------------------------------------------
// AC1 + AC2 — export BỎ SÓT dòng khi có ghi đồng thời
// ---------------------------------------------------------------------------

// TestExport_ConcurrentWrite_KhongDuocBoSotDong dựng lại đúng lỗi đã đo trên bench 524.288 dòng
// (18 dòng hợp lệ vắng mặt khỏi file, giao = 0 với tập dòng bị ghi).
//
// Cơ chế kích TẤT ĐỊNH — không sleep, không goroutine đua:
// mapper được gọi cho từng dòng SAU khi một batch đã fetch xong. Ở lần gọi thứ N (giữa batch 0),
// ta gỡ `active` của vài dòng có id NHỎ NHẤT — tức những dòng ĐÃ được ghi ra file rồi. Tập kết quả
// co lại ở phía trước ⇒ batch kế tiếp lấy OFFSET 5000 trên tập đã ngắn đi ⇒ **nhảy qua** đúng bấy
// nhiêu dòng vô can nằm ngay sau biên batch.
//
// Vì sao phải so TẬP ID chứ không so số dòng: ở lần đo trên bench, số dòng file khớp đúng số dòng
// hợp lệ cuối kỳ (262.124) mà vẫn thiếu 18 dòng — file chứa 18 dòng đã hết hợp lệ và thiếu 18 dòng
// vô can. Đếm số dòng KHÔNG bắt được.
// Chạy cho CẢ HAI nhánh ghi file: `export.go` có HAI vòng OFFSET riêng biệt — dòng ~152
// `for offset < int(total)` (Excel) và dòng ~238 `for {` (CSV). Story-02 viết lại cả hai, nên chỉ
// phủ một nhánh là để hở: làm hỏng riêng nhánh kia thì lưới vẫn xanh.
func TestExport_ConcurrentWrite_KhongDuocBoSotDong(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows int
		ext  string
	}{
		{"nhánh CSV", rowsCSVBranch, ".csv"},
		{"nhánh Excel", rowsXLSXBranch, ".xlsx"},
	} {
		t.Run(tc.name, func(t *testing.T) { runConcurrentWriteCase(t, tc.rows, tc.ext) })
	}
}

func runConcurrentWriteCase(t *testing.T, totalRows int, wantExt string) {
	t.Helper()
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	table := fmt.Sprintf("export_stream_concurrent_%d_test", totalRows)
	mkStreamTable(t, testDB, table, totalRows)
	db2 := openSecondPool(t)

	const (
		mutateAfterNRows = 10 // giữa batch 0
		deactivateCount  = 5  // số dòng id nhỏ nhất bị gỡ khỏi tập kết quả
	)

	// mapper được Export gọi TUẦN TỰ trên chính goroutine của test ⇒ không cần khoá.
	var (
		calls   int
		mutated bool
	)
	mapper := func(r map[string]interface{}) []interface{} {
		calls++
		if calls == mutateAfterNRows && !mutated {
			mutated = true
			// Ghi trên kết nối KHÁC, mô phỏng nhân viên gỡ vài dòng trong lúc kế toán bấm export.
			if err := db2.Exec(fmt.Sprintf(
				"UPDATE %s SET active = 0 WHERE id <= %d", table, deactivateCount)).Error; err != nil {
				t.Errorf("ghi đồng thời thất bại: %v", err)
			}
		}
		return []interface{}{r["id"], r["code"], r["fullname"], r["amount"], r["created_at"], r["payload"]}
	}

	dir := t.TempDir()
	url, err := Export(streamQuery(testDB, table), Config{
		FileName:    "concurrent",
		Headers:     streamHeaders(),
		ExportDir:   dir + "/",
		DownloadURL: "https://example.test",
	}, mapper)
	if err != nil {
		t.Fatalf("Export lỗi: %v", err)
	}
	if !mutated {
		t.Fatal("hook ghi đồng thời chưa từng chạy — test không chứng minh được gì")
	}
	if ext := filepath.Ext(url); ext != wantExt {
		t.Fatalf("chạy nhầm nhánh: file ra %q (đuôi %s), muốn %s — CSVThreshold đã đổi?", url, ext, wantExt)
	}

	gotIDs := idsFromFile(t, dir, url)

	// Tập dòng CÒN hợp lệ sau khi export xong. Mọi dòng trong đây PHẢI có mặt trong file:
	// chúng thoả điều kiện từ đầu tới cuối và không hề bị ai đụng tới.
	var stillActive []int64
	if err := db2.Raw("SELECT id FROM " + table + " WHERE active = 1 ORDER BY id").Scan(&stillActive).Error; err != nil {
		t.Fatalf("đọc tập còn hợp lệ: %v", err)
	}
	if len(stillActive) != totalRows-deactivateCount {
		t.Fatalf("tập còn hợp lệ = %d, muốn %d — hook ghi sai chỗ", len(stillActive), totalRows-deactivateCount)
	}

	var missing []int64
	for _, id := range stillActive {
		if !gotIDs[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		show := missing
		if len(show) > 20 {
			show = show[:20]
		}
		t.Fatalf("export BỎ SÓT %d dòng đang hợp lệ (20 id đầu: %v). "+
			"File có %d dòng. Đây là lỗi đúng-sai: file thiếu dữ liệu mà không có cảnh báo nào.",
			len(missing), show, len(gotIDs))
	}
}

// ---------------------------------------------------------------------------
// AC3 + AC4 + AC5 — golden: số dòng, THỨ TỰ id, nội dung ô, KIỂU Go
// ---------------------------------------------------------------------------

type goldenFile struct {
	Branch     string              `json:"branch"`
	RowCount   int                 `json:"rowCount"`
	OrderedIDs []int64             `json:"orderedIds"`
	SampleRows map[string][]string `json:"sampleRows"` // "index" → nội dung ô đã format
	GoTypes    map[string]string   `json:"goTypes"`    // tên cột → kiểu Go mapper nhận
}

func TestExport_Golden_CSVBranch(t *testing.T) {
	runGolden(t, "csv", rowsCSVBranch, ".csv")
}

func TestExport_Golden_XLSXBranch(t *testing.T) {
	runGolden(t, "xlsx", rowsXLSXBranch, ".xlsx")
}

func runGolden(t *testing.T, branch string, n int, wantExt string) {
	if testDB == nil {
		t.Skip("không có TEST_MYSQL_DSN — bỏ qua integration test")
	}
	table := "export_stream_golden_" + branch + "_test"
	mkStreamTable(t, testDB, table, n)

	goTypes := map[string]string{}
	var typesOnce sync.Once
	mapper := func(r map[string]interface{}) []interface{} {
		typesOnce.Do(func() {
			for k, v := range r {
				goTypes[k] = fmt.Sprintf("%T", v)
			}
		})
		return []interface{}{r["id"], r["code"], r["fullname"], r["amount"], r["created_at"], r["payload"]}
	}

	dir := t.TempDir()
	url, err := Export(streamQuery(testDB, table), Config{
		FileName:    "golden-" + branch,
		Headers:     streamHeaders(),
		ExportDir:   dir + "/",
		DownloadURL: "https://example.test",
	}, mapper)
	if err != nil {
		t.Fatalf("Export lỗi: %v", err)
	}
	if ext := filepath.Ext(url); ext != wantExt {
		t.Fatalf("nhánh %s: file ra %q, muốn đuôi %s — ngưỡng CSVThreshold đã đổi?", branch, url, wantExt)
	}

	rows := rowsFromFile(t, dir, url)
	got := goldenFile{
		Branch:     branch,
		RowCount:   len(rows),
		OrderedIDs: make([]int64, 0, len(rows)),
		SampleRows: map[string][]string{},
		GoTypes:    goTypes,
	}
	for _, r := range rows {
		id, err := strconv.ParseInt(strings.TrimSpace(r[0]), 10, 64)
		if err != nil {
			t.Fatalf("cột id không phải số: %q", r[0])
		}
		got.OrderedIDs = append(got.OrderedIDs, id)
	}
	for _, idx := range sampleIndexes(len(rows)) {
		got.SampleRows[strconv.Itoa(idx)] = rows[idx]
	}

	path := filepath.Join("testdata", "golden-"+branch+".json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatalf("tạo testdata: %v", err)
		}
		b, _ := json.MarshalIndent(got, "", "  ")
		if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
			t.Fatalf("ghi golden: %v", err)
		}
		t.Logf("đã ghi %s (%d dòng)", path, got.RowCount)
		return
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("thiếu golden %s — sinh bằng `-update-golden` TRÊN BẢN CŨ: %v", path, err)
	}
	var want goldenFile
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("đọc golden: %v", err)
	}
	compareGolden(t, want, got)
}

func compareGolden(t *testing.T, want, got goldenFile) {
	t.Helper()
	if got.RowCount != want.RowCount {
		t.Errorf("số dòng: %d, golden %d", got.RowCount, want.RowCount)
	}
	if len(got.OrderedIDs) == len(want.OrderedIDs) {
		for i := range want.OrderedIDs {
			if got.OrderedIDs[i] != want.OrderedIDs[i] {
				t.Errorf("THỨ TỰ dòng lệch tại vị trí %d: id %d, golden %d. "+
					"⚠ Thứ tự dòng KHÔNG được đổi (user chốt) — đây là điều kiện Blocked, "+
					"KHÔNG tự thêm ORDER BY để chữa.", i, got.OrderedIDs[i], want.OrderedIDs[i])
				break
			}
		}
	}
	// Tập id: bắt mất/trùng dòng kể cả khi số dòng tình cờ khớp.
	if d := diffIDSets(want.OrderedIDs, got.OrderedIDs); d != "" {
		t.Errorf("tập id lệch: %s", d)
	}
	for k, wv := range want.GoTypes {
		gv, ok := got.GoTypes[k]
		if !ok {
			t.Errorf("cột %q biến mất khỏi map mapper nhận", k)
			continue
		}
		if gv != wv {
			t.Errorf("KIỂU Go của cột %q: %s, golden %s. "+
				"Đổi kiểu làm mapper dạng `return r[%q]` ghi ra ô khác đi mà không có lỗi nào.", k, gv, wv, k)
		}
	}
	for idx, wr := range want.SampleRows {
		gr, ok := got.SampleRows[idx]
		if !ok {
			t.Errorf("thiếu dòng mẫu %s", idx)
			continue
		}
		if strings.Join(gr, "\x1f") != strings.Join(wr, "\x1f") {
			t.Errorf("nội dung dòng %s lệch:\n  got  %q\n  want %q", idx, gr, wr)
		}
	}
}

func diffIDSets(want, got []int64) string {
	w := map[int64]int{}
	for _, id := range want {
		w[id]++
	}
	g := map[int64]int{}
	for _, id := range got {
		g[id]++
	}
	var missing, extra []int64
	for id, c := range w {
		if g[id] < c {
			missing = append(missing, id)
		}
	}
	for id, c := range g {
		if w[id] < c {
			extra = append(extra, id)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return ""
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })
	return fmt.Sprintf("thiếu %d id (%v…), thừa %d id (%v…)",
		len(missing), head(missing), len(extra), head(extra))
}

func head(v []int64) []int64 {
	if len(v) > 10 {
		return v[:10]
	}
	return v
}

func sampleIndexes(n int) []int {
	if n == 0 {
		return nil
	}
	var out []int
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, n / 2, n/2 + 1, n/2 + 2,
		n - 10, n - 9, n - 8, n - 7, n - 6, n - 5, n - 4, n - 3, n - 2, n - 1} {
		if i >= 0 && i < n {
			out = append(out, i)
		}
	}
	sort.Ints(out)
	return out
}

// ---------------------------------------------------------------------------
// Đọc lại file export
// ---------------------------------------------------------------------------

// filePathFromURL — Export trả URL dạng <DownloadURL>/api/download/export/<tên file>; file thật nằm ở
// <ExportDir>/<tên file> (test không set OwnerID ⇒ không có thư mục con owner).
func filePathFromURL(t *testing.T, dir, url string) string {
	t.Helper()
	name := url[strings.LastIndex(url, "/")+1:]
	if name == "" {
		t.Fatalf("không rút được tên file từ URL %q", url)
	}
	return filepath.Join(dir, name)
}

// rowsFromFile trả về các dòng DỮ LIỆU (đã bỏ dòng header) của file export.
func rowsFromFile(t *testing.T, dir, url string) [][]string {
	t.Helper()
	path := filePathFromURL(t, dir, url)
	if strings.HasSuffix(path, ".csv") {
		return csvRows(t, path)
	}
	return xlsxRows(t, path)
}

func csvRows(t *testing.T, path string) [][]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("đọc %s: %v", path, err)
	}
	// exportCSV ghi BOM UTF-8 ở đầu file (cho Excel nhận đúng mã tiếng Việt) — phải bóc trước khi parse.
	body := strings.TrimPrefix(string(b), "\ufeff")
	r := csv.NewReader(strings.NewReader(body))
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse CSV %s: %v", path, err)
	}
	if len(all) == 0 {
		t.Fatalf("CSV rỗng: %s", path)
	}
	return all[1:] // bỏ dòng header
}

func xlsxRows(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("mở xlsx %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	// Sheet dữ liệu LUÔN ở vị trí đầu tiên (importer đọc GetSheetName(0)).
	all, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		t.Fatalf("đọc sheet: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("sheet rỗng: %s", path)
	}
	return all[1:]
}

// idsFromFile trả về tập id có trong file export.
func idsFromFile(t *testing.T, dir, url string) map[int64]bool {
	t.Helper()
	out := map[int64]bool{}
	for _, r := range rowsFromFile(t, dir, url) {
		if len(r) == 0 {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(r[0]), 10, 64)
		if err != nil {
			t.Fatalf("cột id không phải số: %q", r[0])
		}
		out[id] = true
	}
	return out
}

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"io"
	"mime/multipart"
	"regexp"
	"strings"
	"testing"

	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"

	"github.com/xuri/excelize/v2"
)

// Dòng header thật của file do ExportService sinh ra.
func exportHeaderRows() [][]string {
	keys := exportHandlers.ServiceExportFieldKeys()
	labels := make([]string, len(keys))
	// Nhãn không quan trọng với nhánh key-row; dùng nhãn thật ở đâu có để test sát thực tế.
	labelOf := map[string]string{}
	for label, col := range serviceImportHeaderToCol {
		if _, seen := labelOf[col]; !seen {
			labelOf[col] = label
		}
	}
	for i, k := range keys {
		labels[i] = labelOf[k]
	}
	return [][]string{labels, keys}
}

// File export ra PHẢI import ngược lại được: mọi field key ở dòng 2 phải nằm trong allowlist
// của importer. Lệch 1 key = cột đó im lặng biến mất khi user sửa file rồi import lại.
func TestServiceExportFieldKeysAreImportable(t *testing.T) {
	valid := map[string]bool{}
	for _, col := range serviceImportHeaderToCol {
		valid[col] = true
	}
	keys := exportHandlers.ServiceExportFieldKeys()
	if len(keys) == 0 {
		t.Fatal("ServiceExportFieldKeys rỗng")
	}
	for i, k := range keys {
		if !valid[k] {
			t.Errorf("field key cột %d (%q) không nằm trong allowlist importer", i, k)
		}
	}
	if !valid["id"] {
		t.Error("thiếu cột id trong allowlist — không UPDATE ngược được, import lại sẽ tạo bản ghi trùng")
	}
}

func TestResolveServiceImportColumns(t *testing.T) {
	header := exportHeaderRows()

	t.Run("file export 2 dòng header → đọc theo field key, data từ dòng 3", func(t *testing.T) {
		rows := append([][]string{}, header...)
		rows = append(rows, []string{"1", "", "Cấy tóc"})
		cols, dataStart, dup := resolveServiceImportColumns(rows)
		if dataStart != 2 || dup != "" {
			t.Fatalf("dataStart = %d, dup = %q; muốn 2 và rỗng", dataStart, dup)
		}
		keys := exportHandlers.ServiceExportFieldKeys()
		if len(cols) != len(keys) {
			t.Fatalf("nhận ra %d cột, muốn %d", len(cols), len(keys))
		}
		for i, k := range keys {
			if cols[i].idx != i || cols[i].col != k {
				t.Errorf("cột %d = {%d,%q}, muốn {%d,%q}", i, cols[i].idx, cols[i].col, i, k)
			}
		}
	})

	t.Run("template cũ 1 dòng nhãn → đọc theo nhãn, data từ dòng 2", func(t *testing.T) {
		// Story-39 (D23): cột "Thương hiệu" của template cũ nay bị BỎ QUA (không còn trong allowlist)
		// — expected cũ có "company_id" ở vị trí thứ 4, nay chỉ 3 cột được nhận.
		rows := [][]string{
			{"id", "Tên dịch vụ", "Mã dịch vụ", "Thương hiệu"},
			{"7", "Cấy tóc FUE", "DV001", "1"},
		}
		cols, dataStart, _ := resolveServiceImportColumns(rows)
		if dataStart != 1 {
			t.Fatalf("dataStart = %d, muốn 1", dataStart)
		}
		want := []string{"id", "name", "code"}
		if len(cols) != len(want) {
			t.Fatalf("nhận %d cột %v, muốn %d (cột Thương hiệu phải bị bỏ qua)", len(cols), cols, len(want))
		}
		for i, w := range want {
			if cols[i].col != w {
				t.Errorf("cột %d = %q, muốn %q", i, cols[i].col, w)
			}
		}
	})

	t.Run("nhãn cũ đã đổi tên vẫn nhận được qua alias", func(t *testing.T) {
		rows := [][]string{
			{"id", "HH telesale cũ"},
			{"7", "10"},
		}
		cols, dataStart, _ := resolveServiceImportColumns(rows)
		if dataStart != 1 {
			t.Fatalf("dataStart = %d, muốn 1", dataStart)
		}
		if cols[1].col != "commission_telesale" {
			t.Errorf("alias nhãn cũ không map đúng: %v", cols)
		}
	})

	t.Run("dòng dữ liệu không bị nhận nhầm là field-key row", func(t *testing.T) {
		rows := [][]string{
			{"id", "Tên dịch vụ", "Mã dịch vụ", "Thương hiệu"},
			{"7", "code", "name", "1"}, // giá trị trùng tên cột nhưng ít hơn số nhãn nhận ra
		}
		_, dataStart, _ := resolveServiceImportColumns(rows)
		if dataStart != 1 {
			t.Fatalf("dataStart = %d, muốn 1 (không được coi dòng data là header)", dataStart)
		}
	})

	t.Run("file rác → không nhận cột nào", func(t *testing.T) {
		rows := [][]string{
			{"Cột lạ", "Cột lạ 2"},
			{"a", "b"},
		}
		cols, _, _ := resolveServiceImportColumns(rows)
		if len(cols) != 0 {
			t.Fatalf("nhận ra %d cột trên file rác, muốn 0", len(cols))
		}
	})

	t.Run("file rỗng không panic", func(t *testing.T) {
		cols, dataStart, _ := resolveServiceImportColumns(nil)
		if len(cols) != 0 || dataStart != 1 {
			t.Fatalf("got (%v, %d)", cols, dataStart)
		}
	})
}

// Round-trip thật qua excelize: dựng file đúng cách exportExcel ghi (dòng 1 nhãn, dòng 2 field key,
// dữ liệu từ dòng 3) rồi đọc lại bằng chính GetRows mà handler import dùng.
func TestImportReadsExportedWorkbook(t *testing.T) {
	header := exportHeaderRows()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)

	for r, line := range header {
		for i, v := range line {
			cell, _ := excelize.CoordinatesToCellName(i+1, r+1)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	// 1 dòng dữ liệu ở dòng 3
	if err := f.SetCellValue(sheet, "A3", 7); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	reopened, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()

	rows, err := reopened.GetRows(reopened.GetSheetName(0))
	if err != nil {
		t.Fatal(err)
	}
	cols, dataStart, dup := resolveServiceImportColumns(rows)
	if dataStart != 2 || dup != "" {
		t.Fatalf("dataStart = %d, dup = %q", dataStart, dup)
	}
	if len(cols) != len(exportHandlers.ServiceExportFieldKeys()) {
		t.Fatalf("nhận ra %d cột, muốn %d", len(cols), len(exportHandlers.ServiceExportFieldKeys()))
	}
	if cols[0].col != "id" || rows[dataStart][0] != "7" {
		t.Fatalf("cột id/dữ liệu dòng đầu sai: col=%q val=%q", cols[0].col, rows[dataStart][0])
	}
}

// Ba nhóm xử lý ô trống phải chia HẾT allowlist, không chồng lấn. Thiếu một cột ở cả ba nhóm =
// ô trống của cột đó bị bỏ qua âm thầm (đúng lớp bug "xoá giá trị không được"); nằm ở hai nhóm =
// hành vi phụ thuộc thứ tự if.
func TestServiceImportEmptyCellClassesCoverAllColumns(t *testing.T) {
	all := map[string]bool{}
	for _, col := range serviceImportHeaderToCol {
		all[col] = true
	}
	for col := range all {
		n := 0
		if serviceImportKeepOnEmptyCols[col] {
			n++
		}
		if serviceImportRequiredCols[col] {
			n++
		}
		if _, ok := serviceImportClearOnEmptyCols[col]; ok {
			n++
		}
		if n != 1 {
			t.Errorf("cột %q thuộc %d nhóm xử lý ô trống, phải đúng 1", col, n)
		}
	}
	// Chiều ngược lại: không nhóm nào được chứa cột ngoài allowlist (gõ sai tên = quy tắc chết lặng).
	for col := range serviceImportKeepOnEmptyCols {
		if !all[col] {
			t.Errorf("keep-on-empty có cột lạ %q", col)
		}
	}
	for col := range serviceImportRequiredCols {
		if !all[col] {
			t.Errorf("required có cột lạ %q", col)
		}
	}
	for col := range serviceImportClearOnEmptyCols {
		if !all[col] {
			t.Errorf("clear-on-empty có cột lạ %q", col)
		}
	}
	for col := range serviceImportMoneyCols {
		if !all[col] {
			t.Errorf("money có cột lạ %q", col)
		}
	}
}

// Cột tiền của export phải nằm trong danh sách parse tiền của import — nếu không, giá trị "1.000"
// đi thẳng vào MySQL và thành 1.0 (lệch 1000×).
func TestServiceMoneyColumnsMatchExport(t *testing.T) {
	for _, k := range exportHandlers.ServiceExportFieldKeys() {
		if strings.HasPrefix(k, "commission_") && !strings.HasSuffix(k, "_type") && !serviceImportMoneyCols[k] {
			t.Errorf("cột hoa hồng %q không được parse qua ParseExcelMoney", k)
		}
	}
	for _, k := range []string{"min_price", "max_price"} {
		if !serviceImportMoneyCols[k] {
			t.Errorf("cột tiền %q không được parse qua ParseExcelMoney", k)
		}
	}
}

func TestResolveServiceImportColumnsDuplicate(t *testing.T) {
	// Nhãn cũ và nhãn mới cùng trỏ commission_telesale → phải từ chối, không để thắng thua ngẫu nhiên.
	rows := [][]string{
		{"id", "HH telesale cũ", "HH tư vấn viên cũ"},
		{"7", "10", "20"},
	}
	_, _, dup := resolveServiceImportColumns(rows)
	if dup != "commission_telesale" {
		t.Fatalf("dup = %q, muốn commission_telesale", dup)
	}
}

func TestResolveServiceImportColumnsKeyRowAlone(t *testing.T) {
	// User xoá dòng nhãn khỏi file export, chỉ còn dòng field key.
	rows := append([][]string{}, exportHandlers.ServiceExportFieldKeys())
	rows = append(rows, []string{"7"})
	cols, dataStart, dup := resolveServiceImportColumns(rows)
	if dataStart != 1 || dup != "" {
		t.Fatalf("dataStart = %d, dup = %q; muốn 1 và rỗng", dataStart, dup)
	}
	if len(cols) != len(exportHandlers.ServiceExportFieldKeys()) {
		t.Fatalf("nhận ra %d cột, muốn %d", len(cols), len(exportHandlers.ServiceExportFieldKeys()))
	}
}

// readImportRows phải đọc được .csv — export >= CSVThreshold dòng ra CSV, không đọc được thì
// round-trip đứt đúng lúc danh mục lớn nhất.
func TestReadImportRowsCSV(t *testing.T) {
	keys := exportHandlers.ServiceExportFieldKeys()
	var body bytes.Buffer
	body.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM giống hệt file export ghi ra
	w := csv.NewWriter(&body)
	_ = w.Write([]string{"id", "Tên dịch vụ"})
	_ = w.Write([]string{keys[0], "name"})
	_ = w.Write([]string{"7", "Cấy tóc"})
	w.Flush()

	rows, err := readImportRowsFromBytes(t, "Dich_vu.csv", body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cols, dataStart, _ := resolveServiceImportColumns(rows)
	if dataStart != 2 {
		t.Fatalf("dataStart = %d, muốn 2", dataStart)
	}
	if len(cols) != 2 || cols[0].col != "id" || cols[1].col != "name" {
		t.Fatalf("cột đọc từ CSV sai: %v", cols)
	}
	if rows[2][1] != "Cấy tóc" {
		t.Fatalf("dữ liệu CSV sai: %q", rows[2][1])
	}
}

// readImportRowsFromBytes dựng multipart.FileHeader thật từ bytes để test đúng hàm production.
func readImportRowsFromBytes(t *testing.T, filename string, content []byte) ([][]string, error) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(&buf, mw.Boundary())
	form, err := mr.ReadForm(10 << 20)
	if err != nil {
		t.Fatal(err)
	}
	return readImportRows(form.File["file"][0])
}

// Excel: ô đã định dạng (nghìn/phần trăm) phải đọc ra giá trị THÔ, không phải chuỗi đã render.
func TestReadImportRowsExcelRawValues(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	fmtVN := "#,##0"
	styleVN, err := f.NewStyle(&excelize.Style{CustomNumFmt: &fmtVN})
	if err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue(sheet, "A1", "min_price")
	_ = f.SetCellValue(sheet, "A2", float64(1500000))
	_ = f.SetCellStyle(sheet, "A2", "A2", styleVN)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	rows, err := readImportRowsFromBytes(t, "Dich_vu.xlsx", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][0] != "1500000" {
		t.Fatalf("đọc ra %q — phải là giá trị thô 1500000, không phải chuỗi đã định dạng", rows[1][0])
	}
}

// rewriteCellRaw — ghi đè giá trị THÔ của 1 ô trong file xlsx, thứ excelize không cho làm qua API
// (SetCellValue(3.0) vẫn ghi "3"). Cần thiết để dựng lại đúng thứ Google Sheets/LibreOffice sinh ra:
// số nguyên nằm trong XML dưới dạng "3.0".
func rewriteCellRaw(t *testing.T, src []byte, cellRef, oldVal, newVal string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	pattern := regexp.MustCompile(`(<c r="` + cellRef + `"[^>]*>)<v>` + regexp.QuoteMeta(oldVal) + `</v>`)
	replaced := false
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") {
			if pattern.Match(body) {
				body = pattern.ReplaceAll(body, []byte(`${1}<v>`+newVal+`</v>`))
				replaced = true
			}
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if !replaced {
		t.Fatalf("không tìm thấy ô %s giá trị %q để sửa — test tự nó hỏng, không phải code sai", cellRef, oldVal)
	}
	return out.Bytes()
}

// File do Google Sheets xuất ra ghi số nguyên thành "3.0". Ô `id` phải vẫn dẫn dòng vào nhánh
// UPDATE của đúng bản ghi 3 — trước bản vá nó ra 0, khiến dòng không được gom vào importIDs mà cũng
// không rơi về nhánh create, báo "Không tồn tại" rồi rollback cả file (sự cố ARA Plus 2026-09-18).
func TestImportReadsGoogleSheetsIntegerCells(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	for col, key := range []string{"id", "name", "code", "status", "service_type_id"} {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		_ = f.SetCellValue(sheet, cell, key)
	}
	_ = f.SetCellValue(sheet, "A2", 3)
	_ = f.SetCellValue(sheet, "B2", "Cấy tóc")
	_ = f.SetCellValue(sheet, "C2", "DV001")
	_ = f.SetCellValue(sheet, "D2", 1)
	_ = f.SetCellValue(sheet, "E2", 1)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	content := buf.Bytes()
	for _, m := range []struct{ ref, old, new string }{
		{"A2", "3", "3.0"}, {"D2", "1", "1.0"}, {"E2", "1", "1.0"},
	} {
		content = rewriteCellRaw(t, content, m.ref, m.old, m.new)
	}

	rows, err := readImportRowsFromBytes(t, "Dich_vu.xlsx", content)
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][0] != "3.0" {
		t.Fatalf("tiền đề của test sai: ô id đọc ra %q, phải là %q", rows[1][0], "3.0")
	}

	cols, dataStart, dup := resolveServiceImportColumns(rows)
	if dup != "" || dataStart != 1 {
		t.Fatalf("nhận diện header sai: dataStart=%d dup=%q", dataStart, dup)
	}
	byCol := map[string]string{}
	for _, ic := range cols {
		byCol[ic.col] = normalizeExcelIntCell(ic.col, strings.TrimSpace(rows[dataStart][ic.idx]))
	}
	if got := helpers.ToInt64(byCol["id"]); got != 3 {
		t.Errorf("id = %d, muốn 3 — 0 làm dòng kẹt giữa update và create", got)
	}
	if byCol["id"] == "" {
		t.Error("id rỗng — dòng sẽ bị hiểu nhầm thành tạo mới")
	}
	if v, ok := parseEnumValue(define.ServiceStatusOptions, byCol["status"]); !ok || v != 1 {
		t.Errorf("status = (%d,%v), muốn (1,true) — giá trị 1 do chính hệ thống export ra", v, ok)
	}
	if id, name := parseCatalogRef(byCol["service_type_id"]); id != 1 || name != "" {
		t.Errorf("service_type_id = (id:%d,name:%q), muốn (1,\"\")", id, name)
	}
}

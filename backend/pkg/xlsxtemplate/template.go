// Package xlsxtemplate dựng file mẫu import Excel dùng chung cho mọi module.
//
// File mẫu import là FILE TĨNH ở frontend/src/assets/download/, sinh bằng `cmd/gen-template` rồi
// commit vào repo frontend — BE KHÔNG có endpoint phát file mẫu (quy định chung của dự án). Vì vậy
// Spec.Catalog chỉ được nhận danh mục CỐ ĐỊNH của hệ thống; danh mục theo từng doanh nghiệp (khoa,
// nhóm dịch vụ, đơn vị, thương hiệu, thuế suất) có id khác nhau ở mỗi database khách hàng nên chỉ
// đi kèm FILE EXPORT (sheet tra cứu), không nằm trong file mẫu.
package xlsxtemplate

import (
	"fmt"

	"erpCluvix/pkg/export"

	"github.com/xuri/excelize/v2"
)

// Spec — mô tả 1 file mẫu import. Cột và thứ tự cột LUÔN lấy thẳng từ colDef export của
// module (`XxxExportHeaders()` / `XxxExportFieldKeys()`), không khai lại bằng tay: template và file
// export vì thế không thể lệch nhau — đó là cả mục đích của việc sinh tự động.
type Spec struct {
	SheetName  string             // sheet dữ liệu — PHẢI là sheet đầu tiên, importer đọc GetSheetName(0)
	GuideTitle string             // tiêu đề sheet hướng dẫn
	Headers    []string           // dòng 1: nhãn hiển thị
	Keys       []string           // dòng 2: tên cột DB (importer nhận cột theo dòng này)
	Notes      map[string]string  // ghi chú từng cột, khoá theo tên cột DB
	ColWidths  map[string]float64 // bề rộng riêng cho vài cột dài; còn lại 18
	Rules      []string           // "QUY TẮC QUAN TRỌNG" — riêng theo nghiệp vụ từng module

	// Catalog — danh mục tra cứu của DOANH NGHIỆP đang đăng nhập, ghi ra sheet riêng và dùng làm
	// nguồn cho dropdown. Bỏ trống → file mẫu không có sheet danh mục, không có dropdown.
	Catalog []CatalogColumn
	// CatalogSheetName — tên sheet chứa danh mục. Bỏ trống → "Danh_muc".
	CatalogSheetName string
	// DropdownRows — số dòng dữ liệu được gắn dropdown (từ dòng 3). Bỏ trống → 1000.
	DropdownRows int
}

// CatalogColumn — 1 danh mục tra cứu: một cột trong sheet danh mục, đồng thời là nguồn dropdown cho
// những cột dữ liệu dùng chung danh mục đó (vd "Loại hoa hồng" có thể gắn cho nhiều cột
// commission_*_type).
type CatalogColumn struct {
	Title  string   // tiêu đề cột trong sheet danh mục
	Cols   []string // tên cột DB (dòng 2 sheet dữ liệu) được gắn dropdown này
	Values []string // giá trị chọn được, dạng "12 — Khoa Nội" hoặc "Người lớn"
}

// templateIntro — phần mô tả cấu trúc file, giống nhau ở mọi template vì luật đọc file (2 dòng
// header, dữ liệu từ dòng 3) là luật chung của importer, không phải của riêng module nào.
var templateIntro = []string{
	"  - Sheet 'Mau_import' chứa dữ liệu nhập. KHÔNG xoá dòng 1 (nhãn) và dòng 2 (tên cột DB).",
	"  - Dòng 1: nhãn hiển thị tiếng Việt — chỉ để đọc.",
	"  - Dòng 2: tên cột DB — hệ thống dùng ĐÚNG dòng này để nhận cột. Không sửa.",
	"  - Dòng 3 trở đi: dữ liệu. File mẫu cố ý không có dòng ví dụ để bạn không import nhầm.",
	"  - File này có đúng bộ cột như file bạn bấm Export ra, nên có thể export → sửa → import lại.",
}

// Build dựng workbook template import.
//
// Sheet dữ liệu CHỈ có 2 dòng header, KHÔNG có dòng ví dụ hay dòng ghi chú. Lý do: importer coi mọi
// dòng trong vùng dữ liệu là một dòng import thật — dòng ví dụ quên xoá sẽ tạo ra bản ghi rác. Ví dụ
// và ghi chú nằm ở sheet 'Huong_dan'. Import file mẫu chưa điền gì sẽ nhận đúng thông báo
// "không có dữ liệu".
func Build(spec Spec) (*excelize.File, error) {
	f := excelize.NewFile()

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7B3F9E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
			{Type: "bottom", Color: "5C2D7A", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}
	dbFieldStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1E3A5F", Size: 10, Italic: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EDE9F6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "C4B5E8", Style: 1},
			{Type: "right", Color: "C4B5E8", Style: 1},
			{Type: "bottom", Color: "C4B5E8", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}

	idx, err := f.NewSheet(spec.SheetName)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		return nil, err
	}

	for i, key := range spec.Keys {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		width := 18.0
		if w, ok := spec.ColWidths[key]; ok {
			width = w
		}
		if err := f.SetColWidth(spec.SheetName, colName, colName, width); err != nil {
			return nil, err
		}
	}

	if err := f.SetRowHeight(spec.SheetName, 1, 36); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(spec.SheetName, 2, 24); err != nil {
		return nil, err
	}
	for i := range spec.Keys {
		labelCell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(spec.SheetName, labelCell, spec.Headers[i]); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(spec.SheetName, labelCell, labelCell, headerStyle); err != nil {
			return nil, err
		}
		keyCell, _ := excelize.CoordinatesToCellName(i+1, 2)
		if err := f.SetCellValue(spec.SheetName, keyCell, spec.Keys[i]); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(spec.SheetName, keyCell, keyCell, dbFieldStyle); err != nil {
			return nil, err
		}
	}

	if err := f.SetPanes(spec.SheetName, &excelize.Panes{
		Freeze: true, YSplit: 2, TopLeftCell: "A3", ActivePane: "bottomLeft",
		Selection: []excelize.Selection{{SQRef: "A3", ActiveCell: "A3", Pane: "bottomLeft"}},
	}); err != nil {
		return nil, err
	}

	if err := writeCatalogSheet(f, spec); err != nil {
		return nil, err
	}
	if err := writeGuideSheet(f, spec); err != nil {
		return nil, err
	}
	return f, nil
}

// CatalogSheetNameOf — tên sheet danh mục thực tế của spec.
func CatalogSheetNameOf(spec Spec) string {
	if spec.CatalogSheetName != "" {
		return spec.CatalogSheetName
	}
	return "Danh_muc"
}

// writeCatalogSheet ghi sheet danh mục (mỗi danh mục 1 cột) rồi gắn dropdown cho các cột dữ liệu
// tương ứng ở sheet nhập liệu.
//
// Dropdown trỏ vào VÙNG Ô của sheet danh mục chứ không nhúng danh sách thẳng vào ô: Excel giới hạn
// danh sách nhúng ~255 ký tự, mà danh mục khoa/dịch vụ của một phòng khám thật vượt xa mức đó —
// nhúng thẳng thì Excel bỏ luôn dropdown, không báo gì.
func writeCatalogSheet(f *excelize.File, spec Spec) error {
	if len(spec.Catalog) == 0 {
		return nil
	}
	sheet := CatalogSheetNameOf(spec)
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7B3F9E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	if err != nil {
		return err
	}

	colIndexByKey := map[string]int{}
	for i, key := range spec.Keys {
		colIndexByKey[key] = i + 1
	}

	dataRows := spec.DropdownRows
	if dataRows <= 0 {
		dataRows = 1000
	}

	for ci, cat := range spec.Catalog {
		colName, _ := excelize.ColumnNumberToName(ci + 1)
		if err := f.SetColWidth(sheet, colName, colName, 38); err != nil {
			return err
		}
		headCell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		if err := f.SetCellValue(sheet, headCell, cat.Title); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, headCell, headCell, headerStyle); err != nil {
			return err
		}
		for ri, v := range cat.Values {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			// Giá trị danh mục do người dùng đặt tên (tên khoa, tên thương hiệu, nhãn mức thuế) —
			// phải qua cùng bộ lọc formula injection với file export.
			if err := f.SetCellValue(sheet, cell, export.SanitizeCell(v)); err != nil {
				return err
			}
		}
		if len(cat.Values) == 0 {
			// Danh mục rỗng (khách chưa khai báo mức nào): KHÔNG gắn dropdown trỏ vào vùng trống —
			// Excel sẽ chặn mọi giá trị, biến ô thành không điền được.
			continue
		}
		src := fmt.Sprintf("%s!$%s$2:$%s$%d", sheet, colName, colName, len(cat.Values)+1)
		for _, key := range cat.Cols {
			dataCol, ok := colIndexByKey[key]
			if !ok {
				continue // cột không có trong file này → bỏ qua, không phải lỗi
			}
			dataColName, _ := excelize.ColumnNumberToName(dataCol)
			dv := excelize.NewDataValidation(true)
			dv.SetSqref(fmt.Sprintf("%s3:%s%d", dataColName, dataColName, dataRows+2))
			dv.SetSqrefDropList(src)
			// Cảnh báo thay vì CHẶN: user vẫn được phép gõ id trần hoặc dán từ file cũ — importer
			// hiểu cả 3 dạng (id / "id — Tên" / tên), dropdown chỉ là lối đi nhanh.
			dv.SetError(excelize.DataValidationErrorStyleWarning, "Giá trị ngoài danh mục",
				"Nên chọn từ danh sách. Nếu bạn tự gõ, hãy điền đúng id hoặc đúng tên trong sheet "+sheet+".")
			if err := f.AddDataValidation(spec.SheetName, dv); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeGuideSheet(f *excelize.File, spec Spec) error {
	const guide = "Huong_dan"
	if _, err := f.NewSheet(guide); err != nil {
		return err
	}

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "7B3F9E"},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	sectionStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "DC2626"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FEF2F2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	bodyStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
	})
	tableHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7B3F9E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	rowStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
		Border: []excelize.Border{
			{Type: "left", Color: "E2E8F0", Style: 1},
			{Type: "right", Color: "E2E8F0", Style: 1},
			{Type: "top", Color: "E2E8F0", Style: 1},
			{Type: "bottom", Color: "E2E8F0", Style: 1},
		},
	})
	rowAltStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F5F3FF"}, Pattern: 1},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
		Border: []excelize.Border{
			{Type: "left", Color: "E2E8F0", Style: 1},
			{Type: "right", Color: "E2E8F0", Style: 1},
			{Type: "top", Color: "E2E8F0", Style: 1},
			{Type: "bottom", Color: "E2E8F0", Style: 1},
		},
	})

	_ = f.SetColWidth(guide, "A", "A", 32)
	_ = f.SetColWidth(guide, "B", "B", 30)
	_ = f.SetColWidth(guide, "C", "C", 62)

	_ = f.SetRowHeight(guide, 1, 36)
	_ = f.SetCellValue(guide, "A1", spec.GuideTitle)
	_ = f.SetCellStyle(guide, "A1", "A1", titleStyle)

	_ = f.SetCellValue(guide, "A3", "CẤU TRÚC FILE:")
	_ = f.SetCellStyle(guide, "A3", "A3", sectionStyle)

	for i, line := range templateIntro {
		row := 4 + i
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = f.SetRowHeight(guide, row, 22)
		_ = f.SetCellValue(guide, cell, line)
		_ = f.SetCellStyle(guide, cell, cell, bodyStyle)
	}

	rulesRow := 4 + len(templateIntro) + 1
	_ = f.SetCellValue(guide, cellA(rulesRow), "QUY TẮC QUAN TRỌNG:")
	_ = f.SetCellStyle(guide, cellA(rulesRow), cellA(rulesRow), sectionStyle)
	for i, line := range spec.Rules {
		row := rulesRow + 1 + i
		_ = f.SetRowHeight(guide, row, 22)
		_ = f.SetCellValue(guide, cellA(row), line)
		_ = f.SetCellStyle(guide, cellA(row), cellA(row), bodyStyle)
	}

	tableRow := rulesRow + len(spec.Rules) + 2
	_ = f.SetCellValue(guide, cellA(tableRow), "CÁC CỘT TRONG FILE:")
	_ = f.SetCellStyle(guide, cellA(tableRow), cellA(tableRow), sectionStyle)

	headRow := tableRow + 1
	_ = f.SetRowHeight(guide, headRow, 26)
	for i, h := range []string{"Tên cột DB (dòng 2)", "Nhãn hiển thị (dòng 1)", "Ghi chú"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, headRow)
		_ = f.SetCellValue(guide, cell, h)
		_ = f.SetCellStyle(guide, cell, cell, tableHeaderStyle)
	}
	for i, key := range spec.Keys {
		row := headRow + 1 + i
		st := rowStyle
		if i%2 == 1 {
			st = rowAltStyle
		}
		_ = f.SetRowHeight(guide, row, 22)
		for ci, v := range []string{key, spec.Headers[i], spec.Notes[key]} {
			cell, _ := excelize.CoordinatesToCellName(ci+1, row)
			_ = f.SetCellValue(guide, cell, v)
			_ = f.SetCellStyle(guide, cell, cell, st)
		}
	}
	return nil
}

func cellA(row int) string {
	cell, _ := excelize.CoordinatesToCellName(1, row)
	return cell
}

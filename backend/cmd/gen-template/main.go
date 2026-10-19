// Chạy: go run backend/cmd/gen-template/main.go
// Output: frontend/src/assets/download/mau_import_khach_hang.xlsx
//
//	frontend/src/assets/download/mau_import_nhan_su.xlsx
//	frontend/src/assets/download/mau_import_dich_vu.xlsx
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

// -only chọn template cần sinh. Mặc định sinh tất cả, nhưng file .xlsx nhúng thời điểm tạo trong
// docProps nên chạy lại luôn làm bẩn cả template không định đụng tới — cần sinh lại đúng một cái thì
// dùng cờ này để diff của repo frontend chỉ chứa thứ mình thật sự đổi.
func main() {
	only := flag.String("only", "all", "template cần sinh: all | customer | user | service")
	flag.Parse()

	switch *only {
	case "all":
		genCustomerTemplate()
		genUserTemplate()
		genServiceTemplate()
	case "customer":
		genCustomerTemplate()
	case "user":
		genUserTemplate()
	case "service":
		genServiceTemplate()
	default:
		log.Fatalf("-only không hợp lệ: %q (all | customer | user | service)", *only)
	}
}

func genCustomerTemplate() {
	f := excelize.NewFile()
	defer f.Close()

	// ── Styles ───────────────────────────────────────────────────────
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7B3F9E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
			{Type: "bottom", Color: "5C2D7A", Style: 1},
		},
	})
	dbFieldStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1E3A5F", Size: 10, Italic: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EDE9F6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "C4B5E8", Style: 1},
			{Type: "right", Color: "C4B5E8", Style: 1},
			{Type: "bottom", Color: "C4B5E8", Style: 1},
		},
	})
	exampleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "374151", Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F9F7FE"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "E2D9F3", Style: 1},
			{Type: "right", Color: "E2D9F3", Style: 1},
			{Type: "top", Color: "E2D9F3", Style: 1},
			{Type: "bottom", Color: "E2D9F3", Style: 1},
		},
	})
	noteStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFFBEB"}, Pattern: 1},
		Font:      &excelize.Font{Color: "92400E", Size: 9, Italic: true},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "FDE68A", Style: 1},
			{Type: "right", Color: "FDE68A", Style: 1},
			{Type: "top", Color: "FDE68A", Style: 1},
			{Type: "bottom", Color: "FDE68A", Style: 1},
		},
	})

	// ── Sheet 1: Mau_import ───────────────────────────────────────────
	sheet := "Mau_import"
	idx, _ := f.NewSheet(sheet)
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")

	type colDef struct {
		label   string // Row 1: tiêu đề tiếng Việt
		dbField string // Row 2: tên cột DB (BE dùng để map)
		note    string // Row 5: ghi chú định dạng
		ex1     string // Row 3: ví dụ 1
		ex2     string // Row 4: ví dụ 2
		width   float64
	}

	cols := []colDef{
		{"Họ và tên *", "fullname", "Bắt buộc. Hệ thống tự Title Case.", "Nguyen Van An", "Tran Thi Binh", 22},
		{"Số điện thoại", "phone", "9 chữ số → tự thêm '0'. VD: 901234567 → 0901234567", "0901234567", "0912345678", 18},
		{"SĐT khác", "other_phone", "Số điện thoại phụ (tuỳ chọn)", "0909111222", "", 18},
		{"Tên khác / Biệt danh", "nickname", "Tên gọi khác của khách", "An", "Binh", 22},
		{"Ngày sinh", "birthday", "Định dạng: DD/MM/YYYY. VD: 15/08/1990", "15/08/1990", "01/01/1985", 16},
		{"Năm sinh", "year_born", "Chỉ nhập năm (số). VD: 1990", "1990", "1985", 12},
		{"Giới tính", "gender", "Nhập Nam/nam/1 (Nam), Nữ/nữ/nu/0/2 (Nữ). Giá trị khác → Không xác định.", "Nam", "Nữ", 14},
		{"Email", "email", "Địa chỉ email", "an@gmail.com", "", 24},
		{"Zalo", "zalo", "Số Zalo hoặc link Zalo", "0901234567", "", 18},
		{"Facebook", "facebook", "Link profile Facebook", "facebook.com/nguyenvanan", "", 28},
		{"Instagram", "instagram", "Link hoặc username Instagram", "@nguyenvanan", "", 20},
		{"Địa chỉ", "address", "Địa chỉ đầy đủ", "123 Nguyen Trai, Q1, TP.HCM", "", 36},
		{"Mã khách cũ", "old_code", "Mã từ hệ thống cũ (nếu có)", "KH001", "", 16},
		{"Ngày thu thập", "created_day", "DD/MM/YYYY hoặc DD/MM/YYYY HH:MM:SS", "20/01/2024", "15/03/2023 08:30:00", 22},
	}

	// Column widths
	for i, cd := range cols {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, colName, colName, cd.width)
	}

	// Row 1: Vietnamese headers
	f.SetRowHeight(sheet, 1, 36)
	for i, cd := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, cd.label)
		f.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	// Row 2: DB field names (BE dùng để map)
	f.SetRowHeight(sheet, 2, 24)
	for i, cd := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, cd.dbField)
		f.SetCellStyle(sheet, cell, cell, dbFieldStyle)
	}

	// Rows 3–4: Example data
	examples := [][]string{}
	for _, cd := range cols {
		examples = append(examples, []string{cd.ex1, cd.ex2})
	}
	for row := 0; row < 2; row++ {
		f.SetRowHeight(sheet, row+3, 20)
		for i := range cols {
			cell, _ := excelize.CoordinatesToCellName(i+1, row+3)
			f.SetCellValue(sheet, cell, examples[i][row])
			f.SetCellStyle(sheet, cell, cell, exampleStyle)
		}
	}

	// Row 5: Format notes
	f.SetRowHeight(sheet, 5, 36)
	for i, cd := range cols {
		if cd.note != "" {
			cell, _ := excelize.CoordinatesToCellName(i+1, 5)
			f.SetCellValue(sheet, cell, cd.note)
			f.SetCellStyle(sheet, cell, cell, noteStyle)
		}
	}

	// Freeze rows 1–2
	f.SetPanes(sheet, &excelize.Panes{
		Freeze: true, YSplit: 2, TopLeftCell: "A3", ActivePane: "bottomLeft",
		Selection: []excelize.Selection{{SQRef: "A3", ActiveCell: "A3", Pane: "bottomLeft"}},
	})

	// ── Sheet 2: Huong_dan ────────────────────────────────────────────
	guide := "Huong_dan"
	f.NewSheet(guide)

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
		Border: []excelize.Border{
			{Type: "left", Color: "FFFFFF", Style: 1},
			{Type: "right", Color: "FFFFFF", Style: 1},
		},
	})
	tableRowStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "top"},
		Border: []excelize.Border{
			{Type: "left", Color: "E2E8F0", Style: 1},
			{Type: "right", Color: "E2E8F0", Style: 1},
			{Type: "top", Color: "E2E8F0", Style: 1},
			{Type: "bottom", Color: "E2E8F0", Style: 1},
		},
	})
	tableRowAltStyle, _ := f.NewStyle(&excelize.Style{
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

	f.SetColWidth(guide, "A", "A", 26)
	f.SetColWidth(guide, "B", "B", 28)
	f.SetColWidth(guide, "C", "C", 40)
	f.SetColWidth(guide, "D", "D", 28)

	// Title
	f.SetRowHeight(guide, 1, 36)
	f.SetCellValue(guide, "A1", "HUONG DAN NHAP FILE IMPORT KHACH HANG")
	f.SetCellStyle(guide, "A1", "A1", titleStyle)

	// Cau truc section
	f.SetRowHeight(guide, 3, 24)
	f.SetCellValue(guide, "A3", "CAU TRUC FILE:")
	f.SetCellStyle(guide, "A3", "A3", sectionStyle)

	type guideRow struct {
		row  int
		text string
		col  string
		h    float64
		st   int // 0=title, 1=section, 2=body
	}
	guideRows := []guideRow{
		{4, "  - Sheet 'Mau_import': chua du lieu nhap. KHONG xoa dong 1 (tieu de) va dong 2 (ten truong DB).", "A", 22, 2},
		{5, "  - Dong 1: ten hien thi tieng Viet — chi de doc, khong dung de map.", "A", 22, 2},
		{6, "  - Dong 2: ten truong DB — HE THONG DUNG DONG NAY de map du lieu. KHONG sua.", "A", 22, 2},
		{7, "  - Dong 3, 4: du lieu vi du — XOA truoc khi nhap du lieu that.", "A", 22, 2},
		{8, "  - Dong 5: ghi chu dinh dang — co the xoa (he thong bo qua dong trong).", "A", 22, 2},
		{10, "CAC TRUONG TRONG FILE:", "A", 24, 1},
	}
	for _, gr := range guideRows {
		cell, _ := excelize.CoordinatesToCellName(1, gr.row)
		f.SetCellValue(guide, cell, gr.text)
		f.SetRowHeight(guide, gr.row, gr.h)
		switch gr.st {
		case 1:
			f.SetCellStyle(guide, cell, cell, sectionStyle)
		case 2:
			f.SetCellStyle(guide, cell, cell, bodyStyle)
		}
	}

	// Field reference table header
	f.SetRowHeight(guide, 11, 28)
	for i, h := range []string{"Ten truong DB", "Nhan hien thi", "Dinh dang / Ghi chu", "Vi du"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 11)
		f.SetCellValue(guide, cell, h)
		f.SetCellStyle(guide, cell, cell, tableHeaderStyle)
	}

	// Field reference rows
	for idx2, cd := range cols {
		row := idx2 + 12
		f.SetRowHeight(guide, row, 22)
		st := tableRowStyle
		if idx2%2 == 1 {
			st = tableRowAltStyle
		}
		for ci, v := range []string{cd.dbField, cd.label, cd.note, cd.ex1} {
			cell, _ := excelize.CoordinatesToCellName(ci+1, row)
			f.SetCellValue(guide, cell, v)
			f.SetCellStyle(guide, cell, cell, st)
		}
	}

	// Luu y section
	noteRow := len(cols) + 14
	f.SetRowHeight(guide, noteRow, 24)
	c1, _ := excelize.CoordinatesToCellName(1, noteRow)
	f.SetCellValue(guide, c1, "LUU Y QUAN TRONG:")
	f.SetCellStyle(guide, c1, c1, sectionStyle)

	notes := []string{
		"  - Cac truong select (Chi nhanh, Nguon KH, Benh ly, Dich vu quan tam, Page Facebook, Nhom KH, Chien dich) KHONG can trong file.",
		"  - Cac truong select duoc chon TRONG DIALOG khi bam nut Import — se ap dung cho tat ca dong trong file.",
		"  - Neu file CO truong select (VD: branch_id=3) thi gia tri trong file se duoc uu tien hon gia tri chon trong dialog.",
		"  - Trung lap: neu trung phone + ho ten → HE THONG TU DONG UPDATE, khong tao moi.",
		"  - Dinh dang ngay sinh va ngay thu thap: DD/MM/YYYY hoac YYYY-MM-DD.",
		"  - Toan bo file chay trong 1 giao dich — 1 dong loi se rollback tat ca.",
	}
	for ni, note := range notes {
		r := noteRow + 1 + ni
		f.SetRowHeight(guide, r, 22)
		c, _ := excelize.CoordinatesToCellName(1, r)
		f.SetCellValue(guide, c, note)
		f.SetCellStyle(guide, c, c, bodyStyle)
	}

	// ── Save ────────────────────────────────────────────────────────
	outPath := filepath.Join("..", "frontend", "src", "assets", "download", "mau_import_khach_hang.xlsx")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := f.SaveAs(outPath); err != nil {
		log.Fatalf("save: %v", err)
	}
	log.Printf("✅ Đã tạo: %s", outPath)
}

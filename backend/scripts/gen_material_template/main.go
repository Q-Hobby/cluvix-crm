// Script tạo file XLSX mẫu cho chức năng import vật tư.
// Chạy: go run scripts/gen_material_template/main.go
// Output: ../../frontend/src/assets/download/mau_import_vat_tu.xlsx
package main

import (
	"log"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Vật tư"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"id (để trống = tạo mới)",
		"Mã vật liệu",
		"Nguyên vật liệu (*)",
		"Loại NVL (*)",
		"Đơn vị tính (*)",
		"Giá vốn",
		"Giá bán",
		"Định mức",
		"Mô tả",
		"Trạng thái (1=Hiển thị / 0=Ẩn)",
	}
	example := []interface{}{
		"", "VT001", "Bông y tế", "Vật tư tiêu hao", "Cuộn",
		15000, 25000, 2, "Bông vô trùng", 1,
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F75B6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	exampleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Italic: true, Color: "595959"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"F2F2F2"}, Pattern: 1},
	})
	noteStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: "FF0000", Size: 9},
	})

	cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}
	widths := []float64{24, 16, 30, 22, 18, 14, 14, 12, 30, 32}

	for i, col := range cols {
		cell := col + "1"
		f.SetCellValue(sheet, cell, headers[i])
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, col, col, widths[i])
	}
	f.SetRowHeight(sheet, 1, 36)

	for i, col := range cols {
		cell := col + "2"
		f.SetCellValue(sheet, cell, example[i])
		f.SetCellStyle(sheet, cell, cell, exampleStyle)
	}

	noteRow := "A4"
	f.SetCellValue(sheet, noteRow, "(*) Bắt buộc nhập")
	f.SetCellStyle(sheet, noteRow, noteRow, noteStyle)
	f.MergeCell(sheet, noteRow, "J4")

	f.SetCellValue(sheet, "A5", "Loại NVL và Đơn vị tính nhập đúng tên có trong hệ thống (phân biệt hoa thường)")
	f.SetCellStyle(sheet, "A5", "A5", noteStyle)
	f.MergeCell(sheet, "A5", "J5")

	f.SetCellValue(sheet, "A6", "Nếu có id → cập nhật bản ghi đó; không có id → tạo mới")
	f.SetCellStyle(sheet, "A6", "A6", noteStyle)
	f.MergeCell(sheet, "A6", "J6")

	outPath := filepath.Join("..", "frontend", "src", "assets", "download", "mau_import_vat_tu.xlsx")
	if err := f.SaveAs(outPath); err != nil {
		log.Fatalf("SaveAs failed: %v", err)
	}
	log.Printf("Generated: %s", outPath)
}

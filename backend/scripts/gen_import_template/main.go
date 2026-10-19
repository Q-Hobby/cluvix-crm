// Script tạo file XLSX mẫu cho chức năng import Data Facebook.
// Chạy: go run scripts/gen_import_template/main.go
// Output: ../../frontend/src/assets/download/template_data_facebook.xlsx
package main

import (
	"log"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Data Facebook"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Mã bài QC", "Tiêu đề", "Link",
		"Ngân sách (*)", "Phí",
		"Hiển thị (0=Không / 1=Có)", "Tin nhắn", "Bình luận",
		"Ngày chạy (*) dd/MM/yyyy",
	}
	example := []interface{}{
		"FB001", "Quảng cáo mẫu", "https://facebook.com/ads/...",
		1000000, 50000,
		1, 10, 5,
		"15/06/2026",
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

	cols := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}
	widths := []float64{18, 28, 36, 16, 12, 26, 12, 12, 24}

	for i, col := range cols {
		cell := col + "1"
		f.SetCellValue(sheet, cell, headers[i])
		f.SetCellStyle(sheet, cell, cell, headerStyle)
		f.SetColWidth(sheet, col, col, widths[i])
	}
	f.SetRowHeight(sheet, 1, 32)

	for i, col := range cols {
		cell := col + "2"
		f.SetCellValue(sheet, cell, example[i])
		f.SetCellStyle(sheet, cell, cell, exampleStyle)
	}

	f.SetCellValue(sheet, "A4", "(*) Bắt buộc nhập")
	f.SetCellStyle(sheet, "A4", "A4", noteStyle)
	f.MergeCell(sheet, "A4", "I4")

	f.SetCellValue(sheet, "A5", "Ngày chạy định dạng dd/MM/yyyy — ví dụ: 15/06/2026")
	f.SetCellStyle(sheet, "A5", "A5", noteStyle)
	f.MergeCell(sheet, "A5", "I5")

	// Đường dẫn output tương đối từ working directory (repo root / backend/)
	outPath := filepath.Join("..", "frontend", "src", "assets", "download", "template_data_facebook.xlsx")

	if err := f.SaveAs(outPath); err != nil {
		log.Fatalf("SaveAs failed: %v", err)
	}
	log.Printf("Generated: %s", outPath)
}

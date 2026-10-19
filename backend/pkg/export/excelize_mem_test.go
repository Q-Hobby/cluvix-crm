package export

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Đo RAM của khuôn "dựng file trong bộ nhớ" mà 20 hàm export KHÔNG đi qua pkg/export.Export đang
// dùng (`excelize.NewFile()` + `SetCellValue` từng ô + `SaveAs` ở cuối).
//
// Khác với nhánh streaming: ở đây TOÀN BỘ workbook nằm trong heap cho tới lúc ghi ra đĩa, nên RAM
// tỉ lệ thuận với số dòng × số cột — không có trần nào.
//
// Chạy: go test -run TestExcelizeMemoryPerRow -v ./pkg/export/
func TestExcelizeMemoryPerRow(t *testing.T) {
	if os.Getenv("EXPORT_MEM_PROBE") == "" {
		t.Skip("đặt EXPORT_MEM_PROBE=1 để chạy phép đo RAM (tốn vài giây + vài trăm MB)")
	}
	const cols = 12
	for _, rows := range []int{10000, 50000, 100000, 200000} {
		t.Run(fmt.Sprintf("%d dòng x %d cột", rows, cols), func(t *testing.T) {
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			f := excelize.NewFile()
			sheet := "Sheet1"
			for r := 1; r <= rows; r++ {
				for c := 1; c <= cols; c++ {
					cell, _ := excelize.CoordinatesToCellName(c, r)
					_ = f.SetCellValue(sheet, cell, fmt.Sprintf("giá trị ô %d-%d", r, c))
				}
			}
			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			heap := float64(after.HeapAlloc-before.HeapAlloc) / (1024 * 1024)
			peak := float64(after.TotalAlloc-before.TotalAlloc) / (1024 * 1024)
			t.Logf("RAM | %d dòng x %d cột | heap đang giữ %.0f MB | tổng đã cấp phát %.0f MB | %.2f KB/dòng",
				rows, cols, heap, peak, heap*1024/float64(rows))
			_ = f.Close()
		})
	}
}

// So sánh với StreamWriter — cùng số dòng/cột, cùng nội dung, khác cách ghi.
// StreamWriter ghi tuần tự ra buffer tạm thay vì giữ cả workbook trong heap.
func TestExcelizeStreamWriterMemory(t *testing.T) {
	if os.Getenv("EXPORT_MEM_PROBE") == "" {
		t.Skip("đặt EXPORT_MEM_PROBE=1 để chạy phép đo RAM")
	}
	const cols = 12
	for _, rows := range []int{10000, 100000, 200000} {
		t.Run(fmt.Sprintf("StreamWriter %d dòng x %d cột", rows, cols), func(t *testing.T) {
			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			f := excelize.NewFile()
			sw, err := f.NewStreamWriter("Sheet1")
			if err != nil {
				t.Fatalf("NewStreamWriter: %v", err)
			}
			vals := make([]interface{}, cols)
			for r := 1; r <= rows; r++ {
				for c := 0; c < cols; c++ {
					vals[c] = fmt.Sprintf("giá trị ô %d-%d", r, c+1)
				}
				cell, _ := excelize.CoordinatesToCellName(1, r)
				if err := sw.SetRow(cell, vals); err != nil {
					t.Fatalf("SetRow: %v", err)
				}
			}
			if err := sw.Flush(); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			heap := float64(after.HeapAlloc-before.HeapAlloc) / (1024 * 1024)
			total := float64(after.TotalAlloc-before.TotalAlloc) / (1024 * 1024)
			t.Logf("RAM StreamWriter | %d dòng x %d cột | heap đang giữ %.0f MB | tổng đã cấp phát %.0f MB | %.2f KB/dòng",
				rows, cols, heap, total, heap*1024/float64(rows))
			_ = f.Close()
		})
	}
}

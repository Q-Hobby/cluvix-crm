package export

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

// Sheet dữ liệu PHẢI giữ vị trí đầu tiên sau khi ghi thêm sheet tra cứu: importer đọc
// f.GetSheetName(0) nên nếu sheet tra cứu chen lên trước, file export vừa tải về sẽ import ngược
// lại thành "không nhận ra cột nào" — hoặc tệ hơn, đọc bảng tra cứu như dữ liệu.
func TestWriteExtraSheetsKeepsDataSheetFirst(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetCellValue("Sheet1", "A1", "name"); err != nil {
		t.Fatal(err)
	}
	err := writeExtraSheets(f, []ExtraSheet{{
		Name:    "Thue_suat",
		Headers: []string{"id", "Tên mức"},
		Rows:    [][]interface{}{{5, "Không chịu thuế"}, {}},
	}})
	if err != nil {
		t.Fatalf("writeExtraSheets: %v", err)
	}

	if got := f.GetSheetName(0); got != "Sheet1" {
		t.Fatalf("sheet đầu tiên là %q, muốn Sheet1 (sheet dữ liệu)", got)
	}
	if got, _ := f.GetCellValue("Thue_suat", "A1"); got != "id" {
		t.Fatalf("header sheet tra cứu = %q, muốn id", got)
	}
	if got, _ := f.GetCellValue("Thue_suat", "B2"); got != "Không chịu thuế" {
		t.Fatalf("dòng dữ liệu sheet tra cứu = %q", got)
	}
}

// Nhãn mức thuế do người dùng tự đặt, có thể bắt đầu bằng "=" → phải đi qua sanitize như mọi ô
// khác của file, không thì mở file lên Excel chạy nó như công thức.
func TestWriteExtraSheetsSanitizesFormula(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := writeExtraSheets(f, []ExtraSheet{{
		Name: "Thue_suat", Headers: []string{"Tên mức"},
		Rows: [][]interface{}{{"=1+1"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.GetCellValue("Thue_suat", "A2"); got != "'=1+1" {
		t.Fatalf("ô công thức không được sanitize: %q", got)
	}
}

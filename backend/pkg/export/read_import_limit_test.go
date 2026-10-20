package export

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// uploadedXLSX dựng *multipart.FileHeader chứa file .xlsx nén nhỏ nhưng bung ra ~1 MB (chuỗi lặp).
func uploadedXLSX(t *testing.T) *multipart.FileHeader {
	t.Helper()
	f := excelize.NewFile()
	big := strings.Repeat("A", 1024)
	for r := 1; r <= 1000; r++ {
		if err := f.SetCellValue("Sheet1", fmt.Sprintf("A%d", r), fmt.Sprintf("%d%s", r, big)); err != nil {
			t.Fatal(err)
		}
	}
	var file bytes.Buffer
	if err := f.Write(&file); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "import.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(file.Bytes())
	mw.Close()
	form, err := multipart.NewReader(&body, mw.Boundary()).ReadForm(10 << 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}

// TestReadImportRows_TranGiaiNen — file bung ra vượt ImportUnzipSizeLimit bị từ chối (zip bomb);
// dưới trần đọc bình thường.
func TestReadImportRows_TranGiaiNen(t *testing.T) {
	fh := uploadedXLSX(t)

	rows, err := ReadImportRows(fh)
	if err != nil || len(rows) != 1000 {
		t.Fatalf("dưới trần phải đọc được 1000 dòng: rows=%d err=%v", len(rows), err)
	}

	old := ImportUnzipSizeLimit
	ImportUnzipSizeLimit = 64 << 10
	t.Cleanup(func() { ImportUnzipSizeLimit = old })
	if _, err := ReadImportRows(fh); err == nil {
		t.Fatal("file bung ra vượt trần giải nén phải bị từ chối")
	}
}

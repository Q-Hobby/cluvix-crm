package main

import (
	"bytes"
	"testing"

	exportHandlers "erpCluvix/internal/modules/webapp/export"

	"github.com/xuri/excelize/v2"
)

// Mọi cột của export phải có ghi chú — thêm cột vào export mà quên ghi chú thì template ra một cột
// trống trơn không ai biết điền gì, và lỗi đó không có gì khác bắt được.
func TestUserFieldNotesCoverAllColumns(t *testing.T) {
	keys := exportHandlers.UserExportFieldKeys()
	for _, k := range keys {
		if userFieldNotes[k] == "" {
			t.Errorf("cột %q chưa có ghi chú trong userFieldNotes", k)
		}
	}
	known := map[string]bool{}
	for _, k := range keys {
		known[k] = true
	}
	for k := range userFieldNotes {
		if !known[k] {
			t.Errorf("userFieldNotes có cột lạ %q (không nằm trong export)", k)
		}
	}
}

// Template phải trùng khít bộ cột + thứ tự cột của export, và sheet dữ liệu chỉ có 2 dòng header.
func TestBuildUserTemplate(t *testing.T) {
	f, err := buildUserTemplate()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	g, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = g.Close() }()

	// Importer đọc sheet đầu tiên — sheet dữ liệu bắt buộc đứng vị trí 0.
	if name := g.GetSheetName(0); name != userSheetName {
		t.Fatalf("sheet đầu tiên là %q, phải là %q", name, userSheetName)
	}

	rows, err := g.GetRows(userSheetName, excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	// Không dòng ví dụ/ghi chú trong vùng dữ liệu: dòng thừa bị import thành nhân viên rác.
	if len(rows) != 2 {
		t.Fatalf("sheet dữ liệu có %d dòng, phải đúng 2 (nhãn + field key)", len(rows))
	}

	headers := exportHandlers.UserExportHeaders()
	keys := exportHandlers.UserExportFieldKeys()
	if len(rows[0]) != len(headers) || len(rows[1]) != len(keys) {
		t.Fatalf("số cột: nhãn %d, key %d; export có %d", len(rows[0]), len(rows[1]), len(headers))
	}
	for i := range keys {
		if rows[0][i] != headers[i] {
			t.Errorf("nhãn cột %d = %q, muốn %q", i, rows[0][i], headers[i])
		}
		if rows[1][i] != keys[i] {
			t.Errorf("field key cột %d = %q, muốn %q", i, rows[1][i], keys[i])
		}
	}

	if _, err := g.GetSheetIndex("Huong_dan"); err != nil {
		t.Errorf("thiếu sheet hướng dẫn: %v", err)
	}
}

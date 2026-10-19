package xlsxtemplate

import (
	"strings"
	"testing"
)

func demoSpec() Spec {
	return Spec{
		SheetName:  "Mau_import",
		GuideTitle: "HƯỚNG DẪN",
		Headers:    []string{"id", "Tên", "Khoa thực hiện", "Nhóm tuổi"},
		Keys:       []string{"id", "name", "emr_department_id", "child"},
		Notes:      map[string]string{"id": "a", "name": "b", "emr_department_id": "c", "child": "d"},
		Rules:      []string{"  - quy tắc"},
		Catalog: []CatalogColumn{
			{Title: "Khoa thực hiện", Cols: []string{"emr_department_id"}, Values: []string{"12 — Khoa Nội", "13 — Khoa Ngoại"}},
			{Title: "Nhóm tuổi", Cols: []string{"child"}, Values: []string{"Người lớn", "Trẻ em"}},
		},
	}
}

// Sheet dữ liệu PHẢI đứng đầu kể cả khi có thêm sheet danh mục và sheet hướng dẫn: importer đọc
// GetSheetName(0), lệch là file mẫu tải về không import ngược lại được.
func TestBuildKeepsDataSheetFirst(t *testing.T) {
	f, err := Build(demoSpec())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if got := f.GetSheetName(0); got != "Mau_import" {
		t.Fatalf("sheet đầu tiên = %q, muốn Mau_import", got)
	}
	if got, _ := f.GetCellValue("Danh_muc", "A2"); got != "12 — Khoa Nội" {
		t.Fatalf("sheet danh mục thiếu giá trị: %q", got)
	}
}

// Dropdown phải trỏ vào VÙNG Ô của sheet danh mục (không nhúng danh sách vào ô — Excel giới hạn
// ~255 ký tự và sẽ bỏ luôn dropdown mà không báo gì).
func TestBuildAddsDropdownToMappedColumns(t *testing.T) {
	f, err := Build(demoSpec())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	dvs, err := f.GetDataValidations("Mau_import")
	if err != nil {
		t.Fatal(err)
	}
	if len(dvs) != 2 {
		t.Fatalf("có %d dropdown, muốn 2 (emr_department_id ở cột C, child ở cột D)", len(dvs))
	}
	bySqref := map[string]string{}
	for _, dv := range dvs {
		bySqref[dv.Sqref] = dv.Formula1
	}
	src, ok := bySqref["C3:C1002"]
	if !ok {
		t.Fatalf("không có dropdown cho cột C (emr_department_id): %v", bySqref)
	}
	if !strings.Contains(src, "Danh_muc!$A$2:$A$3") {
		t.Fatalf("nguồn dropdown cột C = %q, muốn trỏ vào vùng ô sheet Danh_muc", src)
	}
}

// Danh mục rỗng (khách chưa khai báo mức nào) mà vẫn gắn dropdown trỏ vùng trống thì Excel chặn
// mọi giá trị → ô thành không điền được.
func TestBuildSkipsDropdownForEmptyCatalog(t *testing.T) {
	spec := demoSpec()
	spec.Catalog[0].Values = nil
	f, err := Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	dvs, _ := f.GetDataValidations("Mau_import")
	for _, dv := range dvs {
		if strings.HasPrefix(dv.Sqref, "C") {
			t.Fatalf("danh mục rỗng mà vẫn gắn dropdown: %+v", dv)
		}
	}
}

// Tên danh mục do người dùng đặt: một khoa tên bắt đầu bằng "=" phải thành chuỗi, không thành công
// thức sống khi mở file mẫu. Cùng bộ lọc với file export (pkg/export.SanitizeCell).
func TestBuildSanitizesCatalogValues(t *testing.T) {
	spec := demoSpec()
	spec.Catalog[0].Values = []string{"=1+1", "12 — Khoa Nội"}
	f, err := Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if got, _ := f.GetCellValue("Danh_muc", "A2"); got != "'=1+1" {
		t.Fatalf("giá trị danh mục không được sanitize: %q", got)
	}
}

package export

import (
	"strings"
	"testing"
)

// Thêm cột vào export mà quên ghi chú thì file mẫu ra một cột không ai biết điền gì.
// (Test này trước ở cmd/gen-template, theo chân notes về đây khi file mẫu chuyển sang sinh runtime.)
func TestServiceTemplateNotesCoverAllColumns(t *testing.T) {
	notes := ServiceTemplateNotes()
	for _, key := range ServiceExportFieldKeys() {
		if strings.TrimSpace(notes[key]) == "" {
			t.Errorf("cột %q chưa có ghi chú trong ServiceTemplateNotes()", key)
		}
	}
}

// Danh mục trỏ tới cột không có trong file = dropdown không bao giờ được gắn, im lặng.
func TestServiceCatalogColumnsTargetExistingColumns(t *testing.T) {
	keys := map[string]bool{}
	for _, k := range ServiceExportFieldKeys() {
		keys[k] = true
	}
	// Không cần DB: chỉ kiểm phần khai báo (Title/Cols), phần Values mới cần query.
	for _, cat := range serviceCatalogSpec() {
		if strings.TrimSpace(cat.Title) == "" {
			t.Errorf("danh mục thiếu Title: %+v", cat)
		}
		for _, col := range cat.Cols {
			if !keys[col] {
				t.Errorf("danh mục %q trỏ tới cột %q không có trong file export", cat.Title, col)
			}
		}
	}
}

// Mỗi cột dữ liệu chỉ được gắn ĐÚNG một danh mục: 2 dropdown chồng lên một ô thì Excel giữ cái
// cuối, và cái nào thắng phụ thuộc thứ tự khai báo — sai âm thầm.
func TestServiceCatalogColumnsNoDuplicateTarget(t *testing.T) {
	seen := map[string]string{}
	for _, cat := range serviceCatalogSpec() {
		for _, col := range cat.Cols {
			if prev, dup := seen[col]; dup {
				t.Errorf("cột %q bị gắn 2 danh mục: %q và %q", col, prev, cat.Title)
			}
			seen[col] = cat.Title
		}
	}
}

// Nguồn dropdown phải trỏ đúng bảng danh mục. Story-62: CatalogSource chỉ còn {Table} (không còn
// scope thương hiệu). Trỏ nhầm bảng thì query lỗi SQL, và vì lỗi bị nuốt để không làm hỏng cả lần
// export, hậu quả là dropdown RỖNG mà không có dấu hiệu gì.
func TestServiceCatalogSourcesScopes(t *testing.T) {
	src := ServiceCatalogSources()
	if got := src["unit_id"]; got.Table != "unit" {
		t.Errorf("unit_id = %+v, muốn bảng unit", got)
	}
	if got := src["service_type_id"]; got.Table != "service_type" {
		t.Errorf("service_type_id = %+v, muốn bảng service_type", got)
	}
}

// Cột tra được theo tên khi import PHẢI có dropdown trong file mẫu, và ngược lại: lệch một chiều
// nào cũng là user thấy một đằng, hệ thống nhận một nẻo.
func TestServiceCatalogSourcesHaveDropdown(t *testing.T) {
	withDropdown := map[string]bool{}
	for _, cat := range serviceCatalogSpec() {
		for _, col := range cat.Cols {
			withDropdown[col] = true
		}
	}
	for col := range ServiceCatalogSources() {
		if !withDropdown[col] {
			t.Errorf("cột %q tra được theo tên khi import nhưng không có dropdown trong file mẫu", col)
		}
	}
}

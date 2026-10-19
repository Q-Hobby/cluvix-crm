package main

import (
	"log"
	"os"
	"path/filepath"

	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/xlsxtemplate"

	"github.com/xuri/excelize/v2"
)

// serviceSheetName — sheet dữ liệu PHẢI là sheet đầu tiên: importer đọc f.GetSheetName(0).
const serviceSheetName = "Mau_import"

// serviceDropdownRows — số dòng được gắn dropdown, khớp trần dòng của importer
// (config/service/handler.MaxImportRows = 5000). Để mặc định (1000) thì file lớn mất dropdown từ
// dòng 1003 mà không có gì báo cho user biết.
const serviceDropdownRows = 5000

// serviceTemplateSpec — file mẫu import dịch vụ.
//
// Cột và thứ tự cột lấy THẲNG từ export (ServiceExportHeaders/ServiceExportFieldKeys) nên template
// và file export không thể lệch nhau. Danh mục thì CHỈ lấy phần cố định của hệ thống
// (ServiceStaticCatalogColumns): đây là file tĩnh dùng chung cho mọi khách hàng, không được nhúng
// khoa/nhóm dịch vụ/đơn vị/thương hiệu/thuế suất của riêng một doanh nghiệp.
func serviceTemplateSpec() xlsxtemplate.Spec {
	return xlsxtemplate.Spec{
		SheetName:    serviceSheetName,
		GuideTitle:   "HƯỚNG DẪN NHẬP FILE IMPORT DỊCH VỤ",
		Headers:      exportHandlers.ServiceExportHeaders(),
		Keys:         exportHandlers.ServiceExportFieldKeys(),
		Notes:        exportHandlers.ServiceTemplateNotes(),
		ColWidths:    exportHandlers.ServiceTemplateColWidths(),
		Rules:        exportHandlers.ServiceTemplateRules(),
		Catalog:      exportHandlers.ServiceStaticCatalogColumns(),
		DropdownRows: serviceDropdownRows,
	}
}

func buildServiceTemplate() (*excelize.File, error) {
	return xlsxtemplate.Build(serviceTemplateSpec())
}

func genServiceTemplate() {
	f, err := buildServiceTemplate()
	if err != nil {
		log.Fatalf("build service template: %v", err)
	}
	defer func() { _ = f.Close() }()

	outPath := filepath.Join("..", "frontend", "src", "assets", "download", "mau_import_dich_vu.xlsx")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := f.SaveAs(outPath); err != nil {
		log.Fatalf("save: %v", err)
	}
	log.Printf("✅ Đã tạo: %s", outPath)
}

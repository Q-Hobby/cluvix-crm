package export

import (
	"fmt"
	"net/http"

	"erpCluvix/pkg/export"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// serviceCol — 1 cột export dịch vụ. Gộp nhãn hiển thị + tên cột DB + cách lấy giá trị vào MỘT
// nguồn sự thật: file export dùng 2 dòng header (dòng 1 nhãn, dòng 2 field key) để import ngược
// lại được, nên 3 danh sách song song sẽ lệch âm thầm khi chèn/xoá cột mà build vẫn xanh.
//
// `key` PHẢI khớp tên cột DB mà importer chấp nhận (allowlist trong
// config/service/handler.ImportServiceFromExcel) — lệch key = cột bị bỏ qua khi import lại.
type serviceCol struct {
	key     string
	header  string
	extract func(row map[string]interface{}) interface{}
}

func plainCol(key, header, field string) serviceCol {
	return serviceCol{key, header, func(r map[string]interface{}) interface{} { return r[field] }}
}

// floatColNullable giữ nguyên NULL (ô trống) thay vì ép về 0 như toFloat.
// Cần cho các cột tiền nullable (min_price/max_price/commission_*): file export là template import
// ngược, nên NULL bị xuất thành 0 sẽ biến thành 0 thật trong DB chỉ vì user export rồi import lại
// mà không sửa gì.
func floatColNullable(key, header, field string) serviceCol {
	return serviceCol{key, header, func(r map[string]interface{}) interface{} {
		if r[field] == nil {
			return nil
		}
		return toFloat(r[field])
	}}
}

var serviceCols = []serviceCol{
	plainCol("id", "id", "id"),
	plainCol("id_old", "id cũ", "id_old"),
	plainCol("name", "Tên dịch vụ", "name"),
	plainCol("code", "Mã dịch vụ", "code"),
	plainCol("code_old", "Mã dịch vụ cũ", "code_old"),
	plainCol("status", "Trạng thái", "status"),
	plainCol("SortOrder", "Vị trí", "SortOrder"),
	plainCol("serviceCategory_id", "Danh mục", "serviceCategory_id"),
	plainCol("service_type_id", "Nhóm dịch vụ", "service_type_id"),
	floatColNullable("min_price", "Giá tối thiểu", "min_price"),
	floatColNullable("max_price", "Giá tối đa", "max_price"),
	plainCol("unit_id", "Đơn vị", "unit_id"),
	plainCol("child", "Nhóm tuổi", "child"),
	plainCol("is_installment", "Trả góp", "is_installment"),
	plainCol("time_reminder", "Nhắc lịch", "time_reminder"),
	// Nhóm hoa hồng TƯ VẤN (chốt lúc lên đơn) — tách rõ khỏi nhóm ĐIỀU TRỊ bên dưới.
	floatColNullable("commission_telesale", "HH tư vấn viên cũ", "commission_telesale"),
	floatColNullable("commission_telesale_new", "HH tư vấn viên mới", "commission_telesale_new"),
	plainCol("commission_telesale_type", "Loại HH tư vấn viên", "commission_telesale_type"),
	// Story-38 (D23): bỏ cột "Thương hiệu" — 1 thương hiệu/DB. Import (config/service
	// ImportServiceFromExcel) khi thiếu cột này fallback thương hiệu duy nhất (đã có sẵn cho
	// trường hợp file thiếu cột), nên round-trip export→import vẫn tạo được dịch vụ mới.
}

// ServiceExportHeaders trả về nhãn hiển thị của file export dịch vụ, đúng thứ tự cột.
// Dùng cho generator template import (cmd/gen-template) để template và file export luôn cùng một
// bộ cột, cùng thứ tự — trước đây template được dựng tay nên lệch dần khỏi export.
func ServiceExportHeaders() []string {
	headers := make([]string, len(serviceCols))
	for i, col := range serviceCols {
		headers[i] = col.header
	}
	return headers
}

// ServiceExportFieldKeys trả về danh sách field key (dòng header thứ 2) của file export dịch vụ,
// đúng thứ tự cột. Dùng cho test đối chiếu với allowlist của importer: key nào importer không nhận
// thì cột đó im lặng biến mất khi user import ngược file vừa export.
func ServiceExportFieldKeys() []string {
	keys := make([]string, len(serviceCols))
	for i, col := range serviceCols {
		keys[i] = col.key
	}
	return keys
}

// taxLabelString — giá trị map-scan của cột VARCHAR về dưới dạng []byte; ép sang string để
// sanitize() (chống Excel formula injection) nhận diện được, vì nó chỉ soi kiểu string.
func taxLabelString(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ExportService exports service list to Excel/CSV.
// File có 2 dòng header (nhãn + field key) để user sửa xong import ngược lại qua
// POST /config/service/import-service-from-excel mà không cần đổi cấu trúc file.
// serviceExportConfig dựng Config từ serviceCols. Tách riêng để test khoá được rằng file export
// LUÔN có dòng field key — nếu ai đó bỏ FieldKeys đi, round-trip hỏng mà build vẫn xanh.
func serviceExportConfig(extra func() []export.ExtraSheet) export.Config {
	return export.Config{
		ExtraSheets: extra,
		FileName:    "Dich_vu",
		Headers:     ServiceExportHeaders(),
		FieldKeys:   ServiceExportFieldKeys(),
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}
}

func ExportService(c *gin.Context, q *gorm.DB) {
	// Sheet tra cứu danh mục (khoa, nhóm DV, đơn vị, thương hiệu, thuế suất + các enum) đi kèm file
	// export: vòng làm việc thật là export → sửa → import lại, mà id danh mục là per-doanh-nghiệp
	// nên không thể có sẵn trong file mẫu tĩnh nào.
	//
	// Truyền dưới dạng HÀM: 5 query danh mục chỉ chạy khi file thật sự ra .xlsx. Danh sách >10.000
	// dòng rơi sang nhánh CSV — không có sheet để ghi, chạy query là phí thuần.
	catalog := func() []export.ExtraSheet {
		if sheet := ServiceCatalogSheet(ServiceCatalogColumns(c)); sheet != nil {
			return []export.ExtraSheet{*sheet}
		}
		return nil
	}
	url, err := doExport(c, q, serviceExportConfig(catalog), func(row map[string]interface{}) []interface{} {
		data := make([]interface{}, len(serviceCols))
		for i, col := range serviceCols {
			data[i] = col.extract(row)
		}
		return data
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

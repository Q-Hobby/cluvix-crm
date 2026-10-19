package export

import (
	"fmt"
	"strings"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/xlsxtemplate"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// File này giữ phần MÔ TẢ CỘT của file mẫu import dịch vụ (ghi chú, bề rộng, quy tắc) nằm ngay
// cạnh `serviceCols` — nguồn sự thật của cột export. Trước đây phần này nằm ở cmd/gen-template nên
// mỗi lần thêm cột export phải nhớ sang chỗ khác cập nhật, và ghi chú đã trôi khỏi hành vi thật
// (child ghi ngược người lớn/trẻ em, time_expired_type ghi 2=tháng trong khi app hiểu 2=năm).

// ServiceTemplateNotes — ghi chú từng cột, khoá theo TÊN CỘT DB (dòng 2 của file).
// Có test bắt buộc mọi cột của export đều có mặt ở đây: thêm cột vào export mà quên ghi chú thì
// file mẫu im lặng ra một cột không ai biết điền gì.
func ServiceTemplateNotes() map[string]string {
	// enumHint — cột có giá trị cố định (không phụ thuộc dữ liệu khách hàng): file mẫu tĩnh vẫn gắn
	// được dropdown vì danh sách giống nhau ở mọi doanh nghiệp.
	const enumHint = "Chọn từ dropdown, hoặc gõ đúng tên. Danh sách nằm ở sheet Danh_muc."
	// catalogHint — cột trỏ tới danh mục riêng của từng doanh nghiệp (khoa, nhóm dịch vụ, đơn vị,
	// thương hiệu, thuế suất). File mẫu là file tĩnh dùng chung nên KHÔNG kèm được id của bạn:
	// cách chắc chắn nhất là Export dịch vụ ra để xem id đang dùng, rồi sửa và import lại.
	const catalogHint = "Gõ id hoặc đúng tên của mục. Không có dropdown vì mỗi doanh nghiệp một id — bấm Export ở màn Dịch vụ để xem id đang dùng, hoặc xem tại màn quản lý danh mục tương ứng."
	return map[string]string{
		"id":                 "Để trống = tạo dịch vụ mới. Có id = cập nhật đúng dịch vụ đó.",
		"id_old":             "Id từ hệ thống cũ (tuỳ chọn).",
		"name":               "Bắt buộc, không được để trống.",
		"code":               "Bắt buộc khi cập nhật. Tạo mới để trống thì hệ thống tự sinh mã.",
		"code_old":           "Mã từ hệ thống cũ (tuỳ chọn).",
		"status":             "Đang dùng / Ngưng. " + enumHint + " Để trống = giữ nguyên.",
		"SortOrder":          "Số thứ tự hiển thị.",
		"serviceCategory_id": "Dịch vụ / Sản phẩm / Liệu trình / Phí tư vấn. " + enumHint,
		"service_type_id":    "Nhóm dịch vụ. " + catalogHint,
		"min_price":          "Giá thấp nhất. Nhập số, không nhập ký hiệu tiền tệ.",
		"max_price":          "Giá cao nhất. Nhập số, không nhập ký hiệu tiền tệ.",
		"unit_id":            "Đơn vị tính. " + catalogHint,
		"child":              "Người lớn / Trẻ em. " + enumHint + " Để trống = không phân biệt.",
		"is_installment":     "Có / Không cho trả góp. " + enumHint,
		"time_reminder":      "Số ngày nhắc lịch sau khi dùng dịch vụ.",

		"commission_telesale":      "Hoa hồng tư vấn viên — khách cũ.",
		"commission_telesale_new":  "Hoa hồng tư vấn viên — khách mới.",
		"commission_telesale_type": "Cách tính hoa hồng tư vấn viên: Số tiền / %. " + enumHint,
	}
}

// ServiceTemplateColWidths — bề rộng riêng cho vài cột dài; còn lại dùng mặc định.
func ServiceTemplateColWidths() map[string]float64 {
	return map[string]float64{
		"name": 32, "code": 16, "code_old": 16,
		"serviceCategory_id": 18, "service_type_id": 26,
		"min_price": 16, "max_price": 16, "unit_id": 18,
	}
}

// ServiceTemplateRules — mục "QUY TẮC QUAN TRỌNG" của sheet hướng dẫn.
//
// File mẫu là file TĨNH nằm ở frontend/src/assets/download/ (quy định chung của dự án: file mẫu
// import do FE phát, BE không có endpoint sinh runtime). Hệ quả phải nói rõ cho người dùng: các cột
// trỏ tới danh mục riêng của doanh nghiệp không có sẵn dropdown, phải tự tra id.
func ServiceTemplateRules() []string {
	return []string{
		"  - Sheet Danh_muc chỉ chứa các danh mục CỐ ĐỊNH của hệ thống (trạng thái, nhóm tuổi, loại hoa hồng...). Các cột này có dropdown: bấm vào ô rồi chọn.",
		"  - 3 cột Nhóm dịch vụ / Đơn vị tính / Thuế suất KHÔNG có dropdown trong file mẫu tĩnh, vì id của chúng khác nhau ở mỗi doanh nghiệp. Điền id, hoặc gõ đúng tên mục.",
		"  - Cách nhanh nhất để biết id đang dùng: vào màn Dịch vụ bấm Export — file export có đúng bộ cột như file mẫu và kèm sheet Danh_muc đầy đủ của doanh nghiệp bạn. Sửa ngay trên file export rồi import lại.",
		"  - Nếu 2 mục trong danh mục trùng tên, phải điền id để chỉ rõ mục nào — hệ thống báo lỗi dòng thay vì tự chọn.",
		"  - Số trần luôn được hiểu là id. Mục danh mục có tên toàn chữ số (vd đơn vị tính tên \"5\") thì phải điền id, đừng gõ tên.",
		"  - Ô ĐỂ TRỐNG nghĩa là XOÁ giá trị đang lưu, với những cột cho phép trống. Muốn giữ nguyên thì xoá hẳn cả cột đó khỏi file.",
		"  - Các cột bắt buộc (Tên dịch vụ) để trống sẽ báo lỗi dòng.",
		"  - id trống = tạo mới, có id = cập nhật. Tạo mới cần quyền tạo, cập nhật cần quyền sửa.",
		"  - Tiền và hoa hồng: nhập số thuần, không thêm ký hiệu tiền tệ. Định dạng nghìn của Excel vẫn nhận đúng.",
		"  - Không để 2 cột cùng trỏ về một trường — hệ thống sẽ từ chối file.",
		"  - Toàn bộ file chạy trong 1 giao dịch: 1 dòng lỗi là rollback tất cả.",
	}
}

// logCatalogQueryError — query danh mục lỗi thì vẫn trả nil để KHÔNG làm hỏng cả lần export/tải file
// mẫu, nhưng phải để lại dấu vết. Nuốt trọn thì biểu hiện duy nhất là "dropdown rỗng" — đúng chân
// dung bệnh mà Hard Rule §"Lỗi SQL bị nuốt" mô tả, và đã suýt dính thật ở bảng `unit` (cột scope là
// `companyId` kiểu JSON, không phải `company_id`).
func logCatalogQueryError(table string, err error) {
	if logger.L == nil {
		return
	}
	logger.L.Error(fmt.Sprintf("[import template] query danh mục %s lỗi: %v", table, err))
}

// CatalogValue dựng nhãn hiển thị của 1 mục danh mục: "12 — Nhóm A".
// Kèm id để phân biệt được 2 mục trùng tên; kèm tên để người đọc hiểu mình đang chọn gì.
// Importer bóc ngược đúng định dạng này (xem parseCatalogRef ở config/service/handler).
func CatalogValue(id interface{}, name string) string {
	return fmt.Sprintf("%v — %s", id, strings.TrimSpace(name))
}

func enumValues(opts []define.ServiceEnumOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Label
	}
	return out
}

// CatalogSource — bảng danh mục. MỘT nguồn sự thật dùng bởi cả file mẫu (sinh
// dropdown) lẫn importer (tra id theo tên), nên "gợi ý trong file" và "cái importer chấp nhận" luôn
// trùng nhau.
type CatalogSource struct {
	Table string
}

// Apply chỉ lấy bản ghi đang bật (1 doanh nghiệp/DB — mọi dòng danh mục đều thuộc doanh nghiệp này).
func (s CatalogSource) Apply(q *gorm.DB) *gorm.DB {
	return q.Where("(status = 1 OR status IS NULL)")
}

// ServiceCatalogSources — cột FK của dịch vụ → bảng danh mục tra ngược được theo TÊN khi import.
func ServiceCatalogSources() map[string]CatalogSource {
	return map[string]CatalogSource{
		"service_type_id": {"service_type"},
		"unit_id":         {"unit"},
	}
}

// serviceCatalogDef — KHAI BÁO 1 danh mục tra cứu: tiêu đề, các cột dữ liệu dùng chung danh mục
// này, và cách lấy giá trị. Tách phần khai báo khỏi phần query để test kiểm được ràng buộc
// "mỗi cột chỉ gắn 1 danh mục" và "cột phải có thật trong file" mà không cần DB.
type serviceCatalogDef struct {
	Title  string
	Cols   []string
	values func(c *gin.Context) []string
	// Static = giá trị cố định của hệ thống, không đọc DB và không phụ thuộc doanh nghiệp nào. Chỉ
	// những danh mục này mới vào được FILE MẪU TĨNH ở frontend/src/assets/download/ (sinh bằng
	// cmd/gen-template, không có gin.Context lẫn DB); danh mục theo tenant chỉ đi kèm file EXPORT.
	Static bool
}

func serviceCatalogSpec() []serviceCatalogDef {
	staticEnum := func(opts []define.ServiceEnumOption) func(*gin.Context) []string {
		return func(*gin.Context) []string { return enumValues(opts) }
	}
	catalogTable := func(col string) func(*gin.Context) []string {
		return func(_ *gin.Context) []string {
			return lookupValues(ServiceCatalogSources()[col])
		}
	}
	return []serviceCatalogDef{
		{Title: "Nhóm dịch vụ", Cols: []string{"service_type_id"}, values: catalogTable("service_type_id")},
		{Title: "Đơn vị tính", Cols: []string{"unit_id"}, values: catalogTable("unit_id")},
		{Title: "Danh mục", Cols: []string{"serviceCategory_id"}, values: staticEnum(define.ServiceCategoryOptions), Static: true},
		{Title: "Nhóm tuổi", Cols: []string{"child"}, values: staticEnum(define.ServiceChildOptions), Static: true},
		{Title: "Loại hoa hồng", Cols: []string{
			"commission_telesale_type",
		}, values: staticEnum(define.ServiceCommissionTypeOptions), Static: true},
		{Title: "Có / Không", Cols: []string{"is_installment"},
			values: staticEnum(define.ServiceFlagOptions), Static: true},
		{Title: "Trạng thái", Cols: []string{"status"}, values: staticEnum(define.ServiceStatusOptions), Static: true},
	}
}

// ServiceCatalogColumns — danh mục tra cứu cho file mẫu import VÀ file export của dịch vụ.
// Mỗi phần tử là 1 cột trong sheet Danh_muc, đồng thời là nguồn dropdown cho các cột `Cols`.
//
// Story-38 (D15): bỏ scope company — danh mục (Nhóm dịch vụ/Đơn vị/Thuế suất) không còn lọc theo
// company, lấy toàn bộ danh mục đang bật của DB (1 thương hiệu/DB nên không cần lọc).
func ServiceCatalogColumns(c *gin.Context) []xlsxtemplate.CatalogColumn {
	defs := serviceCatalogSpec()
	out := make([]xlsxtemplate.CatalogColumn, 0, len(defs))
	for _, d := range defs {
		out = append(out, xlsxtemplate.CatalogColumn{Title: d.Title, Cols: d.Cols, Values: d.values(c)})
	}
	return out
}

// ServiceStaticCatalogColumns — chỉ những danh mục CỐ ĐỊNH của hệ thống (trạng thái, nhóm tuổi,
// loại hoa hồng...). Dùng cho file mẫu import TĨNH: file này nằm ở
// frontend/src/assets/download/ và dùng chung cho mọi khách hàng, nên không được chứa danh mục đọc
// từ DB của một doanh nghiệp cụ thể. 2 cột danh mục (nhóm dịch vụ, đơn vị) vì thế
// không có dropdown trong file mẫu — sheet hướng dẫn nói rõ điều đó và chỉ người dùng sang đường
// Export (ServiceCatalogColumns) để lấy id thật.
func ServiceStaticCatalogColumns() []xlsxtemplate.CatalogColumn {
	out := make([]xlsxtemplate.CatalogColumn, 0, len(serviceCatalogSpec()))
	for _, d := range serviceCatalogSpec() {
		if !d.Static {
			continue
		}
		out = append(out, xlsxtemplate.CatalogColumn{Title: d.Title, Cols: d.Cols, Values: d.values(nil)})
	}
	return out
}

// lookupValues đọc danh mục (id, name) đang bật của 1 bảng.
func lookupValues(src CatalogSource) []string {
	if DB == nil || src.Table == "" {
		return nil
	}
	var rows []map[string]interface{}
	if err := src.Apply(DB.Table(src.Table).Select("id, name")).
		Order("name ASC").Find(&rows).Error; err != nil {
		logCatalogQueryError(src.Table, err)
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, CatalogValue(r["id"], taxLabelString(r["name"])))
	}
	return out
}

// ServiceCatalogSheet — cùng bộ danh mục đó, đóng gói thành sheet tra cứu kèm theo FILE EXPORT.
// Vòng làm việc thật là export → sửa → import lại, nên file export cũng cần bảng tra như file mẫu.
func ServiceCatalogSheet(cols []xlsxtemplate.CatalogColumn) *export.ExtraSheet {
	headers := make([]string, 0, len(cols))
	height := 0
	for _, col := range cols {
		headers = append(headers, col.Title)
		if len(col.Values) > height {
			height = len(col.Values)
		}
	}
	if height == 0 {
		return nil
	}
	rows := make([][]interface{}, height)
	for r := 0; r < height; r++ {
		line := make([]interface{}, len(cols))
		for ci, col := range cols {
			if r < len(col.Values) {
				line[ci] = col.Values[r]
			}
		}
		rows[r] = line
	}
	return &export.ExtraSheet{Name: xlsxtemplate.CatalogSheetNameOf(xlsxtemplate.Spec{}), Headers: headers, Rows: rows}
}

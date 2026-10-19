package handler

import (
	"testing"

	configServiceDao "erpCluvix/internal/modules/webapp/config/service/dao"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"

	"github.com/shopspring/decimal"
)

func TestParseImportBool(t *testing.T) {
	for _, raw := range []string{"1", "true", "TRUE", "x", "Có", "yes"} {
		if v, ok := parseImportBool(raw); !ok || v != 1 {
			t.Errorf("parseImportBool(%q) = %d,%v — muốn 1,true", raw, v, ok)
		}
	}
	for _, raw := range []string{"0", "false", "Không", "no"} {
		if v, ok := parseImportBool(raw); !ok || v != 0 {
			t.Errorf("parseImportBool(%q) = %d,%v — muốn 0,true", raw, v, ok)
		}
	}
	// Chuỗi lạ phải báo lỗi dòng, KHÔNG mặc định về 0: "2" hay "tắt" là user gõ nhầm, im lặng
	// tắt cờ chăm sóc của cả file là hỏng dữ liệu không dấu vết.
	for _, raw := range []string{"2", "tắt", "-1", "1.0"} {
		if _, ok := parseImportBool(raw); ok {
			t.Errorf("parseImportBool(%q) nhận nhầm là hợp lệ", raw)
		}
	}
}

func TestParseImportUint(t *testing.T) {
	for raw, want := range map[string]int64{"0": 0, "10": 10, "1.000": 1000, "1 000": 1000, "1,000": 1000} {
		if v, ok := parseImportUint(raw); !ok || v != want {
			t.Errorf("parseImportUint(%q) = %d,%v — muốn %d", raw, v, ok, want)
		}
	}
	// Số thập phân/âm/chữ: KHÔNG đoán bừa (xem comment trong parseImportUint).
	for _, raw := range []string{"8.5", "8,5", "-1", "abc", ""} {
		if _, ok := parseImportUint(raw); ok {
			t.Errorf("parseImportUint(%q) nhận nhầm là hợp lệ", raw)
		}
	}
}

// Giá trị dropdown ("12 — Khoa Nội"), id trần, và tên trần phải cho cùng một kết quả — nếu không
// thì dropdown chỉ là một cái bẫy mới cho user.
func TestParseCatalogRef(t *testing.T) {
	cases := []struct {
		raw      string
		wantID   int64
		wantName string
	}{
		{"12", 12, ""},
		{" 12 ", 12, ""},
		{"12 — Khoa Nội", 12, "Khoa Nội"},
		{"12 - Khoa Nội", 12, "Khoa Nội"},
		{"3 — VAT 8% (8%)", 3, "VAT 8% (8%)"},
		{"Khoa Nội", 0, "Khoa Nội"},
		// Tên có gạch ngang nhưng vế trái KHÔNG phải số → giữ nguyên cả chuỗi làm tên.
		{"Khoa Nội - Tổng hợp", 0, "Khoa Nội - Tổng hợp"},
		{"", 0, ""},
	}
	for _, tc := range cases {
		id, name := parseCatalogRef(tc.raw)
		if id != tc.wantID || name != tc.wantName {
			t.Errorf("parseCatalogRef(%q) = (%d, %q), muốn (%d, %q)", tc.raw, id, name, tc.wantID, tc.wantName)
		}
	}
}

func TestParseEnumValue(t *testing.T) {
	// Bộ nhãn cục bộ của test (không gắn cột nào): đủ để kiểm cả nhánh số lẫn nhánh nhãn, không phân
	// biệt hoa thường, và dạng "id — nhãn" của dropdown.
	opts := []define.ServiceEnumOption{{Value: 1, Label: "Ngày"}, {Value: 2, Label: "Năm"}, {Value: 3, Label: "Tháng"}}
	for raw, want := range map[string]int{"1": 1, "2": 2, "Ngày": 1, "năm": 2, "THÁNG": 3, "2 — Năm": 2} {
		if v, ok := parseEnumValue(opts, raw); !ok || v != want {
			t.Errorf("parseEnumValue(%q) = %d,%v — muốn %d", raw, v, ok, want)
		}
	}
	// Số ngoài bộ giá trị hợp lệ phải bị từ chối: nhận bừa thì FE hiện "Không rõ" và không ai
	// truy được lỗi từ đâu ra. (0 là ngoại lệ — xem TestParseEnumValueAcceptsLegacyZero.)
	for _, raw := range []string{"9", "-1", "tuần", ""} {
		if _, ok := parseEnumValue(opts, raw); ok {
			t.Errorf("parseEnumValue(%q) nhận nhầm là hợp lệ", raw)
		}
	}
}

// Cột khai trong bảng enum/danh mục mà KHÔNG nằm trong allowlist import = khai báo chết: dropdown
// có trong file mẫu nhưng import bỏ qua cột đó, user không hiểu vì sao điền mà không vào.
func TestEnumAndCatalogColsAreImportable(t *testing.T) {
	allow := map[string]bool{}
	for _, col := range serviceImportHeaderToCol {
		allow[col] = true
	}
	for col := range serviceImportEnumCols {
		if !allow[col] {
			t.Errorf("cột enum %q không nằm trong allowlist import", col)
		}
	}
	for col := range exportHandlers.ServiceCatalogSources() {
		if !allow[col] {
			t.Errorf("cột danh mục %q không nằm trong allowlist import", col)
		}
	}
	// Một cột chỉ được xử lý theo MỘT kiểu: vừa enum vừa cờ/tiền thì nhánh nào chạy trước sẽ nuốt
	// nhánh sau, tuỳ thứ tự if trong handler.
	for col := range serviceImportEnumCols {
		if serviceImportBoolCols[col] || serviceImportMoneyCols[col] || serviceImportUintCols[col] {
			t.Errorf("cột %q vừa là enum vừa là kiểu khác", col)
		}
		if _, isCatalog := exportHandlers.ServiceCatalogSources()[col]; isCatalog {
			t.Errorf("cột %q vừa là enum vừa là danh mục", col)
		}
	}
}

// Bất biến của cả module này: file export ra PHẢI import ngược lại được y nguyên. `service.child`
// mặc định 0 trong DB (baseline migration) và vài cột loại của dữ liệu nhập từ hệ thống cũ cũng
// mang 0 — export ghi thẳng giá trị DB, nên nếu enum từ chối 0 thì user export rồi import lại mà
// không sửa gì cũng bị rollback cả file. Test này khoá đúng cái bất biến đó.
func TestParseEnumValueAcceptsLegacyZero(t *testing.T) {
	for col, opts := range serviceImportEnumCols {
		v, ok := parseEnumValue(opts, "0")
		if !ok || v != 0 {
			t.Errorf("cột %q: parseEnumValue(\"0\") = %d,%v — dữ liệu cũ mang giá trị 0 phải import lại được", col, v, ok)
		}
	}
}

// Bảng tính không phải Excel desktop (Google Sheets, LibreOffice) ghi MỌI số nguyên thành "3.0".
// ReadImportRows đọc giá trị THÔ (bắt buộc, cho ô ngày) nên chuỗi đó tới thẳng parser.
// Sự cố thật 2026-09-18 (ARA Plus): ô id "3.0" → ToInt64 = 0 → dòng kẹt giữa update và create,
// báo "Không tồn tại" rồi rollback cả file.
func TestNormalizeExcelIntCell(t *testing.T) {
	cases := []struct {
		col, raw, want string
		why            string
	}{
		{"id", "3.0", "3", "id từ Google Sheets"},
		{"id", "3", "3", "Excel desktop — giữ nguyên"},
		{"status", "1.0", "1", "trạng thái export ra là 1, không phải nhãn"},
		{"service_type_id", "1.0", "1", "FK danh mục"},
		{"SortOrder", "3.0", "3", "cột số không có parser riêng"},
		{"id_old", "10.0", "10", "cột uint"},
		{"child", "2.0", "2", "cột enum"},
		{"is_installment", "1.0", "1", "cột cờ"},

		// Ranh giới an toàn — vượt qua là lệch 1000× số tiền kế toán.
		{"min_price", "50.000", "50.000", "tiền: '.' ngăn nghìn, ParseExcelMoney lo"},
		{"min_price", "1500000.0", "1500000.0", "tiền: luật đọc chỉ nằm ở ParseExcelMoney"},
		{"id_old", "1.000", "1.000", "'.000' là ngăn nghìn = 1000, KHÔNG phải 1"},
		{"SortOrder", "12.345", "12.345", "không phải đuôi .0 → không đụng"},
		{"time_reminder", "8.5", "8.5", "thập phân thật → để parser báo lỗi dòng"},

		// Cột chữ: dịch vụ đặt tên "3.0" là hợp lệ.
		{"name", "3.0", "3.0", "tên dịch vụ"},
		{"code", "3.0", "3.0", "mã dịch vụ"},
		{"code_old", "3.0", "3.0", "mã dịch vụ cũ"},
	}
	for _, tc := range cases {
		if got := normalizeExcelIntCell(tc.col, tc.raw); got != tc.want {
			t.Errorf("normalizeExcelIntCell(%q, %q) = %q, muốn %q — %s", tc.col, tc.raw, got, tc.want, tc.why)
		}
	}
}

// Sau chuẩn hoá, ô số từ Google Sheets phải đi lọt MỌI parser số của import dịch vụ.
// Khoá riêng `status`: giá trị hệ thống export ra là 1, import lại phải nhận 1 — không được đòi
// người dùng gõ nhãn "Đang dùng".
func TestExcelIntCellsPassParsersAfterNormalize(t *testing.T) {
	if v, ok := parseEnumValue(define.ServiceStatusOptions, normalizeExcelIntCell("status", "1.0")); !ok || v != 1 {
		t.Errorf("status \"1.0\" → (%d,%v), muốn (1,true)", v, ok)
	}
	if v, ok := parseEnumValue(define.ServiceChildOptions, normalizeExcelIntCell("child", "2.0")); !ok || v != 2 {
		t.Errorf("child \"2.0\" → (%d,%v), muốn (2,true)", v, ok)
	}
	if id, name := parseCatalogRef(normalizeExcelIntCell("service_type_id", "1.0")); id != 1 || name != "" {
		t.Errorf("service_type_id \"1.0\" → (id:%d,name:%q), muốn (1,\"\") — id phải là id, không phải tên danh mục", id, name)
	}
	if n, ok := parseImportUint(normalizeExcelIntCell("id_old", "10.0")); !ok || n != 10 {
		t.Errorf("id_old \"10.0\" → (%d,%v), muốn (10,true)", n, ok)
	}
	if flag, ok := parseImportBool(normalizeExcelIntCell("is_installment", "1.0")); !ok || flag != 1 {
		t.Errorf("is_installment \"1.0\" → (%d,%v), muốn (1,true)", flag, ok)
	}
	if got := helpers.ToInt64(normalizeExcelIntCell("id", "3.0")); got != 3 {
		t.Errorf("id \"3.0\" → ToInt64 = %d, muốn 3 — 0 làm dòng kẹt giữa update và create", got)
	}
	// Tiền vẫn phải do ParseExcelMoney quyết, và vẫn đúng quy ước VN.
	if amt, ok := helpers.ParseExcelMoney(normalizeExcelIntCell("min_price", "50.000")); !ok || !amt.Equal(decimal.NewFromInt(50000)) {
		t.Errorf("min_price \"50.000\" → (%v,%v), muốn 50000 — cắt nhầm đuôi là lệch 1000×", amt, ok)
	}
}

// Bản ghi cũ trỏ danh mục của thương hiệu khác là dữ liệu sai CÓ SẴN trong DB. File export luôn ghi
// lại nguyên giá trị đó, nên kiểm tồn tại vô điều kiện sẽ khoá cứng bản ghi — user không sửa nổi cả
// cái tên. Cùng lớp sự cố UpdateCustomerCare 2026-07-27.
func TestCatalogValueUnchanged(t *testing.T) {
	typ, unit := 7, 14
	cur := &configServiceDao.Service{ServiceTypeId: &typ, UnitId: &unit}

	cases := []struct {
		col  string
		cur  *configServiceDao.Service
		id   int64
		want bool
		why  string
	}{
		{"service_type_id", cur, 7, true, "nhóm dịch vụ không đổi"},
		{"service_type_id", cur, 8, false, "đổi nhóm dịch vụ"},
		{"unit_id", cur, 14, true, "đơn vị không đổi"},
		{"service_type_id", nil, 7, false, "tạo mới → luôn phải kiểm"},
		{"some_other_id", cur, 5, false, "cột ngoài danh sách → không được coi là không đổi"},
	}
	for _, tc := range cases {
		if got := catalogValueUnchanged(tc.col, tc.cur, tc.id); got != tc.want {
			t.Errorf("catalogValueUnchanged(%q, %v, %d) = %v, muốn %v — %s", tc.col, tc.cur != nil, tc.id, got, tc.want, tc.why)
		}
	}

	// Con trỏ NULL trong DB: không được coi là "trùng" với bất kỳ id nào.
	empty := &configServiceDao.Service{}
	if catalogValueUnchanged("service_type_id", empty, 0) {
		t.Error("service_type_id NULL mà coi là không đổi → bỏ qua kiểm tồn tại")
	}
}

// Ô "Mã dịch vụ" trống = "để hệ thống tự sinh", ĐÚNG như bấm Thêm trên UI — không phải thiếu dữ liệu.
// Nhánh create đã có codegen cho đúng ca này, nhưng nó đứng SAU vòng đọc ô: để `code` ở nhóm
// required là biến codegen thành code chết, vì file mẫu/file export LUÔN có cột mã nên mọi dòng
// thêm mới đều bị chặn (sự cố ARA Plus 2026-09-18: "Lỗi dòng 6: Cột code bắt buộc").
func TestServiceImportEmptyCodeMeansAutoGenerate(t *testing.T) {
	if serviceImportRequiredCols["code"] {
		t.Error("code nằm ở nhóm required → dòng thêm mới không bao giờ chạm tới codegen")
	}
	if !serviceImportKeepOnEmptyCols["code"] {
		t.Error("code phải ở nhóm keep-on-empty: dòng UPDATE ô trống phải GIỮ mã đang lưu, " +
			"không được ghi chuỗi rỗng xuống cột NOT NULL UNIQUE")
	}
	// `name` không có đường tự sinh nào nên vẫn phải bắt buộc.
	if !serviceImportRequiredCols["name"] {
		t.Error("name phải vẫn bắt buộc")
	}
}

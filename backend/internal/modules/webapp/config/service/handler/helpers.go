package handler

import (
	"fmt"
	"mime/multipart"
	"regexp"
	"strconv"
	"strings"

	"erpCluvix/pkg/define"
	pkgExport "erpCluvix/pkg/export"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/validation"

	"github.com/gin-gonic/gin"
)

//nolint:unused
func requireName(c *gin.Context, name string) bool {
	if strings.TrimSpace(name) == "" {
		response.Validate(c, i18n.T(c, "name_required"))
		return false
	}
	return true
}

//nolint:unused
func requireFields(c *gin.Context, pairs [][2]any) bool {
	v := validation.New()
	for _, p := range pairs {
		v.Required(p[0], strFromAny(p[1]))
	}
	if v.HasErrors() {
		response.Validate(c, v.Error())
		return false
	}
	return true
}

//nolint:unused
func strFromAny(a any) string {
	if s, ok := a.(string); ok {
		return s
	}
	return ""
}

func parseUint(s string) uint {
	var id uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + uint(c-'0')
		}
	}
	return id
}

const MaxImportRows = 5000

func allEmpty(values []string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// serviceImportHeaderToCol — allowlist cột nhận từ file Excel import dịch vụ, map nhãn tiếng Việt
// (dòng 1) → tên cột DB. Tập GIÁ TRỊ của map cũng chính là allowlist field key (dòng 2), nên mọi
// cột muốn import được PHẢI có mặt ở đây — cột ngoài allowlist bị bỏ qua (chống column injection).
var serviceImportHeaderToCol = map[string]string{
	"id": "id", "id cũ": "id_old", "Tên dịch vụ": "name", "Mã dịch vụ": "code",
	"Mã dịch vụ cũ": "code_old", "Trạng thái": "status", "Vị trí": "SortOrder",
	"Danh mục": "serviceCategory_id", "Nhóm dịch vụ": "service_type_id",
	"Giá tối thiểu": "min_price", "Giá tối đa": "max_price",
	"Đơn vị": "unit_id", "Nhóm tuổi": "child",
	"Trả góp": "is_installment", "Nhắc lịch": "time_reminder",
	"HH tư vấn viên cũ": "commission_telesale", "HH tư vấn viên mới": "commission_telesale_new", "Loại HH tư vấn viên": "commission_telesale_type",
	// Header cũ — giữ tương thích ngược cho template Excel đã tải trước khi đổi nhãn (field code
	// commission_telesale* không đổi, chỉ đổi tên hiển thị "Telesale" → "Tư vấn viên").
	"HH telesale cũ": "commission_telesale", "HH telesale mới": "commission_telesale_new", "Loại HH telesale": "commission_telesale_type",
	// Story-39 (D23): cột "Thương hiệu" KHÔNG còn trong allowlist — file mẫu/export cũ còn cột này
	// thì cột bị bỏ qua như mọi cột lạ (không lỗi, không đổi dữ liệu).
}

// importCol — 1 cột nhận từ file: vị trí trong dòng + tên cột DB. Dùng slice theo thứ tự index
// tăng dần thay vì map[int]string: duyệt map cho thứ tự NGẪU NHIÊN, nên khi file có 2 cột cùng map
// về một field (vd nhãn cũ "HH telesale cũ" nằm cạnh nhãn mới "HH tư vấn viên cũ") thì giá trị nào
// thắng là ngẫu nhiên theo từng dòng, từng lần chạy.
type importCol struct {
	idx int
	col string
}

// resolveServiceImportColumns nhận diện dạng header của file import dịch vụ.
// Trả (danh sách cột theo thứ tự index, chỉ số dòng dữ liệu đầu tiên, tên cột bị lặp).
//
// File hỗ trợ 3 dạng:
//   - 2 dòng header: dòng 1 nhãn hiển thị, dòng 2 TÊN CỘT DB — dạng do export.ExportService sinh ra,
//     cho phép export → sửa tay → import ngược lại. Dữ liệu từ dòng 3 (dataStart = 2).
//   - 1 dòng nhãn tiếng Việt: template cũ + mọi file user đã tải trước đây. Dữ liệu từ dòng 2.
//   - 1 dòng field key: user xoá dòng nhãn khỏi file export. Dữ liệu từ dòng 2.
//
// Nhận dạng: dòng 2 khớp >= 2 tên cột hợp lệ VÀ khớp không ít hơn số cột nhận ra ở dòng nhãn thì đó
// là field-key row — một dòng dữ liệu thật gần như không thể vượt ngưỡng này. Nếu không, thử coi
// chính dòng 1 là field-key row trước khi rơi về nhãn.
// Trả danh sách rỗng nghĩa là không nhận ra cột nào → caller PHẢI báo lỗi, KHÔNG import 0 dòng âm thầm.
// dupCol khác rỗng → file có cột lặp, caller phải từ chối thay vì để giá trị thắng thua ngẫu nhiên.
func resolveServiceImportColumns(rows [][]string) (cols []importCol, dataStart int, dupCol string) {
	if len(rows) == 0 {
		return nil, 1, ""
	}
	validCols := map[string]bool{}
	for _, col := range serviceImportHeaderToCol {
		validCols[col] = true
	}
	byLabel := func(line []string) []importCol {
		out := make([]importCol, 0, len(line))
		for j, h := range line {
			if col, ok := serviceImportHeaderToCol[strings.TrimSpace(h)]; ok {
				out = append(out, importCol{j, col})
			}
		}
		return out
	}
	byKey := func(line []string) []importCol {
		out := make([]importCol, 0, len(line))
		for j, h := range line {
			if col := strings.TrimSpace(h); validCols[col] {
				out = append(out, importCol{j, col})
			}
		}
		return out
	}

	labelCols := byLabel(rows[0])
	var keyCols []importCol
	if len(rows) > 1 {
		keyCols = byKey(rows[1])
	}
	switch {
	case len(keyCols) >= 2 && len(keyCols) >= len(labelCols):
		cols, dataStart = keyCols, 2
	default:
		// Dòng 1 có thể chính là field-key row (user xoá dòng nhãn). Chỉ nhận khi nó khớp field key
		// nhiều hơn hẳn cách đọc theo nhãn — tránh cướp mất file template cũ hợp lệ.
		if row0Keys := byKey(rows[0]); len(row0Keys) >= 2 && len(row0Keys) > len(labelCols) {
			cols, dataStart = row0Keys, 1
		} else {
			cols, dataStart = labelCols, 1
		}
	}

	seen := map[string]bool{}
	for _, ic := range cols {
		if seen[ic.col] {
			return cols, dataStart, ic.col
		}
		seen[ic.col] = true
	}
	return cols, dataStart, ""
}

// readImportRows — mỏng, uỷ quyền cho pkg/export.ReadImportRows (luật đọc file nằm cạnh luật ghi
// file trong cùng package đó). Giữ tên cũ để không phải sửa loạt callsite + test đang có.
func readImportRows(fh *multipart.FileHeader) ([][]string, error) {
	return pkgExport.ReadImportRows(fh)
}

// --- Phân loại cột theo cách xử lý ô TRỐNG -------------------------------------------------
// File export là template import, nên ô trống là chủ ý của user chứ không phải thiếu dữ liệu:
// cột có mặt + ô trống = XOÁ giá trị đang lưu. Ba nhóm dưới đây chia hết allowlist (có test khoá).

// serviceImportKeepOnEmptyCols — ô trống giữ nguyên giá trị đang lưu, KHÔNG xoá và KHÔNG báo lỗi.
// `id` trống nghĩa là tạo mới; `status` có bản ghi cũ đang NULL thật trong DB nên file export của nó
// vốn đã trống — báo lỗi hay xoá đều là phạt oan file người dùng không hề sửa.
// (Cột "Khoa thực hiện" từng nằm đây — đã bỏ cùng khái niệm khoa ở story-12 crm-standalone, D16.)
var serviceImportKeepOnEmptyCols = map[string]bool{
	"id": true, "status": true,
	"code": true,
}

// serviceImportRequiredCols — cột NOT NULL/UNIQUE, ô trống là lỗi dòng tường minh.
// Chỉ đặt cột vào đây khi ĐÃ kiểm rằng không bản ghi nào trong DB để trống nó — nếu không, file do
// chính hệ thống export ra sẽ bị chặn (bài học cột khoa cũ, 61/271 dịch vụ trống).
//
// `code` KHÔNG nằm ở đây (sửa 2026-09-18, sự cố ARA Plus) dù schema là NOT NULL UNIQUE: ô trống ở
// cột mã KHÔNG phải thiếu dữ liệu mà là "để hệ thống tự sinh", đúng như đường form. Nhánh create đã
// có sẵn codegen cho đúng ca này (2-step insert: temp code → Create → codegen.Generate → update mã
// thật) nhưng đứng SAU vòng đọc ô, nên required-check chặn trước làm nhánh đó thành code chết: file
// mẫu và file export LUÔN có cột "Mã dịch vụ", nên mọi dòng thêm mới đều bị từ chối và người dùng
// buộc phải tự bịa mã — trái hẳn hành vi thêm dịch vụ trên UI. Ô trống ở dòng UPDATE thì giữ nguyên
// mã đang lưu (nhóm keep-on-empty), không bao giờ ghi chuỗi rỗng xuống cột NOT NULL.
var serviceImportRequiredCols = map[string]bool{
	"name": true,
}

// serviceImportClearOnEmptyCols — cột nullable: ô trống ghi giá trị "rỗng" tương ứng.
// nil → NULL (cột con trỏ trong entity); "" → chuỗi rỗng (cột kiểu string như code_old).
var serviceImportClearOnEmptyCols = map[string]interface{}{
	"id_old": nil, "code_old": "", "SortOrder": nil,
	"serviceCategory_id": nil, "service_type_id": nil,
	"min_price": nil, "max_price": nil, "unit_id": nil,
	"child": nil, "is_installment": nil, "time_reminder": nil,
	"commission_telesale": nil, "commission_telesale_new": nil, "commission_telesale_type": nil,
}

// serviceImportMoneyCols — cột decimal: parse qua helpers.ParseExcelMoney thay vì đẩy chuỗi thô cho
// MySQL tự cast. "1.000" theo quy ước VN là một nghìn nhưng cast thẳng ra 1.0 (lệch 1000×).
var serviceImportMoneyCols = map[string]bool{
	"min_price": true, "max_price": true,
	"commission_telesale": true, "commission_telesale_new": true,
}

// --- Kiểu giá trị của các cột KHÔNG phải tiền ------------------------------------------------
// Không có các bảng này thì chuỗi thô đi thẳng xuống MySQL: "có" ghi vào cột `is_installment` (INT) làm
// nổ lỗi 1366 ở tầng driver → rollback CẢ file với thông báo không chỉ ra dòng nào sai.

// serviceImportBoolCols — cột cờ chỉ nhận 0/1 (chấp nhận vài cách viết thường gặp của Excel).
var serviceImportBoolCols = map[string]bool{
	"is_installment": true,
}

// serviceImportUintCols — cột số nguyên không âm.
// `id_old` nằm đây để ô rác ("abc", "36 5") bị chặn ngay tại dòng thay vì đẩy chuỗi xuống cột INT
// cho MySQL trả 1366 (truncated) và rollback cả file; đồng thời chuẩn hoá về số để so trùng
// id_old — xem checkIDOld trong ImportServiceFromExcel.
var serviceImportUintCols = map[string]bool{
	"id_old": true,
}

// parseImportBool nhận các cách viết cờ 0/1 mà người dùng hay gõ trong Excel.
// Trả (giá trị, hợp lệ) — KHÔNG đoán bừa: chuỗi lạ phải báo lỗi dòng thay vì mặc định về 0.
func parseImportBool(raw string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "x", "có", "co", "yes", "y":
		return 1, true
	case "0", "false", "không", "khong", "no", "n":
		return 0, true
	}
	return 0, false
}

// parseImportUint nhận số nguyên không âm. Dấu chấm/phẩy CHỈ được chấp nhận khi đứng đúng vị trí
// phân nhóm nghìn ("1.000", "12,345,678") — đó là dạng Excel hay tự thêm khi định dạng số.
//
// CỐ Ý từ chối "8.5"/"8,5": ở cột số nguyên đó là người dùng gõ nhầm, mà hai quy ước vùng miền cho
// hai kết quả trái ngược (85 nếu coi là phân nhóm, 8 nếu coi là thập phân) — đoán bừa kiểu nào cũng
// sai âm thầm. Báo lỗi dòng để họ sửa.
var thousandGroupedInt = regexp.MustCompile(`^[0-9]{1,3}([.,][0-9]{3})+$`)

func parseImportUint(raw string) (int64, bool) {
	v := strings.NewReplacer(" ", "", "\u00a0", "").Replace(strings.TrimSpace(raw))
	if v == "" {
		return 0, false
	}
	if strings.ContainsAny(v, ".,") {
		if !thousandGroupedInt.MatchString(v) {
			return 0, false
		}
		v = strings.NewReplacer(".", "", ",", "").Replace(v)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// --- Ô số nguyên do bảng tính ghi ra dạng "3.0" ----------------------------------------------
// ReadImportRows CỐ Ý đọc bằng excelize.Options{RawCellValue: true} (bắt buộc, xem
// skill patterns-bulk-import §6: không có nó thì ô ngày bị render lại theo locale và parse fail).
// Hệ quả kèm theo: ô nhận đúng chuỗi thô nằm trong XML, mà Google Sheets/LibreOffice ghi MỌI số
// nguyên thành "3.0" chứ không phải "3". Excel bản desktop ghi "3" nên bẫy này chỉ lộ ra với file
// đi qua bảng tính khác — rất dễ tưởng là dữ liệu user sai.
//
// Không chuẩn hoá thì "3.0" trượt hết mọi parser số: helpers.ToInt64 (chỉ ParseInt) trả 0,
// parseCatalogRef coi là TÊN danh mục, parseEnumValue coi là nhãn, parseImportUint từ chối.
// Sự cố thật 2026-09-18 (ARA Plus): ô id "3.0" → ToInt64 = 0 → dòng không vào importIDs nên câu
// query tìm bản ghi KHÔNG BAO GIỜ chạy, mà cũng không rơi về nhánh tạo mới (rawID != "") → báo
// "Không tồn tại" ở dòng đầu tiên và rollback cả file. Cùng file đó ô Trạng thái "1.0" bị đòi
// nhập nhãn "Đang dùng" dù 1 mới là giá trị hệ thống vừa export ra.
//
// Cách xử lý thống nhất với import khách hàng (`import/customer/handler/import.go` §intCols dùng
// ParseFloat để "nhận cả 2 lẫn 2.0") — xem skill working-on-ops/reference/import.md.
//
// CHỈ cắt đuôi ".0" MỘT chữ số 0. Đây là ranh giới an toàn bắt buộc, không phải chi tiết thẩm mỹ:
// quy ước VN dùng "." ngăn nghìn với ĐÚNG 3 chữ số ("1.000" = một nghìn, "50.000" = năm mươi
// nghìn). Nới thành `\.0+$` sẽ biến "50.000" thành "50" — đúng lớp lỗi lệch 1000× mà skill
// patterns-bulk-import §7 cảnh báo. "1.0" thì không thể là ngăn nghìn nên không nhập nhằng.
var excelIntWithZeroFraction = regexp.MustCompile(`^(-?[0-9]+)\.0$`)

// serviceImportRawTextCols — cột chứa CHỮ, không được chuẩn hoá số: dịch vụ tên "3.0" là hợp lệ.
var serviceImportRawTextCols = map[string]bool{
	"name": true, "code": true, "code_old": true,
}

// normalizeExcelIntCell — "3.0" → "3" cho các cột mang ý nghĩa SỐ.
// Cột tiền tự loại trừ ngay trong hàm: luật đọc tiền nằm trọn trong helpers.ParseExcelMoney (một
// nguồn sự thật), hàm này không được phép chạm vào.
func normalizeExcelIntCell(col, raw string) string {
	if serviceImportRawTextCols[col] || serviceImportMoneyCols[col] {
		return raw
	}
	if m := excelIntWithZeroFraction.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	return raw
}

// --- Cột danh mục & enum trong file import ---------------------------------------------------
// File mẫu sinh runtime gắn dropdown với giá trị dạng "12 — Khoa Nội" (xem export.CatalogValue).
// Người dùng cũng có thể tự gõ id trần hoặc chỉ tên, hoặc dán lại file export cũ (chỉ có id) —
// cả 3 dạng phải cho ra cùng một kết quả, nếu không thì dropdown chỉ là bẫy mới.

// catalogRefSep — dấu ngăn giữa id và tên trong giá trị dropdown. Phải khớp export.CatalogValue.
const catalogRefSep = " — "

// parseCatalogRef bóc ô danh mục thành (id, tên).
//
//	"12"                  → id 12
//	"12 — Khoa Nội"       → id 12   (dạng dropdown; id thắng vì không nhập nhằng)
//	"3 — VAT 8% (8%)"     → id 3
//	"Khoa Nội"            → tên "Khoa Nội" (caller phải tra DB)
//
// Trả id = 0 nghĩa là chỉ có tên.
func parseCatalogRef(raw string) (int64, string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, ""
	}
	if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
		return id, ""
	}
	// Chấp nhận cả gạch ngang dài lẫn gạch nối thường: người dùng gõ tay hiếm khi ra đúng "—".
	for _, sep := range []string{catalogRefSep, " - ", " – "} {
		if i := strings.Index(v, sep); i > 0 {
			head := strings.TrimSpace(v[:i])
			if id, err := strconv.ParseInt(head, 10, 64); err == nil && id > 0 {
				return id, strings.TrimSpace(v[i+len(sep):])
			}
		}
	}
	return 0, v
}

// serviceImportEnumCols — cột enum → bộ nhãn hợp lệ. Nhãn là MỘT nguồn sự thật ở pkg/define, dùng
// chung với sheet Danh_muc và dropdown của file mẫu: file mẫu ghi "Năm" mà importer chỉ hiểu số 2
// thì user chọn từ dropdown xong import lại lỗi.
var serviceImportEnumCols = map[string][]define.ServiceEnumOption{
	"serviceCategory_id":       define.ServiceCategoryOptions,
	"child":                    define.ServiceChildOptions,
	"commission_telesale_type": define.ServiceCommissionTypeOptions,
	"status":                   define.ServiceStatusOptions,
}

// parseEnumValue nhận cả số ("2") lẫn nhãn ("Năm"), không phân biệt hoa thường.
// Số KHÔNG nằm trong bộ giá trị hợp lệ bị từ chối: cột enum mà nhận bừa số lạ thì FE render ra
// "Không rõ" và không ai truy được lỗi từ đâu.
func parseEnumValue(opts []define.ServiceEnumOption, raw string) (int, bool) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil {
		for _, o := range opts {
			if o.Value == n {
				return n, true
			}
		}
		// 0 = "chưa xác định" của dữ liệu cũ, KHÔNG có trong danh sách chọn nhưng ĐANG NẰM TRONG DB:
		// `service.child` mặc định 0 (baseline migration), và dữ liệu nhập từ hệ thống cũ cũng có
		// 0 ở vài cột loại. File export ghi thẳng giá trị DB, nên từ chối 0 là làm hỏng đúng luồng
		// "export → sửa vài ô → import lại" trên phần lớn dịch vụ hiện có — user không hề sửa ô đó
		// mà cả file bị rollback. Giữ nguyên 0 thay vì đổi thành NULL: đây là bản ghi user không đụng tới.
		if n == 0 {
			return 0, true
		}
		return 0, false
	}
	// Giá trị dropdown có thể còn phần id ở đầu nếu user tự dán ("1 — Người lớn") — bóc trước.
	if _, name := parseCatalogRef(v); name != "" {
		v = name
	}
	lower := strings.ToLower(v)
	for _, o := range opts {
		if strings.ToLower(o.Label) == lower {
			return o.Value, true
		}
	}
	return 0, false
}

// enumLabelList — chuỗi liệt kê nhãn hợp lệ, để thông báo lỗi nói thẳng phải điền gì.
func enumLabelList(opts []define.ServiceEnumOption) string {
	parts := make([]string, len(opts))
	for i, o := range opts {
		parts[i] = fmt.Sprintf("%s (%d)", o.Label, o.Value)
	}
	return strings.Join(parts, ", ")
}

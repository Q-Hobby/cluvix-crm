package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"erpCluvix/pkg/cleanup"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type Config struct {
	FileName string
	Headers  []string
	// FieldKeys (tuỳ chọn) — dòng header thứ 2 chứa TÊN CỘT DB, ghi ngay dưới Headers.
	// Bật khi file export cần import ngược lại được: importer map cột theo key máy đọc
	// (dòng 2) thay vì nhãn tiếng Việt (dòng 1), nên đổi nhãn hiển thị không phá import.
	// Độ dài PHẢI bằng Headers. Bỏ trống → file giữ nguyên 1 dòng header như cũ.
	FieldKeys    []string
	CSVThreshold int
	// BoundedRows — kết quả CHẮC CHẮN nhỏ hơn CSVThreshold dòng (báo cáo nhóm theo ngày/chi nhánh…) ⇒ bỏ
	// câu `SELECT COUNT(*) FROM (<query>)` và ghi thẳng Excel; 0 dòng vẫn trả lỗi "Không có dữ liệu".
	// Lý do: COUNT bọc NGUYÊN query — query GROUP BY/UNION nặng bị chạy HAI lần mỗi lần export
	// (/check-scale 2026-09-14, export Doanh thu theo ngày @600.000 phiếu: 1 năm 4,0s ⇒ nửa là COUNT).
	// KHÔNG bật cho export dạng danh sách: vượt ngưỡng mà vẫn ghi Excel là giữ cả file trong RAM.
	BoundedRows bool
	// CountRows (tuỳ chọn) — handler đã có cách đếm RẺ hơn `SELECT COUNT(*) FROM (<query>)` (vd lượt quét song song
	// dùng chung với màn danh sách) ⇒ gọi hàm này thay câu COUNT bọc nguyên query. Số trả về PHẢI đếm đúng tập dòng
	// của `query` — nó chọn định dạng (Excel/CSV) và chặn "không có dữ liệu". Bỏ qua khi BoundedRows.
	// /check-scale 2026-09-17, export Công nợ sản phẩm toàn chi nhánh: COUNT bọc query 4,8s ⇒ lượt quét 1,3s.
	CountRows func() (int64, error)
	// BatchSize — KHÔNG còn ảnh hưởng đường đọc dữ liệu, GIỮ LẠI chỉ để không phá hợp đồng `Config`
	// với ~80 hàm Export* đang dùng. Trước đây nó là cỡ trang của vòng LIMIT/OFFSET; cơ chế đó đã
	// bị thay bằng MỘT con trỏ duy nhất (xem `streamRows`), nên đặt giá trị nào cũng như nhau.
	// Đừng chỉnh nó để "tinh chỉnh hiệu năng" — sẽ là no-op im lặng.
	BatchSize   int
	ExportDir   string
	DownloadURL string
	// OwnerID — user tạo file. File ghi vào `<ExportDir>/<OwnerID>/` và endpoint tải có auth
	// (`GET /api/download/export/:name`) CHỈ tra trong thư mục của user đang đăng nhập ⇒ người khác
	// nhận 404. 0 = ghi thẳng vào ExportDir (chỉ dành cho test gọi trực tiếp; không tải được qua API).
	OwnerID   uint
	OnSuccess func(rowCount int64) // optional: called after successful export
	// ExtraSheets (tuỳ chọn) — HÀM dựng các sheet TRA CỨU ghi kèm SAU sheet dữ liệu (không phải dữ
	// liệu export). Dùng cho danh mục mà người dùng phải tra id khi sửa file rồi import ngược lại:
	// danh mục là per-tenant nên không thể nằm sẵn trong file template tĩnh.
	//
	// Là HÀM chứ không phải dữ liệu sẵn vì nó tốn nhiều query, mà nhánh CSV (khi vượt CSVThreshold)
	// không có khái niệm sheet nên sẽ vứt đi toàn bộ — truyền dữ liệu sẵn là luôn trả giá kể cả khi
	// chắc chắn không dùng. Chỉ được gọi ở nhánh Excel, gọi đúng 1 lần.
	//
	// Sheet dữ liệu LUÔN giữ vị trí đầu tiên — importer đọc f.GetSheetName(0).
	ExtraSheets func() []ExtraSheet
}

// ExtraSheet — 1 sheet tra cứu tĩnh (dữ liệu đã nằm sẵn trong bộ nhớ, không stream theo batch).
type ExtraSheet struct {
	Name    string
	Headers []string
	Rows    [][]interface{}
}

type RowMapper func(row map[string]interface{}) []interface{}

// headerRows trả về các dòng header sẽ ghi lên đầu file: luôn có dòng nhãn, thêm dòng field key
// khi cfg.FieldKeys được set. Tách riêng để cả 3 nhánh ghi (Excel 1 sheet, CSV, multi-sheet) dùng
// chung đúng 1 định nghĩa — trước đây mỗi nhánh tự ghi header nên dễ lệch nhau âm thầm.
func headerRows(cfg Config) [][]string {
	rows := [][]string{cfg.Headers}
	if len(cfg.FieldKeys) > 0 {
		rows = append(rows, cfg.FieldKeys)
	}
	return rows
}

// validateConfig chặn cấu hình sai ngay tại cửa thay vì sinh ra file lệch cột.
// FieldKeys lệch độ dài với Headers = dòng key không còn khớp cột dữ liệu → importer đọc ngược file
// sẽ ghi giá trị cột A vào cột B, sai âm thầm và không có exception nào bắt được.
func validateConfig(cfg Config) error {
	if len(cfg.FieldKeys) > 0 && len(cfg.FieldKeys) != len(cfg.Headers) {
		return fmt.Errorf("export config error: FieldKeys (%d) phải cùng độ dài với Headers (%d)",
			len(cfg.FieldKeys), len(cfg.Headers))
	}
	return nil
}

// Unsanitize đảo ngược sanitize() cho luồng đọc-ngược-file-export (export → user sửa → import lại).
// sanitize() thêm dấu nháy đơn vào đầu chuỗi bắt đầu bằng ký tự có thể bị Excel hiểu là công thức;
// dấu này nằm TRONG giá trị ô nên nếu import không bóc ra thì tên dịch vụ "- Combo A" sẽ thành
// "'- Combo A", và mỗi vòng export/import lại chồng thêm một dấu nữa.
// CHỈ bóc khi ký tự ngay sau đúng là ký tự mà sanitize() nhắm tới — tên bắt đầu bằng nháy đơn vì
// lý do khác (hiếm nhưng hợp lệ) phải giữ nguyên.
func Unsanitize(s string) string {
	if len(s) < 2 || s[0] != '\'' {
		return s
	}
	if strings.ContainsAny(s[1:2], "=+-@\t\r") {
		return s[1:]
	}
	return s
}

// errNoExportData — cùng thông điệp cho cả đường có COUNT và đường BoundedRows.
var errNoExportData = fmt.Errorf("Không có dữ liệu để export")

func Export(query *gorm.DB, cfg Config, mapper RowMapper) (string, error) {
	if err := validateConfig(cfg); err != nil {
		return "", err
	}
	if cfg.CSVThreshold == 0 {
		cfg.CSVThreshold = 10000
	}
	if cfg.ExportDir == "" {
		cfg.ExportDir = DefaultDir
	}

	// Dùng subquery wrap để Count luôn trả đúng số row,
	// kể cả khi query có GROUP BY + HAVING reference SELECT aliases.
	ts := time.Now().Format("2006-01-02-15-04-05")
	if cfg.BoundedRows {
		url, written, err := exportExcel(query, cfg, mapper, ts)
		if err == nil && cfg.OnSuccess != nil {
			cfg.OnSuccess(written)
		}
		return url, err
	}

	var total int64
	if cfg.CountRows != nil {
		n, err := cfg.CountRows()
		if err != nil {
			return "", fmt.Errorf("count error: %v", err)
		}
		total = n
	} else if err := query.Session(&gorm.Session{NewDB: true}).
		Raw("SELECT COUNT(*) FROM (?) AS count_q", query.Session(&gorm.Session{})).
		Scan(&total).Error; err != nil {
		return "", fmt.Errorf("count error: %v", err)
	}
	if total == 0 {
		return "", errNoExportData
	}

	// `total` CHỈ còn hai việc: chặn "không có dữ liệu" (ở trên) và chọn định dạng (ngay dưới).
	// Nó KHÔNG còn điều khiển vòng lặp đọc — số dòng ghi ra đến từ chính con trỏ.
	var url string
	var written int64
	var exportErr error
	if total < int64(cfg.CSVThreshold) {
		url, written, exportErr = exportExcel(query, cfg, mapper, ts)
	} else {
		url, written, exportErr = exportCSVCounted(query, cfg, mapper, ts)
	}
	// OnSuccess nhận SỐ DÒNG THẬT ĐÃ GHI, không phải `total`. Dữ liệu có thể đổi giữa câu COUNT và
	// lúc mở con trỏ; file đi theo dữ liệu thật, và `history_export.quantityRecord` phải khớp file
	// chứ không khớp COUNT. KHÔNG cắt bớt/đệm thêm cho khớp `total` — làm vậy là tái tạo đúng lỗi
	// bỏ-sót-dòng mà thay đổi này sinh ra để diệt.
	if exportErr == nil && cfg.OnSuccess != nil {
		cfg.OnSuccess(written)
	}
	return url, exportErr
}

func exportExcel(query *gorm.DB, cfg Config, mapper RowMapper, ts string) (string, int64, error) {
	filePath, fileName, err := prepareFile(cfg, ts, "xlsx")
	if err != nil {
		return "", 0, err
	}

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Sheet1"

	hdr := headerRows(cfg)
	for r, line := range hdr {
		for i, h := range line {
			cell, _ := excelize.CoordinatesToCellName(i+1, r+1)
			if err := f.SetCellValue(sheet, cell, h); err != nil {
				return "", 0, fmt.Errorf("set header error: %v", err)
			}
		}
	}
	rowIdx := len(hdr) + 1

	written, err := streamRows(query, func(r map[string]interface{}) error {
		data := mapper(r)
		for i, val := range data {
			cell, _ := excelize.CoordinatesToCellName(i+1, rowIdx)
			if err := f.SetCellValue(sheet, cell, sanitize(val)); err != nil {
				return fmt.Errorf("set cell error: %v", err)
			}
		}
		rowIdx++
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	// BoundedRows bỏ COUNT ⇒ "không có dữ liệu" chỉ biết được ở đây, TRƯỚC khi ghi file rỗng ra đĩa.
	if cfg.BoundedRows && written == 0 {
		return "", 0, errNoExportData
	}

	if cfg.ExtraSheets != nil {
		if err := writeExtraSheets(f, cfg.ExtraSheets()); err != nil {
			return "", 0, err
		}
	}

	if err := writeFileAtomically(filePath, func(out *os.File) error {
		return f.Write(out)
	}); err != nil {
		return "", 0, err
	}

	cleanup.ScheduleFileDelete(filePath, 30*time.Minute)
	return DownloadLink(cfg.DownloadURL, fileName), written, nil
}

// writeExtraSheets ghi các sheet tra cứu vào CUỐI file (sau sheet dữ liệu) và không đổi sheet đang
// active — mở file lên vẫn thấy dữ liệu trước, sheet tra cứu là thứ user chủ động bấm sang.
func writeExtraSheets(f *excelize.File, sheets []ExtraSheet) error {
	for _, sh := range sheets {
		name := SafeSheetName(sh.Name)
		if _, err := f.NewSheet(name); err != nil {
			return fmt.Errorf("new sheet %s error: %v", name, err)
		}
		rows := append([][]interface{}{toAny(sh.Headers)}, sh.Rows...)
		for r, line := range rows {
			for i, val := range line {
				cell, _ := excelize.CoordinatesToCellName(i+1, r+1)
				if err := f.SetCellValue(name, cell, sanitize(val)); err != nil {
					return fmt.Errorf("set cell error on sheet %s: %v", name, err)
				}
			}
		}
	}
	return nil
}

func toAny(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// exportCSV giữ NGUYÊN chữ ký cũ vì `multi_sheet.go` gọi nó và trả thẳng kết quả — đổi chữ ký ở
// đây sẽ buộc phải sửa file đó, ngoài phạm vi story. Bản đếm nằm ở exportCSVCounted.
func exportCSV(query *gorm.DB, cfg Config, mapper RowMapper, ts string) (string, error) {
	url, _, err := exportCSVCounted(query, cfg, mapper, ts)
	return url, err
}

func exportCSVCounted(query *gorm.DB, cfg Config, mapper RowMapper, ts string) (string, int64, error) {
	filePath, fileName, err := prepareFile(cfg, ts, "csv")
	if err != nil {
		return "", 0, err
	}

	var written int64
	err = writeFileAtomically(filePath, func(file *os.File) error {
		if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return fmt.Errorf("write BOM error: %v", err)
		}

		writer := csv.NewWriter(file)
		for _, line := range headerRows(cfg) {
			if err := writer.Write(line); err != nil {
				return fmt.Errorf("write headers error: %v", err)
			}
		}

		n, err := streamRows(query, func(r map[string]interface{}) error {
			data := mapper(r)
			record := make([]string, len(data))
			for i, val := range data {
				record[i] = fmt.Sprintf("%v", sanitize(val))
			}
			if err := writer.Write(record); err != nil {
				return fmt.Errorf("write row error: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		writer.Flush()
		// csv.Writer đệm trong bộ nhớ — lỗi ghi thật chỉ lộ ra ở đây. Không kiểm là mất dữ liệu im lặng.
		if err := writer.Error(); err != nil {
			return fmt.Errorf("flush csv error: %v", err)
		}
		written = n
		return nil
	})
	if err != nil {
		return "", 0, err
	}

	cleanup.ScheduleFileDelete(filePath, 30*time.Minute)
	return DownloadLink(cfg.DownloadURL, fileName), written, nil
}

// writeFileAtomically ghi ra file TẠM có tên ngẫu nhiên rồi mới `rename` sang tên cuối khi `write`
// thành công; lỗi ở bất kỳ điểm nào ⇒ xoá file tạm và KHÔNG để lại gì mang tên cuối.
//
// Trước đây nhánh CSV `os.Create` thẳng vào tên cuối rồi ghi dần: lỗi giữa chừng (mất kết nối, đĩa
// đầy) để lại **file cụt** nằm vĩnh viễn trong thư mục export — `cleanup.ScheduleFileDelete` chỉ
// được đặt ở nhánh thành công nên không ai dọn nó. Tên file lại đoán được từ mẫu
// `<FileName>-<timestamp>`, nên vẫn có đường tải về một file thiếu dữ liệu mà không dấu hiệu gì.
//
// Tên tạm dùng `os.CreateTemp` (hậu tố ngẫu nhiên, có dấu chấm ở đầu) nên KHÔNG suy ra được từ tên
// cuối; endpoint tải (`GET /api/download/export/:name`) còn từ chối mọi tên bắt đầu bằng `.`
// (ValidFileName), nên file tạm không bao giờ được phục vụ ra ngoài.
func writeFileAtomically(finalPath string, write func(f *os.File) error) error {
	dir := filepath.Dir(finalPath)
	f, err := os.CreateTemp(dir, ".export-*.part")
	if err != nil {
		return fmt.Errorf("create temp file error: %v", err)
	}
	tmpPath := f.Name()

	// Dọn bằng DEFER, không phải bằng nhánh `if err`: `write` gọi xuống `mapper` của caller, mà
	// mapper CÓ THỂ PANIC — panic unwind bỏ qua mọi nhánh `if`, chỉ `defer` sống sót. Đã đo: bản
	// đầu dọn trong nhánh `if` để lại file tạm mỗi khi mapper panic ở nhánh CSV.
	committed := false
	defer func() {
		if !committed {
			_ = f.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := write(f); err != nil {
		return err
	}
	// Close TRƯỚC rename: dữ liệu còn trong buffer của OS phải được đẩy xuống trước khi file mang
	// tên cuối (tên cuối = tín hiệu "file đã hoàn chỉnh").
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file error: %v", err)
	}
	// os.CreateTemp tạo file 0600; tên cuối phải giữ quyền như `os.Create` cũ (0666 & umask) để
	// tầng phục vụ file tĩnh đọc được y như trước.
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return fmt.Errorf("chmod temp file error: %v", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename temp file error: %v", err)
	}
	committed = true
	return nil
}

// StreamRows là bản công khai của cơ chế đọc một-lần-quét, cho các export KHÔNG đi qua `Export()`
// (nhóm tự dựng file bằng excelize: kho, dược, quỹ...). Dùng nó thay cho vòng
// `.Offset(n).Limit(m)` tự chế: vòng đó lấy MỖI TRANG bằng một câu riêng, nên khi tập kết quả đổi
// giữa hai trang thì cửa sổ trượt và dòng vô can bị nhảy qua — im lặng, không lỗi.
//
// `fn` được gọi cho từng dòng theo đúng thứ tự query trả về; trả lỗi từ `fn` để dừng sớm.
func StreamRows(query *gorm.DB, fn func(map[string]interface{}) error) (int64, error) {
	return streamRows(query, fn)
}

// streamRows duyệt TOÀN BỘ tập kết quả bằng MỘT con trỏ duy nhất và gọi `fn` cho từng dòng, trả về
// số dòng đã xử lý.
//
// Thay cho cơ chế cũ: bọc query gốc thành derived table rồi phân trang bằng LIMIT/OFFSET, tức chạy
// lại toàn bộ query gốc cho MỖI batch 5000 dòng. Cơ chế cũ có hai lỗi cùng gốc, đều đã đo ở quy mô:
//
//  1. Chi phí bậc hai: mỗi batch trả tiền cho toàn bộ phần OFFSET bỏ qua. Export 262.144 dòng đọc
//     114.164.534 dòng InnoDB, gấp 28,3 lần một lần quét (4.032.165).
//  2. BỎ SÓT DÒNG IM LẶNG: `LIMIT/OFFSET` không có thứ tự xác định, và mỗi batch là một ảnh chụp
//     KHÁC nhau. Dòng rời khỏi tập kết quả giữa hai batch làm cửa sổ trượt ⇒ dòng phía sau bị nhảy
//     qua. Đo được 18 dòng hợp lệ vắng mặt khỏi file khi có ghi đồng thời, không lỗi, không cảnh báo.
//
// Một câu truy vấn = MỘT ảnh chụp nhất quán ⇒ không còn cửa sổ nào để trượt.
//
// ⚠ KHÔNG thêm `ORDER BY`/`LIMIT`/`OFFSET` vào query ở đây. Câu chạy phải đúng là câu caller dựng:
// thứ tự dòng trong file không được đổi, và `ORDER BY` trên bảng gốc kèm JOIN làm mất early-exit
// (đo: 1 batch đọc 4.031.847 dòng). Xem
// skill/skills/debugging-backend-queries/reference/perf-list-endpoints.md §"Export `?_export=1`".
func streamRows(query *gorm.DB, fn func(map[string]interface{}) error) (int64, error) {
	rows, err := query.Session(&gorm.Session{}).Rows()
	if err != nil {
		return 0, fmt.Errorf("open cursor error: %v", err)
	}
	// Đóng con trỏ ở MỌI nhánh thoát, kể cả khi `fn` panic — con trỏ giữ nguyên 1 connection của
	// pool trong suốt thời gian ghi file.
	defer func() { _ = rows.Close() }()

	// Session riêng để scan: ScanRows đi qua ĐÚNG `prepareValues` + `scanIntoMap` mà `Find` vào
	// `[]map[string]interface{}` vẫn dùng, nên KIỂU Go của từng giá trị không đổi so với trước.
	scanner := query.Session(&gorm.Session{NewDB: true})

	var n int64
	for rows.Next() {
		row := map[string]interface{}{}
		if err := scanner.ScanRows(rows, &row); err != nil {
			return n, fmt.Errorf("scan row error: %v", err)
		}
		if err := fn(row); err != nil {
			return n, err
		}
		n++
	}
	// rows.Err() bắt lỗi xảy ra GIỮA CHỪNG khi stream (mất kết nối, query bị kill). Thiếu dòng này
	// thì lỗi đó thành "đọc xong bình thường" và file thiếu dữ liệu một cách im lặng — đúng loại
	// lỗi mà thay đổi này sinh ra để diệt.
	if err := rows.Err(); err != nil {
		return n, fmt.Errorf("cursor error: %v", err)
	}
	return n, nil
}

// SanitizeCell — bản export ra ngoài package của sanitize(), cho các nơi khác cũng ghi ô Excel từ dữ
// liệu người dùng nhập (vd sheet danh mục trong file mẫu import ở pkg/xlsxtemplate). Một chỗ định
// nghĩa duy nhất: chống Excel formula injection mà mỗi nhánh ghi file tự cài một kiểu thì sớm muộn
// có nhánh quên, và quên kiểu này không có triệu chứng gì cho tới lúc ai đó mở file.
func SanitizeCell(val interface{}) interface{} { return sanitize(val) }

func sanitize(val interface{}) interface{} {
	s, ok := val.(string)
	if !ok || s == "" {
		return val
	}
	if strings.ContainsAny(s[:1], "=+-@\t\r") {
		return "'" + s
	}
	return val
}

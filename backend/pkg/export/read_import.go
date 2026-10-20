package export

import (
	"bufio"
	"encoding/csv"
	"mime/multipart"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ReadImportRows đọc file upload thành ma trận chuỗi, nhận cả .xlsx/.xls lẫn .csv.
//
// Đây là cặp đối xứng của Export/Unsanitize trong cùng package: file đi vào importer chính là file
// package này ghi ra, nên hai luật dưới đây phải sống cạnh nhau, không để mỗi module tự chép lại.
//
// Excel BẮT BUỘC đọc bằng RawCellValue: GetRows mặc định trả về chuỗi ĐÃ RENDER theo định dạng ô —
// ô giá 1500000 định dạng "#,##0" đọc ra "1,500,000", ô 0.1 định dạng phần trăm đọc ra "10.00%",
// mã chấm công 0012 định dạng số đọc ra "12". Giá trị đó đi thẳng vào DB là sai dữ liệu/rollback cả
// file. Đây là hệ quả trực tiếp của việc mời user mở file export bằng Excel để sửa.
//
// CSV cần thiết vì export >= CSVThreshold dòng (mặc định 10000) sẽ ra .csv — không đọc được thì
// round-trip đứt đúng lúc danh mục lớn nhất.
func ReadImportRows(fh *multipart.FileHeader) ([][]string, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()

	if strings.HasSuffix(strings.ToLower(fh.Filename), ".csv") {
		r := csv.NewReader(bufio.NewReader(src))
		r.FieldsPerRecord = -1 // dòng ngắn/dài không đều là chuyện thường với file người sửa tay
		rows, err := r.ReadAll()
		if err != nil {
			return nil, err
		}
		// Bóc BOM UTF-8 mà chính export ghi ra, nếu không ô đầu tiên không khớp header nào.
		if len(rows) > 0 && len(rows[0]) > 0 {
			rows[0][0] = strings.TrimPrefix(rows[0][0], "\ufeff")
		}
		return rows, nil
	}

	f, err := excelize.OpenReader(src, ImportOpenOptions())
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.GetRows(f.GetSheetName(0), excelize.Options{RawCellValue: true})
}

// ImportUnzipSizeLimit — trần tổng dung lượng SAU giải nén của file .xlsx upload. File nén ≤ trần
// body (vài MB) nhưng bung ra được tới hàng GB (zip bomb) — mặc định của excelize là 16 GB ⇒ một
// request đủ làm tràn RAM/đĩa máy chủ. 100 MB dư cho file import thật (MaxImportRows dòng).
var ImportUnzipSizeLimit int64 = 100 << 20 // var (không phải const) để test hạ trần

// ImportOpenOptions — options mở file Excel do NGƯỜI DÙNG upload. Mọi chỗ excelize.OpenReader file
// upload PHẢI truyền options này.
func ImportOpenOptions() excelize.Options {
	return excelize.Options{UnzipSizeLimit: ImportUnzipSizeLimit, UnzipXMLSizeLimit: min(16<<20, ImportUnzipSizeLimit)}
}

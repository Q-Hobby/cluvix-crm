package export

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"erpCluvix/pkg/cleanup"
)

// MultiSheetConfig extends Config with multi-sheet options.
type MultiSheetConfig struct {
	Config
	// TotalSheetName is the name of the first sheet (all rows). Default "Tổng hợp".
	TotalSheetName string
	// MinGroupsForSubSheets is the minimum number of distinct groups required
	// before sub-sheets are created. Default 2.
	MinGroupsForSubSheets int
}

// SheetGrouper returns the sheet name a row belongs to.
// Returning an empty string skips the row from sub-sheet grouping
// (the row is still written to the total sheet).
type SheetGrouper func(row map[string]interface{}) string

// ExportMultiSheet exports data to an Excel file with:
//   - Sheet 1 (TotalSheetName): all rows.
//   - One additional sheet per group when distinct groups >= MinGroupsForSubSheets.
//
// Falls back to a single-sheet CSV when total rows >= cfg.CSVThreshold.
func ExportMultiSheet(query *gorm.DB, cfg MultiSheetConfig, grouper SheetGrouper, mapper RowMapper) (string, error) {
	if err := validateConfig(cfg.Config); err != nil {
		return "", err
	}
	if cfg.CSVThreshold == 0 {
		cfg.CSVThreshold = 10000
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 5000
	}
	if cfg.ExportDir == "" {
		cfg.ExportDir = "./storage/export/"
	}
	if cfg.TotalSheetName == "" {
		cfg.TotalSheetName = "Tổng hợp"
	}
	if cfg.MinGroupsForSubSheets == 0 {
		cfg.MinGroupsForSubSheets = 2
	}

	if err := os.MkdirAll(cfg.ExportDir, 0750); err != nil {
		return "", fmt.Errorf("mkdir error: %v", err)
	}

	var total int64
	if cfg.CountRows != nil { // xem Config.CountRows
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
		return "", fmt.Errorf("Không có dữ liệu để export")
	}

	ts := time.Now().Format("2006-01-02-15-04-05")

	// Fallback to single-sheet CSV for large datasets
	if total >= int64(cfg.CSVThreshold) {
		return exportCSV(query, cfg.Config, mapper, ts)
	}

	// Fetch all rows into memory
	var rows []map[string]interface{}
	if err := query.Session(&gorm.Session{}).Find(&rows).Error; err != nil {
		return "", fmt.Errorf("fetch error: %v", err)
	}

	// Map rows + group by sheet name
	type mappedRow = []interface{}
	var allMapped []mappedRow
	groupData := map[string][]mappedRow{}
	var groupOrder []string
	groupSeen := map[string]bool{}

	for _, r := range rows {
		mapped := mapper(r)
		allMapped = append(allMapped, mapped)

		if grouper != nil {
			name := grouper(r)
			if name == "" {
				continue
			}
			if !groupSeen[name] {
				groupSeen[name] = true
				groupOrder = append(groupOrder, name)
			}
			groupData[name] = append(groupData[name], mapped)
		}
	}

	// Build Excel file
	hdr := headerRows(cfg.Config)
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	f.SetSheetName("Sheet1", cfg.TotalSheetName)
	if err := writeSheet(f, cfg.TotalSheetName, hdr, allMapped); err != nil {
		return "", err
	}

	// Add sub-sheets only when there are enough distinct groups
	if len(groupOrder) >= cfg.MinGroupsForSubSheets {
		for _, name := range groupOrder {
			sheet := SafeSheetName(name)
			f.NewSheet(sheet)
			if err := writeSheet(f, sheet, hdr, groupData[name]); err != nil {
				return "", err
			}
		}
	}

	filePath, fileName, err := prepareFile(cfg.Config, ts, "xlsx")
	if err != nil {
		return "", err
	}
	// Cùng khuôn chống-file-cụt với export.go: ghi ra tên tạm ngẫu nhiên rồi mới rename sang tên
	// cuối. `SaveAs` thẳng vào tên cuối có thể để lại file hỏng nếu ghi đứt giữa chừng.
	if err := writeFileAtomically(filePath, func(out *os.File) error {
		return f.Write(out)
	}); err != nil {
		return "", err
	}

	cleanup.ScheduleFileDelete(filePath, 30*time.Minute)
	return DownloadLink(cfg.DownloadURL, fileName), nil
}

// writeSheet writes headers + rows into the named sheet of f.
// writeSheet ghi 1 sheet: các dòng header (nhãn, và dòng field key nếu Config bật FieldKeys) rồi
// tới dữ liệu. Nhận nguyên hdr thay vì chỉ []string nhãn để nhánh multi-sheet không bỏ sót dòng
// field key trong khi nhánh CSV fallback của chính nó lại ghi — lệch nhau là file 2 dạng khác nhau
// tuỳ số dòng dữ liệu.
func writeSheet(f *excelize.File, sheetName string, hdr [][]string, rows [][]interface{}) error {
	for r, line := range hdr {
		for i, h := range line {
			cell, _ := excelize.CoordinatesToCellName(i+1, r+1)
			if err := f.SetCellValue(sheetName, cell, h); err != nil {
				return fmt.Errorf("set header error: %v", err)
			}
		}
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+len(hdr)+1)
			if err := f.SetCellValue(sheetName, cell, sanitize(val)); err != nil {
				return fmt.Errorf("set cell error: %v", err)
			}
		}
	}
	return nil
}

// SafeSheetName truncates name to ≤ 31 chars and removes characters
// that are invalid in Excel sheet names.
func SafeSheetName(name string) string {
	for _, ch := range []string{":", `\`, "/", "?", "*", "[", "]"} {
		name = strings.ReplaceAll(name, ch, " ")
	}
	runes := []rune(name)
	if len(runes) > 31 {
		return string(runes[:31])
	}
	return name
}

package export

// Helper ghi sheet dùng chung — tách từ file export dược (đã xoá) (story-12
// crm-standalone). Nội dung giữ nguyên bản gốc.

import (
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// toInt — coerce số kiểu bất kỳ (map[string]interface{} từ raw SQL row) về int. Dời từ
// unit.go sang đây (story-15 crm-standalone) vì dùng chung ở nhiều file export khác
// (customer.go, order.go, appointment.go, config.go, payment.go, report_revenue*), không
// riêng ExportUnit.
func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	default:
		return 0
	}
}

// loadNameMap preload toàn bộ 1 bảng master data (id -> name) 1 lần cho cả export,
// dùng để resolve tên trong Go thay vì JOIN/subquery lặp lại theo từng dòng.
func loadNameMap(db *gorm.DB, table, idCol, nameCol string) map[int]string {
	var rows []struct {
		ID   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	db.Session(&gorm.Session{NewDB: true}).Table(table).
		Select(idCol + " as id, " + nameCol + " as name").Find(&rows)
	m := make(map[int]string, len(rows))
	for _, r := range rows {
		m[r.ID] = r.Name
	}
	return m
}

// swWriteRow — bản StreamWriter của writeRow. Dùng cho export có thể ra NHIỀU dòng: cách ghi từng ô
// qua *excelize.File giữ cả workbook trong heap (~7,5 KB/dòng), còn StreamWriter giữ heap phẳng
// ~30 MB bất kể số dòng (đo: pkg/export/excelize_mem_test.go).
//
// Lỗi ghi được gắn vào gin.Context thay vì bỏ qua: mất một dòng giữa file là mất dữ liệu im lặng.
func swWriteRow(c *gin.Context, sw *excelize.StreamWriter, sheet string, row int, data []interface{}) {
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := sw.SetRow(cell, data); err != nil {
		_ = c.Error(err)
	}
}

func writeRow(f *excelize.File, sheet string, row int, data []interface{}) {
	for i, val := range data {
		cell, _ := excelize.CoordinatesToCellName(i+1, row)
		f.SetCellValue(sheet, cell, val)
	}
}

package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BindJSON bind JSON body vào struct/map.
// Dùng UseNumber để giữ số dạng json.Number — tránh ảnh hưởng string fields như note, name, content.
// toFloat64 đã handle json.Number sẵn.
func BindJSON(c *gin.Context, v interface{}) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	return dec.Decode(v)
}

// jsonColumnMap: table_name → set of JSON column names
var (
	jsonColumnMap map[string]map[string]bool
	jsonMapOnce   sync.Once
)

// decimalColumnMap: table_name → set of decimal column names
var (
	decimalColumnMap map[string]map[string]bool
	decimalMapOnce   sync.Once
)

// Trạng thái init của decimalColumnMap — phơi ra /health (story-02).
// Lý do phải quan sát được: InitDecimalColumnMap bọc trong sync.Once, hỏng 1 lần là
// hỏng VĨNH VIỄN tới khi restart; khi map rỗng thì mọi cột DECIMAL trả về dưới dạng
// string, code tiêu thụ type-assert `.(decimal.Decimal)` fail âm thầm → tổng tiền
// tính THIẾU mà không ai biết (tiền lệ: QA order-detail-dialog/story-01).
var (
	decimalMapStatusMu sync.RWMutex
	decimalMapStatus   = DecimalColumnMapStatus{Ready: false, Reason: DecimalMapReasonNotInit, Error: "not initialized"}
)

// schemaLogger — test hook. nil ⇒ dùng logger.Default() (nil-safe wrapper quanh logger.L).
// Không exported: chỉ test trong package helpers set được.
var schemaLogger logger.Logger

func schemaLog() logger.Logger {
	if schemaLogger != nil {
		return schemaLogger
	}
	// Gọi Default() mỗi lần (không cache) vì logger.L được gán muộn lúc bootstrap.
	return logger.Default()
}

// DecimalColumnMapStatus mô tả kết quả lần init gần nhất của decimalColumnMap.
type DecimalColumnMapStatus struct {
	Ready   bool   `json:"ready"`            // true khi query INFORMATION_SCHEMA thành công
	Tables  int    `json:"tables"`           // số bảng có cột decimal
	Columns int    `json:"columns"`          // tổng số cột decimal
	Reason  string `json:"reason,omitempty"` // MÃ ngắn có kiểm soát, an toàn để phơi ra /health
	// Error giữ nguyên văn err.Error() cho LOG + test nội bộ. `json:"-"` là BẮT BUỘC:
	// /health là route KHÔNG auth (router.go, ngoài group /api+JWTAuth), mà err của driver MySQL
	// chứa DB user / DB name / host:port ("Access denied for user 'root'@'localhost'",
	// "Unknown database 'x'", "dial tcp 10.x:3306"). Phơi ra là rò thông tin hạ tầng.
	Error string `json:"-"`
}

// Mã lý do phơi ra /health — cố định, không chứa dữ liệu từ error của driver.
const (
	DecimalMapReasonNotInit     = "not_initialized"
	DecimalMapReasonQueryFailed = "schema_query_failed"
	DecimalMapReasonEmptyResult = "schema_query_empty"
)

// DecimalColumnMapStatusSnapshot trả về trạng thái init hiện tại (thread-safe).
// Dùng cho health-check để phát hiện tình trạng "âm thầm trả string".
func DecimalColumnMapStatusSnapshot() DecimalColumnMapStatus {
	decimalMapStatusMu.RLock()
	defer decimalMapStatusMu.RUnlock()
	return decimalMapStatus
}

func setDecimalMapStatus(s DecimalColumnMapStatus) {
	decimalMapStatusMu.Lock()
	decimalMapStatus = s
	decimalMapStatusMu.Unlock()
}

// JSONColumnMapStatus mô tả kết quả lần init gần nhất của jsonColumnMap — cùng khuôn với
// DecimalColumnMapStatus (story-26: `InitJSONColumnMap` từng nuốt lỗi INFORMATION_SCHEMA im
// lặng y hệt bản decimal trước khi story-02 vá; kiểu riêng thay vì tái dùng
// DecimalColumnMapStatus để /health phân biệt rõ đang báo cáo map nào, tránh nhầm lẫn khi cả
// hai đều "not_initialized" cùng lúc).
type JSONColumnMapStatus struct {
	Ready   bool   `json:"ready"`
	Tables  int    `json:"tables"`
	Columns int    `json:"columns"`
	Reason  string `json:"reason,omitempty"`
	// Error KHÔNG expose qua JSON — lý do giống DecimalColumnMapStatus.Error (rò thông tin hạ tầng
	// qua route /health không auth).
	Error string `json:"-"`
}

const (
	JSONMapReasonNotInit     = "not_initialized"
	JSONMapReasonQueryFailed = "schema_query_failed"
	JSONMapReasonEmptyResult = "schema_query_empty"
)

var (
	jsonMapStatusMu sync.RWMutex
	jsonMapStatus   = JSONColumnMapStatus{Ready: false, Reason: JSONMapReasonNotInit, Error: "not initialized"}
)

// JSONColumnMapStatusSnapshot trả về trạng thái init hiện tại (thread-safe).
func JSONColumnMapStatusSnapshot() JSONColumnMapStatus {
	jsonMapStatusMu.RLock()
	defer jsonMapStatusMu.RUnlock()
	return jsonMapStatus
}

func setJSONMapStatus(s JSONColumnMapStatus) {
	jsonMapStatusMu.Lock()
	jsonMapStatus = s
	jsonMapStatusMu.Unlock()
}

// InitJSONColumnMap queries INFORMATION_SCHEMA once to build a map of all JSON columns.
func InitJSONColumnMap(db *gorm.DB) {
	jsonMapOnce.Do(func() {
		jsonColumnMap = make(map[string]map[string]bool)
		var rows []struct {
			Table  string `gorm:"column:TABLE_NAME"`
			Column string `gorm:"column:COLUMN_NAME"`
		}
		if err := db.Raw(`SELECT TABLE_NAME, COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND DATA_TYPE = 'json'`).Scan(&rows).Error; err != nil {
			// KHÔNG panic / KHÔNG fail-fast (cùng quyết định đã chốt cho decimal) — nhưng PHẢI báo
			// ERROR + phơi qua /health: sync.Once khiến trạng thái hỏng tồn tại tới khi restart, mọi
			// cột JSON sẽ trả về dạng string thô một cách âm thầm (không parse thành array/object).
			setJSONMapStatus(JSONColumnMapStatus{Ready: false, Reason: JSONMapReasonQueryFailed, Error: err.Error()})
			schemaLog().Error(fmt.Sprintf(
				"InitJSONColumnMap FAILED: %v — JSON columns sẽ trả về dạng string thô cho tới khi restart app; "+
					"kiểm tra /health field json_column_map", err))
			return
		}
		// Query THÀNH CÔNG nhưng 0 dòng cũng là THẤT BẠI — cùng lý do với InitDecimalColumnMap
		// (DSN thiếu tên DB / thiếu quyền đọc INFORMATION_SCHEMA / sai schema). Schema ERP này luôn
		// có cột JSON (listRevenues, listDoctors, departmentList, …) nên 0 dòng không có ca hợp lệ.
		if len(rows) == 0 {
			const reason = "INFORMATION_SCHEMA trả 0 cột json (DSN thiếu tên DB / thiếu quyền / sai schema)"
			setJSONMapStatus(JSONColumnMapStatus{Ready: false, Reason: JSONMapReasonEmptyResult, Error: reason})
			schemaLog().Error(
				"InitJSONColumnMap FAILED: " + reason +
					" — JSON columns sẽ trả về dạng string thô cho tới khi restart app; " +
					"kiểm tra /health field json_column_map")
			return
		}
		for _, r := range rows {
			if jsonColumnMap[r.Table] == nil {
				jsonColumnMap[r.Table] = make(map[string]bool)
			}
			jsonColumnMap[r.Table][r.Column] = true
		}
		setJSONMapStatus(JSONColumnMapStatus{
			Ready:   true,
			Tables:  len(jsonColumnMap),
			Columns: len(rows),
		})
	})
}

// InitDecimalColumnMap queries INFORMATION_SCHEMA once to build a map of all decimal columns.
func InitDecimalColumnMap(db *gorm.DB) {
	decimalMapOnce.Do(func() {
		decimalColumnMap = make(map[string]map[string]bool)
		var rows []struct {
			Table  string `gorm:"column:TABLE_NAME"`
			Column string `gorm:"column:COLUMN_NAME"`
		}
		if err := db.Raw(`SELECT TABLE_NAME, COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND DATA_TYPE = 'decimal'`).Scan(&rows).Error; err != nil {
			// KHÔNG panic / KHÔNG fail-fast (quyết định đã chốt: lỗi truy vấn schema không được
			// biến thành sự cố sập dịch vụ) — nhưng PHẢI báo ERROR + phơi qua /health, vì
			// sync.Once khiến trạng thái hỏng tồn tại tới khi restart và mọi DECIMAL sẽ trả
			// về dạng string một cách âm thầm.
			setDecimalMapStatus(DecimalColumnMapStatus{Ready: false, Reason: DecimalMapReasonQueryFailed, Error: err.Error()})
			schemaLog().Error(fmt.Sprintf(
				"InitDecimalColumnMap FAILED: %v — decimal columns sẽ trả về dạng string cho tới khi restart app; "+
					"kiểm tra /health field decimal_column_map", err))
			return
		}
		// Query THÀNH CÔNG nhưng 0 dòng cũng là THẤT BẠI, không phải "schema không có cột decimal":
		// xảy ra khi DSN không kèm tên DB (DATABASE() trả NULL), user MySQL không đủ quyền đọc
		// INFORMATION_SCHEMA (bị lọc hết dòng, KHÔNG trả lỗi), hoặc trỏ nhầm schema rỗng.
		// Không xử lý ở đây thì map là "rỗng khác nil" ⇒ ParseDecimalFieldsForTables qua được guard
		// nil rồi return ở len(cols)==0 ⇒ toàn hệ thống trả DECIMAL dạng string mà /health vẫn
		// báo ready=true và KHÔNG có một dòng log nào — đúng thứ story-02 sinh ra để chặn.
		// Schema ERP này luôn có cột decimal (tiền), nên 0 dòng không có ca hợp lệ.
		if len(rows) == 0 {
			const reason = "INFORMATION_SCHEMA trả 0 cột decimal (DSN thiếu tên DB / thiếu quyền / sai schema)"
			setDecimalMapStatus(DecimalColumnMapStatus{Ready: false, Reason: DecimalMapReasonEmptyResult, Error: reason})
			schemaLog().Error(
				"InitDecimalColumnMap FAILED: " + reason +
					" — decimal columns sẽ trả về dạng string cho tới khi restart app; " +
					"kiểm tra /health field decimal_column_map")
			return
		}
		for _, r := range rows {
			if decimalColumnMap[r.Table] == nil {
				decimalColumnMap[r.Table] = make(map[string]bool)
			}
			decimalColumnMap[r.Table][r.Column] = true
		}
		setDecimalMapStatus(DecimalColumnMapStatus{
			Ready:   true,
			Tables:  len(decimalColumnMap),
			Columns: len(rows),
		})
	})
}

// InvalidateSchemaCache invalidates the cached schema maps.
// Call this after running migrations to refresh column metadata.
func InvalidateSchemaCache() {
	jsonMapOnce = sync.Once{}
	decimalMapOnce = sync.Once{}
	jsonColumnMap = nil
	decimalColumnMap = nil
	setDecimalMapStatus(DecimalColumnMapStatus{Ready: false, Reason: DecimalMapReasonNotInit, Error: "not initialized"})
	setJSONMapStatus(JSONColumnMapStatus{Ready: false, Reason: JSONMapReasonNotInit, Error: "not initialized"})
}

// ParseDecimalFieldsForTables converts decimal columns for rows from given tables from []uint8 to decimal.Decimal.
// Optimized version: reduces allocations and improves cache locality.
func ParseDecimalFieldsForTables(rows []map[string]interface{}, tables ...string) {
	if len(rows) == 0 || decimalColumnMap == nil {
		return
	}

	// Build set of decimal columns from all tables
	// Use map for O(1) lookup during row iteration
	cols := make(map[string]bool)
	for _, t := range tables {
		for col := range decimalColumnMap[t] {
			cols[col] = true
		}
	}

	if len(cols) == 0 {
		return
	}

	// Iterate through rows and parse decimal fields
	// Loop through row keys instead of all cols to reduce unnecessary checks
	for _, row := range rows {
		for col, v := range row {
			if v == nil || !cols[col] {
				continue
			}
			row[col] = ToDecimal(v)
		}
	}
}

// ParseJSONFieldsForTables parses JSON columns for rows from given tables.
// Optimized version: reduces allocations and improves performance.
func ParseJSONFieldsForTables(rows []map[string]interface{}, tables ...string) {
	if len(rows) == 0 || jsonColumnMap == nil {
		return
	}

	// Build set of JSON columns from all tables
	cols := make(map[string]bool)
	for _, t := range tables {
		for col := range jsonColumnMap[t] {
			cols[col] = true
		}
	}

	if len(cols) == 0 {
		return
	}

	// Iterate through rows and parse JSON fields
	// Loop through row keys to avoid checking non-existent columns
	for _, row := range rows {
		for col, v := range row {
			if !cols[col] {
				continue
			}

			s, ok := v.(string)
			if !ok || len(s) == 0 {
				continue
			}

			// Convert string to json.RawMessage for proper JSON serialization
			row[col] = json.RawMessage(s)
		}
	}
}

// ParseJSONFieldForTable parses JSON columns for a single row.
func ParseJSONFieldForTable(row map[string]interface{}, table string) {
	if row == nil {
		return
	}
	ParseJSONFieldsForTables([]map[string]interface{}{row}, table)
}

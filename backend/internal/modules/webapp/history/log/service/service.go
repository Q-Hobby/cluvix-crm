package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/pagination"

	"gorm.io/gorm"
)

const (
	ActionCreate = 1
	ActionUpdate = 2
	ActionDelete = 3
)

// allowedLogTables whitelists (table → fkColumn) to prevent dynamic SQL injection.
var allowedLogTables = map[string]string{
	"log_payment":            "payment_id",
	"log_customer":           "customer_id",
	"log_appointment":        "appointment_id",
	"log_order_service_used": "order_service_used_id",
	"log_user":               "user_id",
}

// logEntityTables — bảng log → bảng gốc của entity (dùng cho LogDeleteSnapshot chụp field trước khi
// xoá). Story-40 (E6/D15): KHÔNG còn chụp/ghi `company_id` snapshot vào dòng log — cột `company_id` của
// `log_*` cho phép NULL và màn lịch sử không còn lọc theo company.
var logEntityTables = map[string]string{
	"log_customer":           "customer",
	"log_appointment":        "appointment",
	"log_payment":            "payment",
	"log_order_service_used": "order_service_used",
}

type FieldDiff struct {
	From interface{} `json:"from"`
	To   interface{} `json:"to"`
}

// LogChanges compares oldData vs newData for includeFields, skips if old is nil for skipNullFields,
// and inserts a log row into the specified table.
//
// Parameters:
//   - table: log table name (e.g. "log_customer")
//   - fkColumn: foreign key column name (e.g. "customer_id")
//   - entityID: the entity's ID
//   - oldData: map of old field values (before update)
//   - newData: map of new field values (after update)
//   - includeFields: only these fields are tracked
//   - skipNullFields: skip if oldVal is nil for these fields
//   - userID: who made the change
//   - extra: optional map merged vào row insert (cột bổ sung riêng của bảng log) —
//     đa số caller không cần truyền, existing call site không phải sửa (variadic).
func LogChanges(db *gorm.DB, table, fkColumn string, entityID int, oldData, newData map[string]interface{}, includeFields, skipNullFields []string, userID int, extra ...map[string]interface{}) error {
	return LogChangesWithAction(db, table, fkColumn, entityID, oldData, newData, includeFields, skipNullFields, userID, ActionUpdate, extra...)
}

// LogChangesWithAction giống LogChanges nhưng cho phép gắn action_type tùy chỉnh — dùng cho
// các thao tác về bản chất là update field (chỉ 1-2 field thực sự đổi) nhưng mang ý nghĩa
// nghiệp vụ khác "cập nhật thông thường" (vd soft-delete user chỉ đổi status, nhưng vẫn muốn
// hiển thị action "Xóa" trong lịch sử thay vì "Cập nhật"). Khác với LogDelete — hàm đó snapshot
// TOÀN BỘ field về nil, chỉ phù hợp cho hard-delete thật (record biến mất hẳn khỏi DB).
func LogChangesWithAction(db *gorm.DB, table, fkColumn string, entityID int, oldData, newData map[string]interface{}, includeFields, skipNullFields []string, userID, actionType int, extra ...map[string]interface{}) error {
	diff := buildDiff(oldData, newData, includeFields, skipNullFields)
	if len(diff) == 0 {
		return nil
	}

	jsonBytes, err := json.Marshal(diff)
	if err != nil {
		return nil
	}

	expectedFK, ok := allowedLogTables[table]
	if !ok || fkColumn != expectedFK {
		return fmt.Errorf("change_log: invalid table/fkColumn: %s/%s", table, fkColumn)
	}

	return insertLogRow(db, table, fkColumn, entityID, actionType, jsonBytes, userID, extra...)
}

// insertLogRow ghi 1 dòng log (story-40: không còn tra/ghi company_id của bản ghi gốc).
func insertLogRow(db *gorm.DB, table, fkColumn string, entityID, actionType int, changedFields []byte, userID int, extra ...map[string]interface{}) error {
	row := map[string]interface{}{
		fkColumn:         entityID,
		"action_type":    actionType,
		"changed_fields": string(changedFields),
		"changed_by":     userID,
		"changed_at":     time.Now().Unix(),
	}
	if len(extra) > 0 {
		for k, v := range extra[0] {
			row[k] = v
		}
	}
	if err := db.Session(&gorm.Session{NewDB: true}).Table(table).Create(row).Error; err != nil {
		return fmt.Errorf("change_log insert to %s: %w", table, err)
	}
	return nil
}

func buildDiff(oldData, newData map[string]interface{}, includeFields, skipNullFields []string) map[string]FieldDiff {
	skipSet := make(map[string]bool, len(skipNullFields))
	for _, f := range skipNullFields {
		skipSet[f] = true
	}

	diff := make(map[string]FieldDiff)
	for _, field := range includeFields {
		oldVal := oldData[field]
		newVal := newData[field]

		if skipSet[field] && isNil(oldVal) {
			continue
		}
		if !isMeaningfullyDifferent(oldVal, newVal) {
			continue
		}

		diff[field] = FieldDiff{From: oldVal, To: newVal}
	}
	return diff
}

func isMeaningfullyDifferent(a, b interface{}) bool {
	na := normalize(a)
	nb := normalize(b)

	fa, aIsNum := toFloat(na)
	fb, bIsNum := toFloat(nb)
	if aIsNum && bIsNum {
		return math.Abs(fa-fb) > 1e-9
	}

	return fmt.Sprintf("%v", na) != fmt.Sprintf("%v", nb)
}

func normalize(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		return normalizeStringValue(val)
	case []uint8:
		return normalizeStringValue(string(val))
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	}
	return v
}

// normalizeStringValue parse chuỗi thô từ DB (string hoặc []uint8 của cột JSON) —
// nếu là JSON array thì canonicalize thứ tự phần tử (sort) trước khi trả về, tránh
// buildDiff báo "thay đổi" giả khi cùng 1 tập giá trị chỉ đổi thứ tự (branchId, companyId,
// allowed_ips, allowed_origins, listRevenues — MySQL JSON
// không tự canonicalize thứ tự phần tử trong array, chỉ canonicalize object key).
func normalizeStringValue(raw string) interface{} {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if canonical, ok := canonicalizeJSONArray(s); ok {
		return canonical
	}
	return s
}

// canonicalizeJSONArray parse 1 chuỗi JSON array và trả về dạng chuẩn hoá (sort theo
// string representation của từng phần tử) để so sánh set-equality thay vì order-sensitive.
func canonicalizeJSONArray(s string) (string, bool) {
	if !strings.HasPrefix(s, "[") {
		return "", false
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return "", false
	}
	parts := make([]string, len(arr))
	for i, el := range arr {
		parts[i] = fmt.Sprintf("%v", el)
	}
	sort.Strings(parts)
	return strings.Join(parts, ","), true
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func isNil(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val) == ""
	case []uint8:
		return len(val) == 0 || strings.TrimSpace(string(val)) == ""
	}
	return false
}

// LogDelete ghi 1 dòng log action_type=delete kèm snapshot dữ liệu trước khi xóa
// (dạng {field: {from: oldVal, to: nil}}), dùng cho các entity muốn giữ audit trail
// khi record gốc bị xóa. Bảng log PHẢI có FK ON DELETE SET NULL (log_partners, log_customer)
// thì dòng này mới sống sót sau khi xoá record gốc.
func LogDelete(db *gorm.DB, table, fkColumn string, entityID int, oldData map[string]interface{}, includeFields []string, userID int, extra ...map[string]interface{}) error {
	diff := make(map[string]FieldDiff, len(includeFields))
	for _, field := range includeFields {
		if oldVal := oldData[field]; !isNil(oldVal) {
			diff[field] = FieldDiff{From: oldVal, To: nil}
		}
	}

	jsonBytes, err := json.Marshal(diff)
	if err != nil {
		return nil
	}

	expectedFK, ok := allowedLogTables[table]
	if !ok || fkColumn != expectedFK {
		return fmt.Errorf("change_log: invalid table/fkColumn: %s/%s", table, fkColumn)
	}

	return insertLogRow(db, table, fkColumn, entityID, ActionDelete, jsonBytes, userID, extra...)
}

// LogDeleteSnapshot chụp `fields` của bản ghi gốc rồi ghi 1 dòng log xoá (story-40: bỏ company_id).
// PHẢI gọi TRƯỚC khi xoá bản ghi gốc, trong CÙNG transaction, và SAU mọi lệnh dọn log cũ
// (`DELETE FROM log_x WHERE x_id = ?`) — gọi trước lệnh dọn thì chính dòng log xoá bị dọn mất.
// Bảng log phải có FK ON DELETE SET NULL thì dòng này mới sống sót sau khi xoá bản ghi gốc.
// Bản ghi gốc không tồn tại ⇒ no-op (không có gì để xoá, không ghi log giả).
func LogDeleteSnapshot(tx *gorm.DB, table string, entityID int, fields []string, userID int) error {
	entityTable, ok := logEntityTables[table]
	if !ok {
		return fmt.Errorf("change_log: %s không có trong logEntityTables", table)
	}
	cols := append([]string{}, fields...)
	var snap map[string]interface{}
	err := tx.Session(&gorm.Session{NewDB: true}).Table(entityTable).Select(cols).Where("id = ?", entityID).Take(&snap).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("change_log snapshot %s#%d: %w", entityTable, entityID, err)
	}
	return LogDelete(tx, table, allowedLogTables[table], entityID, snap, fields, userID)
}

// SnapshotRow fetches a single row as map for diff comparison (before/after audit log).
//
// swallowed-sql-errors-sweep story-05: hàm này KHÔNG có `*gin.Context` (dùng ở tầng service, gọi từ
// ≥29 call site xuyên nhiều module) nên KHÔNG áp được pattern `pagination.FailQuery` như handler
// thường — đổi chữ ký trả `error` sẽ lan xuyên toàn bộ 29 call site (nhiều cái ở tầng service khác
// cũng không có `c`), vượt quy mô hợp lý của 1 điểm trong story rải rác. Xếp (b) thay vì (a): lỗi ở
// đây làm 1 dòng LỊCH SỬ THAY ĐỔI thiếu snapshot before/after (LogChanges vẫn ghi log, chỉ diff kém
// chi tiết), KHÔNG làm sai dữ liệu nghiệp vụ hiển thị cho user như các điểm (a) khác. Để lỗi không
// hoàn toàn vô hình, log ra ops (khác im lặng tuyệt đối trước đây).
func SnapshotRow(db *gorm.DB, table string, id interface{}, fields []string) map[string]interface{} {
	var row map[string]interface{}
	tx := db.Table(table).Select(fields).Where("id = ?", id).Take(&row)
	if pagination.IsQueryError(tx.Error) {
		logger.L.Error(fmt.Sprintf("history.SnapshotRow query error table=%s id=%v: %v", table, id, tx.Error))
	}
	return row
}

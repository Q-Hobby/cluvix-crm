// Package userscope — helper hợp nhất phạm vi chi nhánh của `user` lưu trong cột JSON array
// `branchId` (story-62: bỏ cột thương hiệu).
//
// Story-03 (epic onboarding-setup-wizard): trước đây có 3 bản sao gần y hệt của cùng 1 hàm —
// `config/company/handler.appendUserJSONID`, `config/branch/handler.appendUserJSONID` (cả 2 nuốt
// lỗi Scan/Update, nhận `*gorm.DB` không tx-able rõ ràng), và `config/onboarding/service.
// appendUserJSONIDTx` (story-02, tx-able, trả error). Vượt ngưỡng "lặp ≥3 mới tách"
// (backend-hard-rules §Phân định trách nhiệm) ⇒ gộp về đây, giữ hành vi của bản story-02 (đã qua
// integration test onboarding).
package userscope

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Cột JSON array trên bảng `user` mà AppendID chấp nhận. `column` được nối thẳng vào chuỗi
// SELECT/UPDATE (không phải giá trị bind) ⇒ BẮT BUỘC whitelist, không nhận tự do từ caller.
const (
	ColumnBranchIDs = "branchId"
)

var allowedColumns = map[string]struct{}{
	ColumnBranchIDs: {},
}

// AppendID thêm `newID` vào mảng JSON id lưu tại cột `column` (branchId) của bảng
// `user` có `id = userID`, qua `db` (tx-able — truyền `*gorm.DB` thường hoặc một `tx` đang mở).
//
// Idempotent: id đã có trong mảng ⇒ không đổi gì, trả nil. Giá trị cột NULL/""/"null"/"[]" ⇒ coi
// là mảng rỗng, kết quả `[newID]`. Giá trị JSON hỏng (không phải mảng số) ⇒ trả error, KHÔNG âm
// thầm ghi đè thành `[newID]` (sẽ làm mất quyền chi nhánh cũ của user).
func AppendID(db *gorm.DB, userID uint, column string, newID uint) error {
	if _, ok := allowedColumns[column]; !ok {
		return fmt.Errorf("userscope: cột %q không được phép", column)
	}

	var row struct {
		Value *string `gorm:"column:value"`
	}
	if err := db.Table("user").Select(column+" AS value").Where("id = ?", userID).Take(&row).Error; err != nil {
		return err
	}

	ids, err := parseIDs(row.Value)
	if err != nil {
		return fmt.Errorf("user.%s không phải JSON array id: %w", column, err)
	}
	for _, id := range ids {
		if id == newID {
			return nil
		}
	}
	ids = append(ids, newID)
	updated, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return db.Table("user").Where("id = ?", userID).Update(column, string(updated)).Error
}

// parseIDs đọc giá trị thô của cột JSON array id — tách riêng khỏi AppendID để unit test được
// KHÔNG cần DB (cột JSON thật của MySQL không thể chứa `""` — chỉ NULL/`"null"`/`"[]"`/mảng hợp lệ
// — nên ca `""` chỉ tái hiện được ở tầng parse này, không qua round-trip DB thật).
//
// raw == nil (cột SQL NULL) hoặc rỗng/"null" (JSON null) ⇒ mảng rỗng, không lỗi. JSON hỏng (không
// phải mảng số) ⇒ trả error.
func parseIDs(raw *string) ([]uint, error) {
	var ids []uint
	if raw == nil {
		return ids, nil
	}
	v := strings.TrimSpace(*raw)
	if v == "" || v == "null" {
		return ids, nil
	}
	if err := json.Unmarshal([]byte(v), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

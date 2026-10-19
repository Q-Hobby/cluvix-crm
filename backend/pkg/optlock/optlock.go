// Package optlock — khoá lạc quan theo `updated_at` (BIGINT Unix giây) cho các bảng CRM
// (thiết kế D6, bmad/crm-standalone/architecture.md "Optimistic lock: version → updated_at").
//
// Vấn đề: `updated_at` là giây. `WHERE updated_at = ?` + `SET updated_at = now` lọt lost-update
// khi 2 lần ghi cùng giây (giá trị không đổi ⇒ người thứ 2 vẫn khớp). Cách giải: MỌI lần ghi vào
// các bảng trong `tables` đặt `updated_at = GREATEST(now, updated_at + 1)` ⇒ tăng NGHIÊM NGẶT.
//
// Cưỡng chế 1 chỗ: callback GORM `Update` (Register) — chạy cho Save / Updates(map|struct) /
// Update / UpdateColumn(s) (UpdateColumn chỉ bỏ hook model + autoUpdateTime, callback đăng ký
// vẫn chạy). SQL thô (`Exec("UPDATE ...")`) KHÔNG qua callback ⇒ phải nối `RawSet`/`RawSetFor`.
package optlock

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"erpCluvix/pkg/errs"

	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
)

// Column — tên cột khoá.
const Column = "updated_at"

// RawSet — mảnh SET cho SQL thô trên bảng khoá (không alias). Dùng UNIX_TIMESTAMP() để không
// phải chèn thêm tham số vào giữa danh sách `?` sẵn có.
const RawSet = "updated_at = GREATEST(UNIX_TIMESTAMP(), updated_at + 1)"

// RawSetFor — như RawSet nhưng qualify theo alias bảng (vd `UPDATE appointment a ...`).
func RawSetFor(alias string) string {
	return alias + ".updated_at = GREATEST(UNIX_TIMESTAMP(), " + alias + ".updated_at + 1)"
}

// tables — 11 bảng khoá theo updated_at (từng có cột `version`). `print_templates` thêm ở story-57
// (D21) — bảng thật của entity `PrintTemplate` là `print_templates` (số nhiều). (`cron_job` từng nằm
// ở đây, đã gỡ cùng module cron 2026-10-17.)
// `business_profile` thêm ở story-63: bảng 1 dòng, nhiều admin có thể cùng sửa màn "Hồ sơ doanh
// nghiệp" — cùng rủi ro lost-update như các bảng config khác trong tập này.
var tables = map[string]struct{}{
	"appointment":        {},
	"customer":           {},
	"order_service_used": {},
	"order_plan":         {},
	"payment":            {},
	"reminder":           {},
	"service":            {},
	"user":               {},
	"user_profile":       {},
	"print_templates":    {},
	"business_profile":   {},
}

// IsTracked — bảng có thuộc tập khoá theo updated_at không.
func IsTracked(table string) bool {
	_, ok := tables[normalizeTable(table)]
	return ok
}

// Next — giá trị updated_at sau một lần ghi khớp `WHERE updated_at = expected` tại thời điểm now.
func Next(now, expected int64) int64 {
	if expected+1 > now {
		return expected + 1
	}
	return now
}

// BumpExpr — biểu thức gán cho cột updated_at.
func BumpExpr(now int64) clause.Expr {
	return clause.Expr{SQL: "GREATEST(?, updated_at + 1)", Vars: []any{now}}
}

const (
	nowKey      = "optlock:now"
	addedSetKey = "optlock:set_added"
	skipBumpKey = "optlock:skip_bump"
)

// SkipBump — đánh dấu `tx` KHÔNG bump `updated_at` (bỏ qua CẢ bumpCallback lẫn auto-update-time
// mặc định của GORM cho field tên `UpdatedAt`, qua `Omit(Column)`). Dùng cho ghi HỆ THỐNG — KHÔNG
// PHẢI thao tác của người dùng đang giữ 1 bản `updated_at` để so khoá. (Caller gốc là scheduler cron
// ghi `next_run_at`/`last_status`; module cron đã bỏ 2026-10-17 — hiện chưa còn caller sản phẩm.)
//
// Vì sao cần (story-57 review-loop 1): trước khi có cờ này, MỌI UPDATE vào bảng tracked đều bump
// `updated_at`, kể cả ghi nền của scheduler — user đang mở form sửa `cron_job` giữ `updated_at` cũ,
// scheduler chạy job xen giữa (fire/reload) bump `updated_at` lên, user bấm Lưu nhận 409 giả dù
// không ai khác thật sự sửa cấu hình job.
//
// LƯU Ý: chỉ dùng cho ghi mà chương trình chủ động không muốn tính là "thay đổi cấu hình cần khoá"
// — dùng sai chỗ (vd trong 1 update do user submit) sẽ làm khoá lạc quan mất tác dụng ÂM THẦM
// (build + test khác vẫn xanh).
func SkipBump(tx *gorm.DB) *gorm.DB {
	return tx.Set(skipBumpKey, true).Omit(Column)
}

func isSkipBump(db *gorm.DB) bool {
	v, ok := db.Get(skipBumpKey)
	if !ok {
		return false
	}
	skip, _ := v.(bool)
	return skip
}

// Updates — UPDATE có kiểm khoá: `tx` đã chọn bảng (Model/Table) + điều kiện định danh
// (id, ownership…); helper thêm `AND updated_at = expected` và bump. RowsAffected = 0 ⇒
// errs.ErrVersionConflict. Trả updated_at mới (xác định được vì WHERE đã chốt giá trị cũ).
// Vì bump luôn đổi giá trị, RowsAffected ≥ 1 khi khớp kể cả khi các cột khác không đổi.
func Updates(tx *gorm.DB, expected int64, values any) (int64, error) {
	now := time.Now().Unix()
	res := tx.Set(nowKey, now).Where(Column+" = ?", expected).Updates(values)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, errs.ErrVersionConflict
	}
	return Next(now, expected), nil
}

// Save — Save(struct) có kiểm khoá. Dùng Select("*") để GORM KHÔNG rơi vào nhánh
// "update 0 dòng ⇒ Create ON CONFLICT UPDATE ALL" (sẽ ghi đè bản ghi của người khác).
// Sau khi thành công, field UpdatedAt của model được callback đọc lại từ DB.
func Save(tx *gorm.DB, model any, expected int64) error {
	now := time.Now().Unix()
	res := tx.Set(nowKey, now).Select("*").Where(Column+" = ?", expected).Save(model)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrVersionConflict
	}
	return nil
}

// Register — đăng ký callback bump + dọn dẹp. Gọi 1 lần ngay sau gorm.Open (db.Connect, testhelpers).
func Register(db *gorm.DB) error {
	if err := db.Callback().Update().After("gorm:before_update").Before("gorm:update").
		Register("optlock:bump_updated_at", bumpCallback); err != nil {
		return err
	}
	return db.Callback().Update().After("gorm:update").Before("gorm:after_update").
		Register("optlock:after_update", afterCallback)
}

func normalizeTable(t string) string {
	t = strings.TrimSpace(t)
	if i := strings.IndexAny(t, " \t\n"); i >= 0 { // "customer c" / "customer AS c"
		t = t[:i]
	}
	if i := strings.LastIndex(t, "."); i >= 0 { // db.table
		t = t[i+1:]
	}
	return strings.ToLower(strings.Trim(t, "`\""))
}

// stmtTable — tên bảng thật của câu lệnh. Table("x AS a") làm GORM gán Statement.Table = alias
// "a" ⇒ ưu tiên TableExpr (chuỗi gốc) khi có.
func stmtTable(stmt *gorm.Statement) string {
	if stmt.TableExpr != nil && stmt.TableExpr.SQL != "" {
		return stmt.TableExpr.SQL
	}
	return stmt.Table
}

func isUpdatedAtCol(c clause.Column) bool {
	name := c.Name
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.Trim(name, "`") == Column
}

// withBump — thay (hoặc thêm) phép gán updated_at bằng GREATEST(now, updated_at + 1).
func withBump(set clause.Set, now int64) clause.Set {
	out := make(clause.Set, 0, len(set)+1)
	for _, a := range set {
		if isUpdatedAtCol(a.Column) {
			continue
		}
		out = append(out, a)
	}
	return append(out, clause.Assignment{Column: clause.Column{Name: Column}, Value: BumpExpr(now)})
}

func stmtNow(db *gorm.DB) int64 {
	if v, ok := db.Get(nowKey); ok {
		if n, ok := v.(int64); ok {
			return n
		}
	}
	return time.Now().Unix()
}

func bumpCallback(db *gorm.DB) {
	if db.Error != nil {
		return
	}
	if isSkipBump(db) {
		return
	}
	stmt := db.Statement
	if stmt.SQL.Len() != 0 || !IsTracked(stmtTable(stmt)) {
		return
	}
	now := stmtNow(db)
	if c, ok := stmt.Clauses["SET"]; ok { // caller tự dựng clause.Set
		if set, ok := c.Expression.(clause.Set); ok {
			c.Expression = withBump(set, now)
			stmt.Clauses["SET"] = c
		}
		return
	}
	set := callbacks.ConvertToAssignments(stmt)
	if len(set) == 0 {
		return // gorm:update cũng sẽ bỏ qua (không có gì để ghi)
	}
	stmt.AddClause(withBump(set, now))
	db.InstanceSet(addedSetKey, true)
}

func afterCallback(db *gorm.DB) {
	stmt := db.Statement
	if _, added := db.InstanceGet(addedSetKey); added {
		// gorm:update chỉ tự xoá SET khi chính nó dựng; SET do callback dựng phải xoá ở đây,
		// nếu không chained instance dùng lại Statement sẽ ghi lại SET cũ.
		delete(stmt.Clauses, "SET")
		stmt.Settings.Delete(fmt.Sprintf("%p", stmt) + addedSetKey) // khoá InstanceSet = %p(stmt)+key
	}
	if isSkipBump(db) {
		return
	}
	if db.Error != nil || db.DryRun || db.RowsAffected == 0 || !IsTracked(stmtTable(stmt)) {
		return
	}
	refreshUpdatedAt(db)
}

// refreshUpdatedAt — model là 1 struct có khoá chính: đọc lại updated_at thật từ DB (GREATEST
// có thể > now) để response trả FE đúng giá trị khoá, tránh 409 giả ở lần sửa kế tiếp.
func refreshUpdatedAt(db *gorm.DB) {
	stmt := db.Statement
	if stmt.Schema == nil || stmt.ReflectValue.Kind() != reflect.Struct || !stmt.ReflectValue.CanAddr() {
		return
	}
	field := stmt.Schema.LookUpField(Column)
	pk := stmt.Schema.PrioritizedPrimaryField
	if field == nil || pk == nil {
		return
	}
	id, zero := pk.ValueOf(stmt.Context, stmt.ReflectValue)
	if zero {
		return
	}
	var ts int64
	if err := db.Session(&gorm.Session{NewDB: true}).Table(normalizeTable(stmtTable(stmt))).
		Select(Column).Where(pk.DBName+" = ?", id).Limit(1).Scan(&ts).Error; err != nil {
		return
	}
	_ = field.Set(stmt.Context, stmt.ReflectValue, ts)
}

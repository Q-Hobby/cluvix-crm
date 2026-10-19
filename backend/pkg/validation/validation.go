package validation

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"erpCluvix/pkg/phone"

	"gorm.io/gorm"
)

type Errors struct {
	errs []string
}

func New() *Errors {
	return &Errors{}
}

func (e *Errors) Add(msg string) {
	e.errs = append(e.errs, msg)
}

func (e *Errors) HasErrors() bool {
	return len(e.errs) > 0
}

func (e *Errors) Error() string {
	return strings.Join(e.errs, "; ")
}

// ── Required ──

// Required checks that value is not zero/nil/empty.
// Supports: string, *T (pointer), int, float64, slices, etc.
//
//	v.Required(order.BranchId, "Chi nhánh")
//	v.Required(order.CustomerId, "Khách hàng")
//	v.Required(name, "Tên")
func (e *Errors) Required(value interface{}, label string) {
	if isEmpty(value) {
		e.Add(fmt.Sprintf("%s không được để trống", label))
	}
}

// RequiredIf checks required only when condition is true.
//
//	v.RequiredIf(isInsert, order.BranchId, "Chi nhánh")
func (e *Errors) RequiredIf(condition bool, value interface{}, label string) {
	if condition {
		e.Required(value, label)
	}
}

// ── Numeric ──

// Min checks value >= min (int or float64).
//
//	v.Min(order.Total, 0, "Tổng tiền")
func (e *Errors) Min(value interface{}, min float64, label string) {
	if f, ok := toFloat(value); ok && f < min {
		e.Add(fmt.Sprintf("%s phải >= %v", label, min))
	}
}

// Max checks value <= max.
func (e *Errors) Max(value interface{}, max float64, label string) {
	if f, ok := toFloat(value); ok && f > max {
		e.Add(fmt.Sprintf("%s phải <= %v", label, max))
	}
}

// Range checks min <= value <= max.
func (e *Errors) Range(value interface{}, min, max float64, label string) {
	if f, ok := toFloat(value); ok {
		if f < min || f > max {
			e.Add(fmt.Sprintf("%s phải từ %v đến %v", label, min, max))
		}
	}
}

// ── String ──

// MinLen checks string length >= min.
func (e *Errors) MinLen(value string, min int, label string) {
	if value != "" && len([]rune(value)) < min {
		e.Add(fmt.Sprintf("%s phải có ít nhất %d ký tự", label, min))
	}
}

// MaxLen checks string length <= max.
func (e *Errors) MaxLen(value string, max int, label string) {
	if len([]rune(value)) > max {
		e.Add(fmt.Sprintf("%s tối đa %d ký tự", label, max))
	}
}

// ── Enum / In ──

// In checks that value is one of allowed values.
//
//	v.In(status, []int{0, 1, 2}, "Trạng thái")
func (e *Errors) In(value interface{}, allowed interface{}, label string) {
	av := reflect.ValueOf(allowed)
	if av.Kind() != reflect.Slice {
		return
	}
	for i := 0; i < av.Len(); i++ {
		if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", av.Index(i).Interface()) {
			return
		}
	}
	e.Add(fmt.Sprintf("%s không hợp lệ", label))
}

// ── DB checks ──

// Unique checks if a value is unique in the given table/column, excluding the given ID (for updates).
func (e *Errors) Unique(db *gorm.DB, table, column, value string, excludeID uint, label string) {
	if value == "" {
		return
	}
	var count int64
	q := db.Table(table).Where(column+" = ?", value)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	q.Count(&count)
	if count > 0 {
		e.Add(fmt.Sprintf("%s đã tồn tại", label))
	}
}

// Exists checks that a foreign key value exists in the given table.
//
//	v.Exists(db, "customer", order.CustomerId, "Khách hàng")
func (e *Errors) Exists(db *gorm.DB, table string, id interface{}, label string) {
	if isEmpty(id) {
		return // skip if nil/zero — use Required separately
	}
	var count int64
	db.Table(table).Where("id = ?", id).Count(&count)
	if count == 0 {
		e.Add(fmt.Sprintf("%s không tồn tại", label))
	}
}

// ── Pattern ──

// Match validates a string against a regex pattern.
func (e *Errors) Match(value, pattern, message string) {
	if value == "" {
		return
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		e.Add("Regex pattern không hợp lệ")
		return
	}
	if !re.MatchString(value) {
		e.Add(message)
	}
}

// Email validates email format.
func (e *Errors) Email(value string, label string) {
	e.Match(value, `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		fmt.Sprintf("%s không đúng định dạng email", label))
}

// Phone validates Vietnamese mobile phone format.
// Dùng pkg/phone làm single source of truth — chấp nhận 0XXXXXXXXX, +84XXXXXXXXX, 84XXXXXXXXX.
func (e *Errors) Phone(value string, label string) {
	if value == "" {
		return
	}
	if !phone.IsValidMobile(value) {
		e.Add(fmt.Sprintf("%s không đúng định dạng số điện thoại Việt Nam", label))
	}
}

// ── File ──

// FileExtensions validates uploaded file has allowed extension.
func (e *Errors) FileExtensions(file *multipart.FileHeader, allowed []string, label string) {
	if file == nil {
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file.Filename), "."))
	for _, a := range allowed {
		if ext == strings.ToLower(a) {
			return
		}
	}
	e.Add(fmt.Sprintf("%s chỉ chấp nhận định dạng: %s", label, strings.Join(allowed, ", ")))
}

// ── Helpers ──

func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return true
		}
		// *int / *uint với value = 0 là invalid ID → coi như empty
		elem := v.Elem()
		switch elem.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return elem.Int() == 0
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return elem.Uint() == 0
		}
		return false
	case reflect.String:
		return strings.TrimSpace(v.String()) == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	}
	return false
}

func toFloat(value interface{}) (float64, bool) {
	if value == nil {
		return 0, false
	}
	v := reflect.ValueOf(value)
	// Dereference pointer
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	}
	return 0, false
}

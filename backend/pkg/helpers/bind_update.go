package helpers

import (
	"errors"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
)

// preserveAlways — field LUÔN được giữ, kể cả khi caller truyền `preserve` khác.
// Khoá chính không bao giờ được phép đến từ payload: handler update đã chọn bản ghi
// bằng route id + tenant scope ở bước First(), nếu client ghi đè `ID` thì Save() ghi
// sang bản ghi KHÁC và toàn bộ lớp scope phía trên bị vô hiệu.
const preserveAlways = "ID"

// ErrBindUpdateDst — dst không phải con trỏ non-nil tới struct. Lỗi lập trình, không phải lỗi input.
var ErrBindUpdateDst = errors.New("helpers.BindUpdate: dst must be a non-nil pointer to struct")

// BindUpdate bind payload của request vào dst (entity đã load từ DB), nhưng GIỮ NGUYÊN
// giá trị các field hệ thống — mặc định và luôn luôn có `ID`, cộng thêm các field caller liệt kê.
//
// Vì sao phải có: khuôn handler update của repo là
//
//	First(&item, id)  →  ShouldBind*(&item)  →  Save(&item)
//
// `Save()` ghi theo khoá chính NẰM TRONG STRUCT. Client gửi `{"id": 99}` (JSON) hoặc form
// field tên `ID` (multipart) sẽ làm request sửa sang bản ghi 99 — vô hiệu hoá tenant scope
// đã chạy ở bước First(). Lane multipart nguy hiểm hơn lane JSON: gin form-binding KHÔNG
// đọc tag `json`, field thiếu tag `form` thì nó dùng luôn TÊN FIELD làm key, nên `ID` ghi
// đè được ở 100% entity bất kể có dùng base model hay không.
//
// Cách dùng ở handler:
//
//	if err := helpers.BindUpdate(c, &item); err != nil {
//	    response.HandleError(c, err)
//	    return
//	}
//
// Giữ thêm field audit/guard:
//
//	err := helpers.BindUpdate(c, &item, "CompanyID", "CreatedBy")
//
// Content-type tự nhận: `multipart/form-data` → `c.ShouldBind`; còn lại → `c.ShouldBindJSON`.
// Handler nào có hàm bind multipart thủ công riêng (allowlist `c.PostForm`) thì dùng
// BindUpdateWith để giữ nguyên hàm đó.
//
// Field không tồn tại trên struct → bỏ qua im lặng (81 handler dùng entity khác nhau,
// không phải entity nào cũng có `CompanyID`/`CreatedBy`).
func BindUpdate(c *gin.Context, dst any, preserve ...string) error {
	return BindUpdateWith(dst, func() error { return bindByContentType(c, dst) }, preserve...)
}

// BindUpdateWith giống BindUpdate nhưng cho phép truyền hàm bind riêng — dùng cho handler
// có binder multipart thủ công mà
// KHÔNG được thay bằng `c.ShouldBind` (binder thủ công có ngữ nghĩa partial-update: field
// FE không gửi thì giữ nguyên giá trị cũ, còn ShouldBind sẽ zero-out).
func BindUpdateWith(dst any, bind func() error, preserve ...string) error {
	v, err := structValue(dst)
	if err != nil {
		return err
	}

	saved := snapshotFields(v, preserve)
	bindErr := bind()
	// Khôi phục KỂ CẢ khi bind lỗi: bind lỗi vẫn có thể đã ghi một phần vào struct,
	// và một số caller vẫn dùng tiếp `item` sau khi log lỗi.
	restoreFields(v, saved)
	return bindErr
}

// bindByContentType — multipart → ShouldBind (form binding), còn lại → ShouldBindJSON.
// Giữ nguyên kiểu lỗi gin trả về để `response.HandleError(c, err)` hành xử như trước.
func bindByContentType(c *gin.Context, dst any) error {
	if strings.Contains(c.ContentType(), "multipart") {
		return c.ShouldBind(dst)
	}
	return c.ShouldBindJSON(dst)
}

func structValue(dst any) (reflect.Value, error) {
	if dst == nil {
		return reflect.Value{}, ErrBindUpdateDst
	}
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return reflect.Value{}, ErrBindUpdateDst
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, ErrBindUpdateDst
	}
	return v, nil
}

// snapshotFields chụp lại giá trị (bản SAO, không phải alias) của các field cần giữ.
// FieldByName đi xuyên struct nhúng nên field promoted từ base model (`models.BaseModel`,
// `models.SimpleModel`...) cũng lấy được. Field không tồn tại / không set được (unexported,
// hoặc promoted ambiguous cùng độ sâu) → bỏ qua, KHÔNG panic.
func snapshotFields(v reflect.Value, preserve []string) map[string]reflect.Value {
	saved := make(map[string]reflect.Value, len(preserve)+1)
	for _, name := range append([]string{preserveAlways}, preserve...) {
		if _, done := saved[name]; done {
			continue
		}
		f := v.FieldByName(name)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		saved[name] = deepCopy(f)
	}
	return saved
}

// deepCopy trả bản SAO của v theo ĐÚNG kiểu gốc (uint, uint64, int, *uint, []byte...) —
// không ép cứng về uint.
//
// ⚠ Bắt buộc phải deep-copy con trỏ, không được chỉ copy địa chỉ: `encoding/json` ghi
// XUYÊN QUA con trỏ đã khác nil (chỉ cấp phát mới khi field đang nil). Nếu chỉ giữ lại
// địa chỉ thì payload `{"company_id":42}` đã sửa chính ô nhớ mà ta đang "giữ", khôi phục
// xong vẫn ra 42. Ca này đã bị test `d_preserve_multiple` bắt.
//
// Struct được copy nông (gán giá trị) — đủ cho field khoá/audit; không dùng BindUpdate để
// giữ field struct có con trỏ lồng bên trong.
func deepCopy(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		cp := reflect.New(v.Type().Elem())
		cp.Elem().Set(deepCopy(v.Elem()))
		return cp
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		cp := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			cp.Index(i).Set(deepCopy(v.Index(i)))
		}
		return cp
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		cp := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			cp.SetMapIndex(iter.Key(), deepCopy(iter.Value()))
		}
		return cp
	default:
		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		return cp
	}
}

func restoreFields(v reflect.Value, saved map[string]reflect.Value) {
	for name, val := range saved {
		f := v.FieldByName(name)
		if !f.IsValid() || !f.CanSet() {
			continue
		}
		f.Set(val)
	}
}

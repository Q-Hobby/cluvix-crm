package pagination

// story-20 — LỖI SQL Ở ENDPOINT LIST KHÔNG ĐƯỢC IM LẶNG NỮA.
//
// Trước story này `Paginate`/`PaginateWithTotal` KHÔNG đọc `tx.Error` ở bất kỳ đâu: mọi lỗi SQL
// (cột không tồn tại, only_full_group_by, JOIN hỏng, deadlock, connection reset) đều biến thành
// **HTTP 200 + `items: []`**. Đó là lý do `GET /accountant/payment/report-telesale` chạy sai
// (MySQL 1054) rất lâu mà không ai biết — xem story-16 §I.6.2 / story-17.
//
// # Vì sao không đổi chữ ký hàm
//
// `Paginate` trả `Result` (không trả `error`) và handler chuẩn của repo gọi
// `c.JSON(http.StatusOK, result)` **sau khi** hàm trả về. Thêm return value = breaking change cho
// **326 lời gọi / 259 file**. Nên lỗi được ghi thẳng vào `*gin.Context`:
//
//	failList() → response.ServerError(c, err)   // 500 + i18n "server_error" + log (Hard Rule §Response)
//	          → c.Abort()
//	          → NIÊM PHONG c.Writer (sealedWriter)
//
// Niêm phong là mấu chốt: nếu chỉ `AbortWithStatusJSON` thì `c.JSON(200, result)` mà handler gọi
// ngay sau đó vẫn **nối thêm** body thứ hai vào response 500 ⇒ JSON hỏng. `sealedWriter` nuốt mọi
// lần ghi tiếp theo nên response cuối cùng đúng là 500 + `{"message": …}` duy nhất.
//
// # Ranh giới (AC6) — KHÔNG được vá quá tay
//
// `tx.Error == nil` + 0 dòng là **list rỗng hợp lệ** ⇒ vẫn phải 200 + `items: []`.
// `gorm.ErrRecordNotFound` cũng KHÔNG phải lỗi ở ngữ cảnh list (GORM chỉ trả lỗi này cho
// First/Take/Last, nhưng loại trừ tường minh để hành vi không phụ thuộc chi tiết nội bộ GORM).

import (
	"errors"
	"net/http"
	"reflect"

	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// IsQueryError — err có phải lỗi SQL thật ở ngữ cảnh list không (loại trừ gorm.ErrRecordNotFound —
// không phải lỗi). Export để dùng ngoài package `pagination` (story swallowed-sql-errors-sweep, cụm
// map-scan viết tay không đi qua Paginate/PaginateWithTotal).
func IsQueryError(err error) bool {
	return err != nil && !errors.Is(err, gorm.ErrRecordNotFound)
}

// emptySliceNotNull đảm bảo slice nil ra JSON `[]` chứ không phải `null`.
func emptySliceNotNull(dest interface{}) {
	v := reflect.ValueOf(dest)
	if v.Kind() == reflect.Pointer && v.Elem().Kind() == reflect.Slice && v.Elem().IsNil() {
		v.Elem().Set(reflect.MakeSlice(v.Elem().Type(), 0, 0))
	}
}

// FailQuery — bản export của phần "trả 500 + niêm phong writer" của failList, dùng ngoài package
// `pagination` cho handler map-scan viết tay (`.Find`/`.Scan`/`.Take` vào `map`/`[]map`, không qua
// Paginate/PaginateWithTotal). KHÔNG trả Result (không có ý nghĩa ngoài Paginate) — gọi xong, caller
// PHẢI return ngay (không dùng tiếp dest, đã bị niêm phong response).
//
// Nếu handler đã ghi response TRƯỚC điểm gọi này (double-write risk) → không ghi đè, chỉ log qua
// c.Error để lỗi không vô hình — đúng hành vi failList đã làm cho Paginate.
func FailQuery(c *gin.Context, err error) {
	if c == nil {
		return
	}
	if c.Writer != nil && c.Writer.Written() {
		c.Error(err) //nolint:errcheck // gin lưu vào c.Errors cho middleware log
		return
	}
	// ServerErrorReadOnly (không phải ServerError) — lỗi ở đây LUÔN phát sinh từ đường ĐỌC
	// (SELECT/list qua Paginate/PaginateWithTotal hoặc map-scan viết tay), không phải INSERT/UPDATE.
	// response.ServerError có bộ rẽ nhánh 422 + echo mảnh SQL thiết kế riêng cho đường ghi — dùng
	// đường đó ở đây "đúng do may" (story-27), không do cấu trúc.
	response.ServerErrorReadOnly(c, err)
	c.Abort()
	if c.Writer != nil {
		c.Writer = sealedWriter{ResponseWriter: c.Writer, hdr: make(http.Header)}
	}
}

// failList trả 500 cho request hiện tại rồi niêm phong writer.
// Vẫn trả về một `Result` hợp lệ (items = slice rỗng) vì handler còn dùng giá trị trả về sau đó —
// nội dung không đi ra ngoài nữa, nhưng không được để handler panic vì nil.
func failList(c *gin.Context, err error, dest interface{}, page, perPage int) Result {
	emptySliceNotNull(dest)
	FailQuery(c, err)

	return Result{
		Items: dest,
		Meta: Meta{
			TotalCount:  0,
			PageCount:   0,
			CurrentPage: page,
			PerPage:     perPage,
		},
	}
}

// sealedWriter — writer "đã niêm phong": nuốt mọi lần ghi tiếp theo.
//
// Chỉ override phần GHI. `Status()`, `Size()`, `Written()`, `Flush()`, `Hijack()`… vẫn ủy quyền
// cho writer thật để `gin.Logger()` và middleware phía sau đọc đúng status 500 đã gửi.
type sealedWriter struct {
	gin.ResponseWriter
	hdr http.Header
}

// Header trả map RỜI: `c.JSON` sau đó set Content-Type sẽ không đụng header thật (đã gửi rồi).
func (w sealedWriter) Header() http.Header               { return w.hdr }
func (w sealedWriter) Write(b []byte) (int, error)       { return len(b), nil }
func (w sealedWriter) WriteString(s string) (int, error) { return len(s), nil }
func (w sealedWriter) WriteHeader(int)                   {}
func (w sealedWriter) WriteHeaderNow()                   {}

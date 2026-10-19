// Typed Generic wrappers — Phase 1.2 of refactor-roadmap.
//
// Mục đích: cho phép caller pin element type qua type parameter, đổi
// `interface{}` boundary thành compile-time type check. Backward-compat:
// Paginate / PaginateWithTotal cũ vẫn dùng được cho *[]map[string]interface{}
// (auto JSON+decimal parse) hoặc legacy *[]SomeStruct mixed code.
//
// Khi nào dùng typed:
//   - Endpoint trả về list of struct (không phải dynamic map).
//   - Muốn IDE auto-complete + lint catch mismatched type.
//
// Khi nào dùng untyped (Paginate cũ):
//   - dest là *[]map[string]interface{} cần auto-parse JSON/decimal columns.
//   - Multi-table join với select column từ nhiều bảng (map dễ hơn).

package pagination

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaginateTyped is the generic-typed entry point. Equivalent to Paginate but
// constrains dest to *[]T at compile time.
//
// Example:
//
//	var items []emrModels.EmrMedicalRecordWithRel
//	result := pagination.PaginateTyped(c, q, &items)
func PaginateTyped[T any](c *gin.Context, query *gorm.DB, dest *[]T) Result {
	return Paginate(c, query, dest)
}

// PaginateWithTotalTyped — generic version of PaginateWithTotal.
func PaginateWithTotalTyped[T any](c *gin.Context, query *gorm.DB, dest *[]T, total int64) Result {
	return PaginateWithTotal(c, query, dest, total)
}

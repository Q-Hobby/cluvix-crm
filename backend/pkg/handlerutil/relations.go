// Package handlerutil — shared helpers cho list/detail handlers map[string]any.
//
// Trước: 16 module duplicate collectIDs/attachRelation/attachRelationArray
// (Surgical Changes lúc migrate sang layered layout 2026-05-16). Sau extract về
// đây để tránh drift — một bản canonical, callsite gọi trực tiếp.
//
// Signature 7 string + ids []any: caller giữ trách nhiệm collect ids trước
// (qua CollectIDs) — flexibility cho code path đã có sẵn ids từ chỗ khác.
package handlerutil

import (
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CollectIDs returns unique non-zero IDs from `items[].key` as []interface{}
// (usable directly as a GORM `WHERE col IN ?` arg).
func CollectIDs(items []map[string]interface{}, key string) []interface{} {
	seen := make(map[int64]bool)
	var ids []interface{}
	for _, item := range items {
		id := helpers.ToInt64(item[key])
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// attachRelationWithScope — lõi dùng chung cho AttachRelation/
// AttachRelationScoped. Nhận thẳng scope function (story-33: gộp các bản fork
// attachRelation* khác nhau CHỈ ở khoản chọn ScopedDB* nào — xem AttachRelationScoped).
func attachRelationWithScope(scope tenant.ScopeFunc, c *gin.Context, db *gorm.DB, items []map[string]interface{}, fk, targetKey, table, pk, selectFields string, ids []interface{}) {
	if len(ids) == 0 {
		return
	}
	var rows []map[string]interface{}
	scope(c, db, table).Table(table).
		Select(selectFields).Where(pk+" IN ?", ids).Find(&rows)
	helpers.ParseJSONFields(rows)
	// Rows nested KHÔNG đi qua pagination.Paginate (Paginate chỉ parse rows top-level) → phải tự parse
	// decimal, nếu không cột DECIMAL của bảng liên quan ra JSON string. ToDecimal idempotent nên call site
	// đã tự vá tay vẫn đúng.
	helpers.ParseDecimalFieldsForTables(rows, table)
	// Cột ảnh (avatar/image) lưu KEY kho public ⇒ ghép URL công khai cho object lồng.
	storage.ResolveTableImages(rows, table)
	m := make(map[int64]map[string]interface{})
	for _, r := range rows {
		m[helpers.ToInt64(r[pk])] = r
	}
	for _, item := range items {
		item[targetKey] = m[helpers.ToInt64(item[fk])]
	}
}

// AttachRelation loads rows from `table` where `pk IN ids`, then for each
// item assigns items[i][targetKey] = matching row (by item[fk] == row[pk]).
// Scope: tenant.CrossBranchLookupScope — FK lookup CỐ Ý không filter branch (record có thể thuộc branch
// khác). Story-41 (E6/D15): thay helper company-only cũ (vế company vô nghĩa với 1 thương hiệu/DB).
// Tự gọi helpers.ParseJSONFields để decode JSON columns đã in từ DB.
func AttachRelation(c *gin.Context, db *gorm.DB, items []map[string]interface{}, fk, targetKey, table, pk, selectFields string, ids []interface{}) {
	attachRelationWithScope(tenant.CrossBranchLookupScope, c, db, items, fk, targetKey, table, pk, selectFields, ids)
}

// AttachRelationScoped is AttachRelation với scope function do caller chọn thay vì mặc định
// CrossBranchLookupScope — story-33: gộp các bản fork attachRelation* trong 9 vị trí khác nhau
// (print/reception_payment, loyalty/coupon_used, loyalty/broker_commission) mà lý do KHÔNG dùng
// thẳng AttachRelation là cần ScopedDBNullBranch (hoặc chọn theo tên bảng) thay vì lookup không
// lọc branch. (Story-41: xoá biến thể company-only 0 caller.)
func AttachRelationScoped(scope tenant.ScopeFunc, c *gin.Context, db *gorm.DB, items []map[string]interface{}, fk, targetKey, table, pk, selectFields string, ids []interface{}) {
	attachRelationWithScope(scope, c, db, items, fk, targetKey, table, pk, selectFields, ids)
}

// AttachRelationAuto — "slim API" trên AttachRelation: caller chỉ truyền fkField + table +
// targetKey + selectFields, tự động CollectIDs(items, fkField) + tự resolve cột PK (user_profile
// dùng `user_id`, còn lại `id`). No-op nếu items rỗng.
// story-33: gộp 3 bản copy-paste giống hệt nhau ở customer-care/{care,review,complaints}/handler/
// helpers.go (mỗi bản chỉ khác tên Handler struct) — bản copy vốn đã gọi AttachRelation (không có
// lỗi thiếu decimal/JSON parse như 6 fork khác trong story-33), nhưng vẫn là 3x trùng code.
func AttachRelationAuto(c *gin.Context, db *gorm.DB, items []map[string]interface{}, fkField, table, targetKey, selectFields string) {
	if len(items) == 0 {
		return
	}
	ids := CollectIDs(items, fkField)
	pk := "id"
	if table == "user_profile" {
		pk = "user_id"
	}
	AttachRelation(c, db, items, fkField, targetKey, table, pk, selectFields, ids)
}

// AttachRelationReverse — biến thể "1 chọn 1 mới nhất" của AttachRelationArray: tìm rows nơi
// `fk IN ids`, sắp `ORDER BY id DESC`, rồi cho mỗi item gán items[i][targetKey] = row ĐẦU TIÊN
// khớp item[pk] == row[fk] (bản ghi mới nhất, không phải mảng). Scope: ScopedDBNullBranch (khớp cả
// 3 bản fork gốc — treatment_service, order_plan; bản customer_detail không còn call
// site, đã xoá).
// story-33: gộp 3 fork `attachRelationReverse` giống hệt nhau ở 3 module.
func AttachRelationReverse(c *gin.Context, db *gorm.DB, items []map[string]interface{}, pk, targetKey, table, fk, selectFields string, ids []interface{}) {
	if len(ids) == 0 {
		return
	}
	var rows []map[string]interface{}
	tenant.ScopedDBNullBranch(c, db, table).Table(table).
		Select(selectFields).Where(fk+" IN ?", ids).Order("id DESC").Find(&rows)
	helpers.ParseJSONFields(rows)
	helpers.ParseDecimalFieldsForTables(rows, table)
	// Cột ảnh (avatar/image) lưu KEY kho public ⇒ ghép URL công khai cho object lồng.
	storage.ResolveTableImages(rows, table)
	m := make(map[int64]map[string]interface{})
	for _, r := range rows {
		key := helpers.ToInt64(r[fk])
		if _, exists := m[key]; !exists {
			m[key] = r // keep first (latest do ORDER BY id DESC)
		}
	}
	for _, item := range items {
		item[targetKey] = m[helpers.ToInt64(item[pk])]
	}
}

// attachRelationArrayWithScope — lõi dùng chung cho AttachRelationArray/AttachRelationArrayScoped.
func attachRelationArrayWithScope(scope tenant.ScopeFunc, c *gin.Context, db *gorm.DB, items []map[string]interface{}, pk, targetKey, table, fk, selectFields string, ids []interface{}) {
	if len(ids) == 0 {
		return
	}
	var rows []map[string]interface{}
	scope(c, db, table).Table(table).
		Select(selectFields).Where(fk+" IN ?", ids).Find(&rows)
	// Xem chú thích ở AttachRelation — rows nested không được Paginate phủ.
	// story-26 AC3: đã rà cả 3 call site hiện có (treatment_course_assistant/technical) — 0 cột
	// JSON nào bị bỏ lỡ hôm nay (fields chọn đều decimal/int/timestamp), nhưng thiếu gọi
	// ParseJSONFields là THIẾU SÓT so với 2 hàm anh em (AttachRelation),
	// không phải cố ý — thêm cho nhất quán, chặn bẫy im lặng nếu sau này ai thêm cột JSON vào 1
	// trong các bảng qua hàm này.
	helpers.ParseJSONFields(rows)
	helpers.ParseDecimalFieldsForTables(rows, table)
	// Cột ảnh (avatar/image) lưu KEY kho public ⇒ ghép URL công khai cho object lồng.
	storage.ResolveTableImages(rows, table)
	m := make(map[int64][]map[string]interface{})
	for _, r := range rows {
		key := helpers.ToInt64(r[fk])
		m[key] = append(m[key], r)
	}
	for _, item := range items {
		if arr, ok := m[helpers.ToInt64(item[pk])]; ok {
			item[targetKey] = arr
		} else {
			item[targetKey] = []map[string]interface{}{}
		}
	}
}

// AttachRelationArray is the 1-to-N variant of AttachRelation. Loads rows
// where `fk IN ids`, then for each item assigns items[i][targetKey] = []rows
// matching item[pk]. Items không có match nhận `[]map[string]interface{}{}`
// thay vì nil (FE tiêu thụ array consistently).
// Scope: tenant.CrossBranchLookupScope — FK lookup CỐ Ý không filter branch (story-41).
func AttachRelationArray(c *gin.Context, db *gorm.DB, items []map[string]interface{}, pk, targetKey, table, fk, selectFields string, ids []interface{}) {
	attachRelationArrayWithScope(tenant.CrossBranchLookupScope, c, db, items, pk, targetKey, table, fk, selectFields, ids)
}

// AttachRelationArrayScoped is AttachRelationArray với scope function do caller chọn — story-33:
// gộp fork brokerAttachRelationArray (loyalty/broker) cần ScopedDBNullBranch thay vì company-only.
func AttachRelationArrayScoped(scope tenant.ScopeFunc, c *gin.Context, db *gorm.DB, items []map[string]interface{}, pk, targetKey, table, fk, selectFields string, ids []interface{}) {
	attachRelationArrayWithScope(scope, c, db, items, pk, targetKey, table, fk, selectFields, ids)
}

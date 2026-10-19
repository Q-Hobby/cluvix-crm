package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	history "erpCluvix/internal/modules/webapp/history/log/service"
	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) ListLog(c *gin.Context) {
	fieldName := c.Query("_fieldName")
	kw := c.Query("_keyword")

	// S-8: ẩn SĐT (option 14) áp cả cho lịch sử — giá trị cũ/mới của field SĐT trong changed_fields
	// và không cho tìm theo SĐT trong snapshot dòng xoá khi user không được xem số thật.
	showPhone := phoneutil.CanSeePhone(c, h.DB)

	var q *gorm.DB

	switch fieldName {
	case "customer":
		q = h.logCustomerQuery(c, kw, showPhone)
	case "appointment":
		q = h.logAppointmentQuery(c, kw)
	case "payment":
		q = h.logPaymentQuery(c, kw)
	case "order-service-used":
		q = h.logOrderServiceUsedQuery(c, kw)
	case "user":
		q = h.logUserQuery(c, kw)
	default:
		response.Validate(c, i18n.T(c, "history_not_found"))
		return
	}

	var items []map[string]interface{}
	result := pagination.Paginate(c, q, &items)
	if c.Writer.Written() {
		return // Paginate đã trả lỗi query (gồm lỗi đọc phạm vi telesale — fail-closed)
	}

	for i, item := range items {
		switch fieldName {
		case "user":
			items[i]["user"] = gin.H{
				"id":       item["user_id"],
				"username": item["user_username"],
				"fullname": item["user_fullname"],
				"nickname": item["user_nickname"],
			}
			delete(items[i], "user_username")
			delete(items[i], "user_fullname")
			delete(items[i], "user_nickname")
			parseChangedFields(items[i])
			maskChangedPhones(items[i], showPhone)
			continue
		}

		// Nest customer fields into object
		items[i]["customer"] = gin.H{
			"id":       item["customer_id"],
			"fullname": item["customer_fullname"],
			"nickname": item["customer_nickname"],
		}
		delete(items[i], "customer_fullname")
		delete(items[i], "customer_nickname")
		parseChangedFields(items[i])
		maskChangedPhones(items[i], showPhone)
	}

	c.JSON(http.StatusOK, result)
}

// parseChangedFields parse cột changed_fields (lưu dạng JSON string trong DB) thành object.
func parseChangedFields(item map[string]interface{}) {
	cf, ok := item["changed_fields"]
	if !ok || cf == nil {
		return
	}
	str, ok := cf.(string)
	if !ok {
		return
	}
	var parsed interface{}
	if json.Unmarshal([]byte(str), &parsed) == nil {
		item["changed_fields"] = parsed
	}
}

// changedPhoneFields — field SĐT có thể nằm trong changed_fields (log_customer theo dõi `phone`; các
// field còn lại phòng khi được thêm vào danh sách theo dõi sau này).
var changedPhoneFields = []string{"phone", "other_phone", "zalo", "phone_zalo"}

// maskChangedPhones che giá trị from/to của các field SĐT trong changed_fields (đã parse) theo đúng
// phoneutil.MaskPhone — cùng cơ chế option 14 của list/detail khách. show=true ⇒ giữ nguyên.
func maskChangedPhones(item map[string]interface{}, show bool) {
	if show {
		return
	}
	cf, ok := item["changed_fields"].(map[string]interface{})
	if !ok {
		return
	}
	for _, f := range changedPhoneFields {
		entry, ok := cf[f].(map[string]interface{})
		if !ok {
			continue
		}
		for _, k := range []string{"from", "to"} {
			if v, ok := entry[k].(string); ok {
				entry[k] = phoneutil.MaskPhone(v)
			}
		}
	}
}

// snapStr / snapUint — đọc snapshot `changed_fields.<field>.from` của dòng log XOÁ (action_type=3,
// ghi bởi history.LogDeleteSnapshot). Sau khi xoá, fk trên dòng log = NULL (FK ON DELETE SET NULL)
// nên JOIN bảng gốc rỗng: id/code/customer_id phải lấy từ snapshot. Dòng khác action → NULL.
// alias/field là hằng trong code, KHÔNG nhận input người dùng.
func snapStr(alias, field string) string {
	return fmt.Sprintf("IF(%s.action_type = %d, JSON_UNQUOTE(JSON_EXTRACT(%s.changed_fields, '$.%s.from')), NULL)",
		alias, history.ActionDelete, alias, field)
}

func snapUint(alias, field string) string {
	return fmt.Sprintf("IF(%s.action_type = %d, JSON_VALUE(%s.changed_fields, '$.%s.from' RETURNING UNSIGNED), NULL)",
		alias, history.ActionDelete, alias, field)
}

// Story-40 (E6/D15 — 1 thương hiệu/DB): `leftJoinScoped` (vế company trong ON của LEFT JOIN) đã bỏ —
// LEFT JOIN trần. Bảng log (`log_*`) không thuộc `branchTables` nên `ScopedDBNullBranch` bên dưới không
// thêm vế nào (giữ y hệt trước: không có phạm vi chi nhánh trên màn lịch sử).

// logCustomerQuery: customer_id trực tiếp.
//
// Dòng log XOÁ khách có customer_id = NULL ⇒ tên/mã lấy từ snapshot.
// S-10: phạm vi telesale theo lc.customer_id (dòng xoá — customer_id NULL — bị loại khi bị giới hạn).
func (h *Handler) logCustomerQuery(c *gin.Context, kw string, showPhone bool) *gorm.DB {
	q := h.DB.Table("log_customer lc").
		Select("lc.id, lc.customer_id, lc.action_type, lc.changed_fields, lc.changed_by, lc.changed_at, " +
			"COALESCE(c.fullname, " + snapStr("lc", "fullname") + ") AS customer_fullname, " +
			"COALESCE(c.nickname, " + snapStr("lc", "nickname") + ") AS customer_nickname").
		Order("lc.id DESC")
	q = q.Joins("LEFT JOIN customer c ON c.id = lc.customer_id")
	q = tenant.ScopedDBNullBranch(c, q, "log_customer", "lc")
	q = scope.WhereCustomerID(c, h.DB, q, "lc.customer_id")
	if kw != "" {
		like := "%" + strings.TrimSpace(kw) + "%"
		if showPhone {
			q = q.Where("(c.fullname LIKE ? OR c.phone LIKE ? OR c.code LIKE ? OR "+
				snapStr("lc", "fullname")+" LIKE ? OR "+snapStr("lc", "phone")+" LIKE ? OR "+snapStr("lc", "code")+" LIKE ?)",
				like, like, like, like, like, like)
		} else {
			// Bị ẩn SĐT: vẫn tìm theo SĐT hiện tại của khách (như màn danh sách khách) nhưng KHÔNG theo
			// SĐT trong snapshot dòng xoá — số đó không còn hiển thị ở đâu, dò LIKE là cách đọc ngược nó.
			q = q.Where("(c.fullname LIKE ? OR c.phone LIKE ? OR c.code LIKE ? OR "+
				snapStr("lc", "fullname")+" LIKE ? OR "+snapStr("lc", "code")+" LIKE ?)",
				like, like, like, like, like)
		}
	}
	return q
}

// entityLogQuery — khung chung cho log của entity con của khách (lịch hẹn/thanh toán/buổi điều
// trị/dòng dịch vụ): khách lấy qua bản ghi gốc, dòng XOÁ thì qua snapshot customer_id (khách VẪN
// còn, chỉ bản ghi gốc mất). Cột fk trả về = COALESCE(fk, snapshot id) để cột ID vẫn hiện sau khi
// xoá. `customer_id` trả về là c.id (NULL khi khách không còn) chứ KHÔNG phải khoá JOIN — trả khoá
// JOIN thì FE dựng link tới khách không tồn tại.
func (h *Handler) entityLogQuery(c *gin.Context, kw, logTable, alias, fkCol, entityTable, entityAlias, extraSelect string) *gorm.DB {
	customerID := "COALESCE(" + entityAlias + ".customer_id, " + snapUint(alias, "customer_id") + ")"
	sel := alias + ".id, COALESCE(" + alias + "." + fkCol + ", " + snapUint(alias, "id") + ") AS " + fkCol + ", " +
		alias + ".action_type, " + alias + ".changed_fields, " + alias + ".changed_by, " + alias + ".changed_at, " +
		"c.id AS customer_id, c.fullname AS customer_fullname, c.nickname AS customer_nickname"
	if extraSelect != "" {
		sel += ", " + extraSelect
	}
	q := h.DB.Table(logTable + " " + alias).
		Select(sel).
		Order(alias + ".id DESC")
	q = q.Joins("LEFT JOIN " + entityTable + " " + entityAlias + " ON " + entityAlias + ".id = " + alias + "." + fkCol)
	q = q.Joins("LEFT JOIN customer c ON c.id = " + customerID)
	q = tenant.ScopedDBNullBranch(c, q, logTable, alias)
	// S-10: phạm vi telesale theo khách của bản ghi gốc (hoặc snapshot khi bản ghi đã xoá).
	q = scope.WhereCustomerID(c, h.DB, q, customerID)
	if kw != "" {
		like := "%" + strings.TrimSpace(kw) + "%"
		q = q.Where("(c.fullname LIKE ? OR c.phone LIKE ? OR c.code LIKE ? OR "+
			entityAlias+".code LIKE ? OR CAST("+entityAlias+".id AS CHAR) LIKE ? OR "+
			snapStr(alias, "code")+" LIKE ? OR "+snapStr(alias, "id")+" LIKE ?)",
			like, like, like, like, like, like, like)
	}
	return q
}

func (h *Handler) logAppointmentQuery(c *gin.Context, kw string) *gorm.DB {
	return h.entityLogQuery(c, kw, "log_appointment", "la", "appointment_id", "appointment", "a", "")
}

func (h *Handler) logPaymentQuery(c *gin.Context, kw string) *gorm.DB {
	return h.entityLogQuery(c, kw, "log_payment", "lp", "payment_id", "payment", "p", "")
}

func (h *Handler) logOrderServiceUsedQuery(c *gin.Context, kw string) *gorm.DB {
	return h.entityLogQuery(c, kw, "log_order_service_used", "losu", "order_service_used_id", "order_service_used", "osu",
		"losu.reason, osu.order_plan_id, COALESCE(osu.code, "+snapStr("losu", "code")+") AS order_service_used_code")
}

// logUserQuery: log_user join user + user_profile.
//
// Story-40 (E6/D15): bỏ lọc JSON_CONTAINS(u.companyId, …) — cùng hướng story-58 đã bỏ ở
// hr/user BuildUserQuery. Không có vế chi nhánh (trước cũng không có).
func (h *Handler) logUserQuery(_ *gin.Context, kw string) *gorm.DB {
	q := h.DB.Table("log_user lu").
		Select(`lu.*, u.username AS user_username, p.fullname AS user_fullname, p.nickname AS user_nickname`).
		Joins("LEFT JOIN user u ON u.id = lu.user_id").
		Joins("LEFT JOIN user_profile p ON p.user_id = u.id").
		Order("lu.id DESC")

	// user_id không bao giờ NULL qua flow hiện tại (DeleteUser chỉ soft-delete) — không
	// thêm "OR lu.user_id IS NULL" phòng xa vì sẽ mở lại lỗ hổng leak cross-tenant giống
	// log_partners nếu sau này có ai thêm hard-delete user mà không đi kèm fix tương ứng.

	if kw != "" {
		like := "%" + strings.TrimSpace(kw) + "%"
		// Bọc ngoặc: vế OR phải đứng thành 1 khối nếu sau này có thêm điều kiện ghép vào.
		q = q.Where("(u.username LIKE ? OR p.fullname LIKE ? OR p.nickname LIKE ?)", like, like, like)
	}
	return q
}

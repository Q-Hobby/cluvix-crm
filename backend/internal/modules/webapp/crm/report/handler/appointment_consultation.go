package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/tenant"
)

// revenueAppointmentTypeIDs đọc danh sách id loại hẹn "tính doanh thu"
// (`appointment_type.isTypeRevenue = 1`) để nhét vào `IN (...)` dưới dạng HẰNG SỐ.
//
// ⚠ Đây là bản vá hiệu năng, không phải dọn dẹp phong cách. Viết dạng subquery
// (`appointment_type_id IN (SELECT id FROM appointment_type WHERE isTypeRevenue = 1)`)
// bị MySQL 9.7 chuyển thành semi-join ⇒ INNER JOIN lấy `appointment_type` làm bảng
// DẪN rồi `Index lookup on appointment using fk-appointment_appointment_type_id`.
// Hệ quả: bộ lọc NGÀY không dùng được `idx_appointment_date_scope` nữa, mọi lần mở
// màn đều đọc TOÀN BỘ lịch hẹn tư vấn rồi mới sort/filter — khối lượng không phụ
// thuộc khoảng ngày người dùng chọn. Đo trên 872.284 lịch hẹn / 57.384 lịch tư vấn
// (2026-09-08): SELECT trang 1 lọc tháng hiện tại 16.528 dòng InnoDB / 0,119–0,366s
// → 254 dòng / 0,001–0,002s; COUNT không lọc 2.616.852 dòng / 0,50–1,36s →
// 872.284 dòng / 0,21–0,38s. Đã đối chiếu 8 bộ lọc: số dòng + 15 id trang 1 KHỚP
// tuyệt đối giữa hai cách viết.
//
// KHÔNG tenant-scope bảng `appointment_type`: bảng này không có cột `company_id`
// (giữ nguyên ngữ nghĩa subquery cũ, để bản vá không đổi kết quả).
func revenueAppointmentTypeIDs(db *gorm.DB) ([]int64, error) {
	var ids []int64
	err := db.Table("appointment_type").Where("isTypeRevenue = 1").Pluck("id", &ids).Error
	return ids, err
}

// buildAppointmentConsultationQuery builds the base query for lịch hẹn tư vấn
// (appointment_type.isTypeRevenue = 1).
//
// `LEFT JOIN customer` CHỈ gắn khi `_export=1`: đường list không đọc cột nào của
// `customer` (thông tin khách lấy ở query thứ 2 theo `customer_id`), mà MySQL KHÔNG
// loại-bỏ-left-join nên join vô điều kiện vẫn tra PK `customer` cho từng dòng —
// kể cả trong `COUNT(*)` derived của `pagination.Paginate`. `export.ExportAppointment`
// thì BẮT BUỘC cần join này (nó select `customer.code/fullname/phone` và join
// `location_province ON prov.id = customer.province_id`); ép join tại ĐÂY — một điểm
// quyết định duy nhất — chứ không để hàm export tự join (sẽ thành "Not unique
// table/alias: 'customer'" nếu sau này list cũng cần join). Cùng mẫu
// `needsCustomerJoin()` trong crm/appointment/handler/list_search.go.
//
// ⚠ Thêm filter/sort mới đụng cột `customer.*` ⇒ phải mở rộng điều kiện gắn join,
// không thì `Unknown column`.
func buildAppointmentConsultationQuery(c *gin.Context, db *gorm.DB) (*gorm.DB, error) {
	typeIDs, err := revenueAppointmentTypeIDs(db)
	if err != nil {
		return nil, err
	}

	q := tenant.ScopedDBNullBranch(c, db, "appointment").
		Table("appointment")

	if c.Query("_export") == "1" {
		q = q.Joins("LEFT JOIN customer ON customer.id = appointment.customer_id")
	}

	// typeIDs rỗng ⇒ GORM sinh `IN (NULL)` ⇒ 0 dòng, đúng bằng ngữ nghĩa subquery rỗng.
	q = q.Where("appointment.appointment_type_id IN ?", typeIDs)

	q = query.ApplyFilter(c, q, "appointment_date", "appointment.appointment_date")
	q = query.ApplyFilter(c, q, "branch_id", "appointment.branch_id")
	q = query.ApplyFilter(c, q, "status", "appointment.status")

	// S-10: phạm vi telesale — danh sách liệt kê từng khách ⇒ chỉ khách mình phụ trách.
	if q, err = scope.ApplyCustomerID(c, db, q, "appointment.customer_id"); err != nil {
		return nil, err
	}

	q = query.ApplySort(c, q, "appointment.appointment_date DESC",
		"appointment.appointment_date", "appointment.id", "appointment.status")

	return q.Select(`appointment.id, appointment.code, appointment.customer_id,
		appointment.appointment_type_id, appointment.branch_id,
		appointment.appointment_date, appointment.checkin_date, appointment.checkout_date,
		appointment.status, appointment.is_accepted, appointment.type_service_accepted_id,
		appointment.appointment_reason_id, appointment.noteAppointment,
		appointment.created_by, appointment.created_at`), nil
}

// AppointmentConsultationReport GET /crm/report/appointment-consultation
// List lịch hẹn tư vấn (isTypeRevenue=1) kèm thông tin checkin — dùng để đối
// chiếu/theo dõi kết quả tư vấn. Field FK (chi nhánh, loại hẹn, nhóm
// dịch vụ, lý do từ chối, người tạo) CHỈ trả raw ID — FE tự resolve tên/avatar
// qua IndexedDB cache (đã có sẵn: users/branches/appointmentTypes/serviceTypes/
// appointmentReasons), KHÔNG BE join thêm tên hiển thị (xem architecture.md).
func (h *Handler) AppointmentConsultationReport(c *gin.Context) {
	q, err := buildAppointmentConsultationQuery(c, h.DB)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}

	if c.Query("_export") == "1" {
		exportHandlers.ExportAppointment(c, q)
		return
	}

	items := make([]map[string]interface{}, 0)
	result := pagination.Paginate(c, q, &items)

	if len(items) > 0 {
		var customerIds []interface{}
		for _, item := range items {
			if cid, ok := item["customer_id"]; ok && cid != nil {
				customerIds = append(customerIds, cid)
			}
		}
		if len(customerIds) > 0 {
			var customers []map[string]interface{}
			// Story-38: bỏ company — customer có thể thuộc CHI NHÁNH KHÁC chi nhánh của lịch
			// hẹn (đăng ký khách 1 nơi, tư vấn nơi khác) ⇒ CrossBranchLookup (không filter),
			// GIỮ NGUYÊN hành vi trước đây của helper company-only cũ (chỉ lọc company, không lọc
			// branch). Test: TestAppointmentConsultation_AttachKhachChiNhanhKhac.
			tx := tenant.CrossBranchLookup(h.DB, "customer").Table("customer").
				Select("id, code, old_code, fullname, phone, avatar").
				Where("id IN ?", customerIds).
				Find(&customers)
			if pagination.IsQueryError(tx.Error) {
				pagination.FailQuery(c, tx.Error)
				return
			}

			phoneutil.MaskPhoneInItems(phoneutil.CanSeePhone(c, h.DB), customers)
			storage.ResolveRows(customers, "avatar")
			cMap := make(map[int64]map[string]interface{})
			for _, cu := range customers {
				cMap[helpers.ToInt64(cu["id"])] = cu
			}
			for _, item := range items {
				item["customer"] = cMap[helpers.ToInt64(item["customer_id"])]
			}
		}
	}

	c.JSON(http.StatusOK, result)
}

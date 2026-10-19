package handler

import (
	"fmt"
	"net/http"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type branchThreshold struct {
	BranchId          int      `json:"branch_id"`
	AvgBooked         *float64 `json:"avg_booked"`
	SdBooked          *float64 `json:"sd_booked"`
	BusyThreshold     *float64 `json:"busy_threshold"`
	VeryBusyThreshold *float64 `json:"very_busy_threshold"`
}

// scheduleLoc — múi giờ của thương hiệu (story-36: qua tz.LocFromContext, không đọc company active).
func (h *Handler) scheduleLoc(c *gin.Context) *time.Location {
	return tz.LocFromContext(c, h.DB)
}

// tzOffset returns MySQL timezone offset string like '+07:00' from Location
func tzOffset(loc *time.Location) string {
	_, offset := time.Now().In(loc).Zone()
	h := offset / 3600
	m := (offset % 3600) / 60
	if m < 0 {
		m = -m
	}
	return fmt.Sprintf("%+03d:%02d", h, m)
}

// buildAppointmentBranchScopeSQL trả về fragment SQL
// ` AND (<alias>.branch_id = ? OR <alias>.branch_id IS NULL)` — mirror đúng nhánh branch của
// tenant.ScopedDBNullBranch (branchActiveId trước, rồi branchIds, luôn kèm NULL). Là phạm vi tenant
// DUY NHẤT của các raw SQL tổng hợp lịch hẹn (biểu đồ trạng thái, tổng quan năm, ngưỡng đông/vắng)
// sau khi story-36 (D15) bỏ vế company. NGOẶC là bắt buộc: fragment nối thẳng sau `AND`, mất ngoặc
// thì `OR branch_id IS NULL` thoát khỏi mọi điều kiện khác của WHERE/ON.
//
// Threshold "đông/vắng" của bảng lịch hẹn theo giờ khoá theo branch_id — phạm vi chi nhánh phải
// khớp đúng chi nhánh user được xem, nếu không sum hiển thị và baseline đo trên hai tập khác
// nhau, cột "Trạng thái" sẽ lệch.
func buildAppointmentBranchScopeSQL(c *gin.Context, alias string) (string, []any) {
	if bid := middleware.GetBranchActiveID(c); bid > 0 {
		return fmt.Sprintf(" AND (%s.branch_id = ? OR %s.branch_id IS NULL)", alias, alias), []any{bid}
	}
	if bids := middleware.GetBranchIDs(c); len(bids) > 0 {
		return fmt.Sprintf(" AND (%s.branch_id IN (?) OR %s.branch_id IS NULL)", alias, alias), []any{bids}
	}
	return "", nil
}

// buildAppointmentTypeScopeSQL trả về fragment SQL ` AND <alias>.appointment_type_id IN (?)`
// — dùng để threshold busyness cùng phạm vi loại lịch hẹn với sum hiển thị.
//
// typeIDs ở đây là danh sách loại ĐANG HIỂN THỊ (đã lọc status = 1 và giao với filter type_id
// của user), KHÔNG phải raw query param. Trước đây fragment chỉ được gắn khi user có filter
// type_id ⇒ mặc định threshold cộng cả lịch hẹn thuộc loại đã TẮT, trong khi bảng hiển thị bỏ
// chúng (pivot chỉ lặp theo `types` = status 1) ⇒ sum nhỏ hơn baseline một cách hệ thống.
func buildAppointmentTypeScopeSQL(typeIDs []int64, alias string) (string, []any) {
	if len(typeIDs) == 0 {
		return "", nil
	}
	return fmt.Sprintf(" AND %s.appointment_type_id IN (?)", alias), []any{typeIDs}
}

// activeAppointmentTypes lấy các loại lịch hẹn đang bật, giao với filter type_id (nếu có).
// Trả về cả rows (cho pivot dựng cột) lẫn id list (cho WHERE của threshold + query hiển thị) —
// MỘT nguồn sự thật duy nhất cho cả 3 chỗ, để không tái diễn lệch phạm vi.
// Danh sách rỗng ⇒ caller phải trả payload rỗng, KHÔNG được bỏ điều kiện (bỏ = trả về mọi loại).
func activeAppointmentTypes(db *gorm.DB, typeIds []string) ([]map[string]interface{}, []int64, error) {
	var types []map[string]interface{}
	tq := db.Table("appointment_type").Select("id, name").Where("status = 1").Order("SortOrder DESC, id ASC")
	if len(typeIds) > 0 {
		tq = tq.Where("id IN ?", typeIds)
	}
	if tx := tq.Find(&types); tx.Error != nil {
		return nil, nil, tx.Error
	}
	ids := make([]int64, 0, len(types))
	for _, tp := range types {
		ids = append(ids, helpers.ToInt64(tp["id"]))
	}
	return types, ids, nil
}

// AppointmentSchedule GET /crm/appointment/appointment-schedule
func (h *Handler) AppointmentSchedule(c *gin.Context) {
	branchIDs := middleware.GetBranchIDs(c)
	loc := h.scheduleLoc(c)
	offset := tzOffset(loc) // "+07:00"
	// offsetSec = số giây lệch giữa company TZ và UTC (vd 25200 cho ICT).
	// Dùng cho FLOOR((appointment_date + offsetSec) / 86400) → bucket day theo TZ company,
	// thay vì hardcode 25200.
	_, offsetSec := time.Now().In(loc).Zone()
	now := time.Now().In(loc)

	// Timestamps for threshold: 56 days ago → yesterday end
	ago56 := time.Date(now.Year(), now.Month(), now.Day()-56, 0, 0, 0, 0, loc).Unix()
	yesterdayEnd := time.Date(now.Year(), now.Month(), now.Day()-1, 23, 59, 59, 0, loc).Unix()
	// Timestamps for schedule: today start → +1 month end
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Unix()
	nextMonthEnd := time.Date(now.Year(), now.Month()+1, now.Day()+1, 0, 0, 0, 0, loc).Unix() - 1

	// Loại lịch hẹn lấy TRƯỚC threshold query — threshold phải cùng phạm vi loại với sum hiển thị
	// (pivotAppointments), nếu không baseline "đông/vắng" sẽ tính trên tập loại rộng hơn tập được
	// cộng vào sum → lệch, luôn báo "Bình thường" sai. Một nguồn sự thật cho cả 3 chỗ:
	// threshold, query hiển thị, và cột của pivot.
	typeIds := query.GetFilterValues(c, "type_id")
	types, activeTypeIDs, err := activeAppointmentTypes(h.DB, typeIds)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}
	if len(activeTypeIDs) == 0 {
		// Không loại nào hiển thị được ⇒ payload rỗng. KHÔNG bỏ điều kiện loại: bỏ = trả về mọi loại.
		c.JSON(http.StatusOK, pivotAppointments(nil, nil, nil, nil))
		return
	}
	typeScopeSQL, typeScopeArgs := buildAppointmentTypeScopeSQL(activeTypeIDs, "a")

	// 1) Threshold từ lịch sử 8 tuần — phạm vi PHẢI trùng query hiển thị ở bước 2
	// (branch + loại), nếu không sum và baseline đo trên hai tập khác nhau.
	branchScopeSQL, branchScopeArgs := buildAppointmentBranchScopeSQL(c, "a")
	var thresholds []branchThreshold
	thresholdArgs := append([]any{ago56, yesterdayEnd}, branchScopeArgs...)
	thresholdArgs = append(thresholdArgs, typeScopeArgs...)
	thresholdSQL := fmt.Sprintf(`
		SELECT branch_id,
			AVG(booked) as avg_booked,
			STDDEV_SAMP(booked) as sd_booked,
			AVG(booked)+STDDEV_SAMP(booked) as busy_threshold,
			AVG(booked)+2*STDDEV_SAMP(booked) as very_busy_threshold
		FROM (
			SELECT FLOOR((a.appointment_date + %d) / 86400) AS day, a.branch_id, COUNT(*) AS booked
			FROM appointment a
			WHERE a.appointment_date BETWEEN ? AND ?
				AND a.status != %d`+branchScopeSQL+typeScopeSQL+`
			GROUP BY day, a.branch_id
		) d GROUP BY branch_id`, offsetSec, define.AppointmentStatusCancel)
	// KHÔNG nuốt lỗi: threshold hỏng ⇒ thMap rỗng ⇒ classify() trả "Bình thường" cho MỌI ô,
	// nhìn y hệt một ngày vắng khách. Sai im lặng kiểu này không ai phát hiện được từ UI.
	if tx := h.DB.Raw(thresholdSQL, thresholdArgs...).Scan(&thresholds); pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	thMap := make(map[int]branchThreshold)
	for _, t := range thresholds {
		thMap[t.BranchId] = t
	}

	// 2) Query: today → +1 month, group by day × branch × type
	// CONVERT_TZ với source = @@session.time_zone (FROM_UNIXTIME trả về session TZ time, không phải UTC).
	// Mỗi company có thể có TZ riêng → offset là động. Helper tz.SqlFromUnix wrap pattern này.
	dtA := tz.SqlFromUnix("appointment.appointment_date", offset)
	q := tenant.ScopedDBNullBranch(c, h.DB, "appointment").
		Table("appointment").
		Select(fmt.Sprintf(`DATE_FORMAT(`+dtA+`, '%%d-%%m-%%Y') as day,
			FLOOR((appointment.appointment_date + %d) / 86400) as day_sort,
			b.id as branch_id, b.name as branch_name,
			t.id as type_id, t.name as type_name,
			COUNT(*) as booked`, offsetSec)).
		Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
		Joins("INNER JOIN appointment_type t ON t.id = appointment.appointment_type_id").
		Where("appointment.appointment_date BETWEEN ? AND ?", todayStart, nextMonthEnd).
		Where("appointment.status != ?", define.AppointmentStatusCancel).
		Where("appointment.appointment_type_id IN ?", activeTypeIDs)

	var rows []map[string]interface{}
	tx := q.Group("day, day_sort, branch_id, branch_name, type_id, type_name").Order("day_sort ASC").Find(&rows)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	// 3) Branches + types for pivot
	var branches []map[string]interface{}
	bq := h.DB.Table("branch").Select("id, name").Where("status = 1").Order("SortOrder DESC, id ASC")
	if len(branchIDs) > 0 {
		bq = bq.Where("id IN ?", branchIDs)
	}
	tx = bq.Find(&branches)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	// types đã lấy ở đầu handler (cùng nguồn với activeTypeIDs dùng cho threshold + query hiển thị).

	// 4) Pivot
	c.JSON(http.StatusOK, pivotAppointments(rows, branches, types, thMap))
}

func pivotAppointments(rows []map[string]interface{}, branches, types []map[string]interface{}, thMap map[int]branchThreshold) gin.H {
	matrix := map[string]map[int64]map[int64]int{}
	var days []string
	daySet := map[string]bool{}

	for _, r := range rows {
		day, _ := r["day"].(string)
		bid := helpers.ToInt64(r["branch_id"])
		tid := helpers.ToInt64(r["type_id"])
		booked := int(helpers.ToInt64(r["booked"]))

		if !daySet[day] {
			daySet[day] = true
			days = append(days, day)
			matrix[day] = map[int64]map[int64]int{}
			for _, br := range branches {
				bID := helpers.ToInt64(br["id"])
				matrix[day][bID] = map[int64]int{}
				for _, tp := range types {
					matrix[day][bID][helpers.ToInt64(tp["id"])] = 0
				}
			}
		}
		if typeMap, ok := matrix[day][bid]; ok {
			typeMap[tid] = booked
		}
	}

	// Columns
	var columns []gin.H
	for _, br := range branches {
		bID := helpers.ToInt64(br["id"])
		bName, _ := br["name"].(string)
		var cols []gin.H
		for _, tp := range types {
			tID := helpers.ToInt64(tp["id"])
			tName, _ := tp["name"].(string)
			cols = append(cols, gin.H{"key": fmt.Sprintf("b%d_t%d", bID, tID), "label": tName})
		}
		cols = append(cols, gin.H{"key": fmt.Sprintf("b%d_status", bID), "label": "Trạng thái"})
		columns = append(columns, gin.H{"groupLabel": bName, "branchId": bID, "cols": cols})
	}

	// Rows
	var rowsOut []gin.H
	for _, day := range days {
		cells := gin.H{}
		for _, br := range branches {
			bID := helpers.ToInt64(br["id"])
			sum := 0
			for _, tp := range types {
				tID := helpers.ToInt64(tp["id"])
				key := fmt.Sprintf("b%d_t%d", bID, tID)
				val := matrix[day][bID][tID]
				cells[key] = val
				sum += val
			}
			cells[fmt.Sprintf("b%d_status", bID)] = classifyBusyness(sum, int(bID), thMap)
		}
		rowsOut = append(rowsOut, gin.H{"day": day, "cells": cells})
	}

	return gin.H{"columns": columns, "rows": rowsOut}
}

// classifyBusyness trả về MÃ trạng thái ổn định (không phải text hiển thị) — FE tự dịch qua i18n
// (common.label.normal / crm.appointment.label.busy / crm.appointment.label.very_busy) và so khớp
// severity theo mã, không theo text hiển thị (tránh vỡ khi đổi ngôn ngữ hoặc đổi câu chữ).
func classifyBusyness(sum int, branchId int, thMap map[int]branchThreshold) string {
	th, ok := thMap[branchId]
	if !ok || th.SdBooked == nil {
		return "unknown"
	}
	if th.VeryBusyThreshold != nil && float64(sum) >= *th.VeryBusyThreshold {
		return "very_busy"
	}
	if th.BusyThreshold != nil && float64(sum) >= *th.BusyThreshold {
		return "busy"
	}
	return "normal"
}

func classify(sum int, id int, thMap map[int]branchThreshold) string {
	return classifyBusyness(sum, id, thMap)
}

// buildHourTimeSlots dựng danh sách slot 30 phút cho bảng lịch hẹn theo giờ.
// Khung mặc định 7:00–20:30, nhưng NỚI ra đủ chứa mọi slot thực sự có trong dữ liệu của
// ngày đó. Trước đây khung bị hardcode 7–20: pivot lookup trượt rồi `continue`, mọi lịch
// hẹn ngoài khung biến mất khỏi bảng mà không có dấu hiệu gì — tổng hiển thị thiếu im
// lặng (đo trên DB dev: 25/160.253 lịch hẹn năm 2026, status != cancel, nằm ngoài 7h–20h).
func buildHourTimeSlots(rows []map[string]interface{}) []string {
	fromH, toH := 7, 20
	for _, r := range rows {
		slot, _ := r["time_slot"].(string)
		var hour, minute int
		if _, err := fmt.Sscanf(slot, "%d:%d", &hour, &minute); err != nil {
			continue
		}
		if hour < 0 || hour > 23 {
			continue
		}
		if hour < fromH {
			fromH = hour
		}
		if hour > toH {
			toH = hour
		}
	}
	var slots []string
	for h := fromH; h <= toH; h++ {
		slots = append(slots, fmt.Sprintf("%d:00", h))
		slots = append(slots, fmt.Sprintf("%d:30", h))
	}
	return slots
}

// AppointmentScheduleByHour GET /crm/appointment/appointment-schedule-byhour?appointment_date=2026-04-06
func (h *Handler) AppointmentScheduleByHour(c *gin.Context) {
	branchIDs := middleware.GetBranchIDs(c)
	loc := h.scheduleLoc(c)
	offset := tzOffset(loc)

	day := c.DefaultQuery("appointment_date", time.Now().In(loc).Format("2006-01-02"))
	fromTs, toTs := tz.DayRange(loc, day)

	// Chi nhánh + loại lấy TRƯỚC query hiển thị. pivotByTimeSlot chỉ dựng ô theo `branches`
	// (status = 1, đã lọc branch scope) và `types` (status = 1); dòng nào rơi ngoài hai
	// tập đó bị `continue` nuốt mất — đếm thiếu im lặng, không log, không lỗi. Ràng buộc query
	// vào ĐÚNG hai tập sẽ hiển thị ⇒ SQL và pivot luôn cùng phạm vi (và buildHourTimeSlots không
	// còn nới khung giờ theo dòng sẽ bị vứt bỏ).
	var branches []map[string]interface{}
	bq := h.DB.Table("branch").Select("id, name").Where("status = 1").Order("SortOrder DESC, id ASC")
	if len(branchIDs) > 0 {
		bq = bq.Where("id IN ?", branchIDs)
	}
	if tx := bq.Find(&branches); pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	shownBranchIDs := make([]int64, 0, len(branches))
	for _, br := range branches {
		shownBranchIDs = append(shownBranchIDs, helpers.ToInt64(br["id"]))
	}

	types, activeTypeIDs, err := activeAppointmentTypes(h.DB, nil)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}

	if len(shownBranchIDs) == 0 || len(activeTypeIDs) == 0 {
		// Không chi nhánh/loại nào hiển thị được ⇒ payload rỗng. KHÔNG bỏ điều kiện: bỏ = trả về tất cả.
		c.JSON(http.StatusOK, pivotByTimeSlot(nil, nil, nil, buildHourTimeSlots(nil), day))
		return
	}

	timeSlotExpr := fmt.Sprintf(`CONCAT(HOUR(CONVERT_TZ(FROM_UNIXTIME(appointment.appointment_date), @@session.time_zone, '%s')), ':', CASE WHEN MINUTE(CONVERT_TZ(FROM_UNIXTIME(appointment.appointment_date), @@session.time_zone, '%s')) < 30 THEN '00' ELSE '30' END)`, offset, offset)

	// Query: group by 30-min slot × branch × type
	q := tenant.ScopedDBNullBranch(c, h.DB, "appointment").
		Table("appointment").
		Select(timeSlotExpr+` as time_slot,
			b.id as branch_id, b.name as branch_name,
			t.id as type_id, t.name as type_name,
			COUNT(*) as booked`).
		Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
		Joins("INNER JOIN appointment_type t ON t.id = appointment.appointment_type_id").
		Where("appointment.appointment_date BETWEEN ? AND ?", fromTs, toTs).
		Where("appointment.status != ?", define.AppointmentStatusCancel).
		Where("appointment.branch_id IN ?", shownBranchIDs).
		Where("appointment.appointment_type_id IN ?", activeTypeIDs)

	var rows []map[string]interface{}
	// Không ORDER BY: pivot dựng output từ mảng timeSlots nên thứ tự dòng trả về không được
	// dùng (mà sắp theo chuỗi còn sai: "10:00" < "7:00"). Bench 665k dòng, ngày 11.090 lịch
	// hẹn: 29–48ms có ORDER BY → 22–24ms khi bỏ.
	tx := q.Group("time_slot, appointment.branch_id, b.name, t.id, t.name").Find(&rows)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	timeSlots := buildHourTimeSlots(rows)

	c.JSON(http.StatusOK, pivotByTimeSlot(rows, branches, types, timeSlots, day))
}

func pivotByTimeSlot(rows []map[string]interface{}, branches, types []map[string]interface{}, timeSlots []string, dayLabel string) gin.H {
	// matrix[slot][branchId][typeId] = booked
	matrix := map[string]map[int64]map[int64]int{}
	for _, slot := range timeSlots {
		matrix[slot] = map[int64]map[int64]int{}
		for _, br := range branches {
			bID := helpers.ToInt64(br["id"])
			matrix[slot][bID] = map[int64]int{}
			for _, tp := range types {
				matrix[slot][bID][helpers.ToInt64(tp["id"])] = 0
			}
		}
	}
	for _, r := range rows {
		slot, _ := r["time_slot"].(string)
		slotMap, ok := matrix[slot]
		if !ok {
			continue
		}
		typeMap, ok := slotMap[helpers.ToInt64(r["branch_id"])]
		if !ok {
			continue
		}
		typeMap[helpers.ToInt64(r["type_id"])] = int(helpers.ToInt64(r["booked"]))
	}

	// Columns
	var columns []gin.H
	for _, br := range branches {
		bID := helpers.ToInt64(br["id"])
		bName, _ := br["name"].(string)
		var cols []gin.H
		for _, tp := range types {
			tID := helpers.ToInt64(tp["id"])
			tName, _ := tp["name"].(string)
			cols = append(cols, gin.H{"key": fmt.Sprintf("b%d_t%d", bID, tID), "label": tName})
		}
		columns = append(columns, gin.H{"groupLabel": bName, "branchId": bID, "cols": cols})
	}

	// Rows: each row = 30-min slot
	var rowsOut []gin.H
	for _, slot := range timeSlots {
		cells := gin.H{}
		for _, br := range branches {
			bID := helpers.ToInt64(br["id"])
			for _, tp := range types {
				tID := helpers.ToInt64(tp["id"])
				key := fmt.Sprintf("b%d_t%d", bID, tID)
				cells[key] = matrix[slot][bID][tID]
			}
		}
		// Format: "7:00–7:30"
		var hour, minute int
		fmt.Sscanf(slot, "%d:%d", &hour, &minute)
		nextH, nextM := hour, minute+30
		if nextM >= 60 {
			nextH++
			nextM = 0
		}
		label := fmt.Sprintf("%d:%02d–%d:%02d", hour, minute, nextH, nextM)
		rowsOut = append(rowsOut, gin.H{"timeSlot": label, "cells": cells})
	}

	return gin.H{"date": dayLabel, "columns": columns, "rows": rowsOut}
}

// appointmentScheduleList — logic for byhour-list
//
// ⚠ `orderBy` PHẢI kết thúc bằng một khoá duy nhất (`appointment.id ASC`). Hàm này chạy
// `.Offset(offset).Limit(perPage)` với perPage mặc định 30 ⇒ phân trang thật. ORDER BY không
// duy nhất + LIMIT/OFFSET = MySQL được tự do đổi thứ tự giữa 2 query ⇒ một lịch hẹn ra 2 lần ở
// trang này và BIẾN MẤT khỏi trang kia — mất dòng im lặng, không có lỗi nào được ném ra.
// Không phải rủi ro lý thuyết: đo trên DB dev, riêng ngày 2026-09-06 có 934/1064 dòng nằm trong
// nhóm trùng khoá (branch_id, appointment_date).
func (h *Handler) appointmentScheduleList(c *gin.Context, orderBy string) {
	loc := h.scheduleLoc(c)
	day := c.DefaultQuery("appointment_date", time.Now().In(loc).Format("2006-01-02"))
	page := 1
	perPage := 30
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := c.Query("per_page"); v != "" {
		fmt.Sscanf(v, "%d", &perPage)
	}
	// per_page > 10000: cap cứng. <= 10000: cho qua nguyên giá trị — dùng chung cho cả
	// list tương tác (mặc định nhỏ) lẫn luồng print (per_page lớn hơn để lấy hết 1 ngày).
	if perPage > 10000 {
		perPage = 10000
	}

	fromT, toT := tz.DayRange(loc, day)

	// S-10: phạm vi telesale — danh sách theo giờ liệt kê TÊN khách ⇒ chỉ khách mình phụ trách.
	teleCond, teleArgs, err := scope.CustomerIDCond(c, h.DB, "appointment.customer_id")
	if err != nil {
		response.ServerError(c, err)
		return
	}

	// Explicit select — chỉ field FE (schedule-byhour-list) thực sự dùng, không SELECT
	// appointment.* (tránh kéo theo ~70 cột EMR nội bộ ra endpoint lịch hẹn theo giờ).
	// Không cần DISTINCT: customer/branch/appointment_type join theo PK (1:1), service
	// join theo PK — không thể nhân bản dòng.
	baseQ := tenant.ScopedDBNullBranch(c, h.DB, "appointment").
		Table("appointment").
		Joins("INNER JOIN customer c ON c.id = appointment.customer_id").
		Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
		Joins("INNER JOIN appointment_type t ON t.id = appointment.appointment_type_id").
		Joins("LEFT JOIN service s ON s.id = appointment.service_id").
		Where("appointment.appointment_date BETWEEN ? AND ?", fromT, toT).
		Where("appointment.status != ?", define.AppointmentStatusCancel)
	if teleCond != "" {
		baseQ = baseQ.Where(teleCond, teleArgs...)
	}

	// loadAll: caller (schedule-byhour-list, print/export) truyền
	// per_page lớn để lấy hết lịch hẹn trong ngày, không phân trang thật → bỏ COUNT(*) (JOIN 4
	// bảng) vì totalCount/hasMore không được dùng, suy trực tiếp từ số item đã fetch.
	loadAll := perPage >= 1000

	var total int64
	if !loadAll {
		baseQ.Session(&gorm.Session{}).Count(&total)
	}

	q := baseQ.Select(`appointment.id, appointment.customer_id, appointment.appointment_date,
		appointment.branch_id, appointment.status, appointment.content,
		IF(c.fullname IS NOT NULL AND c.fullname != '', c.fullname, c.nickname) as customer_name,
		b.name as branch_name,
		t.name as appointment_type_name,
		s.name as service_name`)

	offset := (page - 1) * perPage
	var items []map[string]interface{}
	tx := q.Order(orderBy).Offset(offset).Limit(perPage).Find(&items)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	if items == nil {
		items = make([]map[string]interface{}, 0)
	}

	if loadAll {
		total = int64(offset + len(items))
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       items,
		"totalCount": total,
		"page":       page,
		"perPage":    perPage,
		"hasMore":    int64(offset+len(items)) < total,
	})
}

// AppointmentScheduleByHourList GET /crm/appointment/appointment-schedule-byhour-list
func (h *Handler) AppointmentScheduleByHourList(c *gin.Context) {
	h.appointmentScheduleList(c, "appointment.branch_id ASC, appointment.appointment_date ASC, appointment.id ASC")
}

type branchAppointmentGroup struct {
	BranchID   int64 `json:"branch_id"`
	BranchName string
	Count      int64
}

// scheduleGroupPageRowBudget — trần SỐ LỊCH HẸN mỗi trang của các endpoint *-grouped.
//
// VÌ SAO CÓ TRẦN NÀY (đo 2026-09-06, DB bench 751.919 lịch hẹn / 17 chi nhánh / 77k khách):
// phân trang theo chi nhánh/bác sĩ KHÔNG chặn được số dòng, vì số lịch hẹn TRONG mỗi nhóm
// không có trần. Trang 1 (8 chi nhánh) trả:
//
//	   514 lịch/ngày  →   234 dòng /  93 KB / 15-59ms
//	 1.064 lịch/ngày  →   629 dòng / 207 KB / 25-28ms      (tải thật hiện tại)
//	 4.420 lịch/ngày  → 2.872 dòng / 2,0 MB / 78-103ms
//	12.213 lịch/ngày  → 6.911 dòng / 3,6 MB / 134ms nóng, 1,05s nguội
//	12.213 lịch/ngày, per_page=50 → 11.710 dòng / 6,2 MB / 451ms nóng, 2,93s nguội
//
// SQL không phải nút thắt (cả 2 query đều index range scan trên idx_appointment_date_scope,
// chi phí tỉ lệ số lịch TRONG NGÀY chứ không theo kích thước bảng) — payload mới là.
//
// Trần 800 chọn để KHÔNG đổi hành vi ở tải thật hiện tại (ngày đông nhất 1.064 lịch → trang 1
// vẫn gom đủ 8 chi nhánh như trước), chỉ bắt đầu cắt trang nhỏ lại từ ~4.000 lịch/ngày trở lên.
//
// Biến thể ĐÃ THỬ VÀ LOẠI — đừng làm lại: cắt trần 300 dòng/nhóm bằng
// ROW_NUMBER() OVER (PARTITION BY branch_id) ở SQL = 338-372ms vs 214-854ms của cách hiện tại,
// tức KHÔNG nhanh hơn ở DB (window function vẫn materialize đủ 6.911 dòng rồi mới lọc rn<=300),
// mà lại cắt mất dữ liệu. Trần phải là trần SỐ NHÓM/trang, không phải trần dòng TRONG nhóm.
const scheduleGroupPageRowBudget = 800

// scheduleGroupPageWindow trả [start, end) của trang `page` khi cắt trang theo HAI trần cùng lúc:
// tối đa `perPage` nhóm VÀ tối đa scheduleGroupPageRowBudget lịch hẹn.
//
// counts[i] = số lịch hẹn của nhóm thứ i, theo ĐÚNG thứ tự nhóm sẽ trả về. Vì mọi nhóm của ngày
// đã có sẵn trong bộ nhớ (query aggregate rẻ, số nhóm/ngày nhỏ), biên trang tính được xác định
// và ổn định cho cùng một ngày — FE cứ tăng page lên 1 như cũ, không cần đổi gì.
//
// MỘT NHÓM TỰ NÓ VƯỢT TRẦN VẪN ĐƯỢC TRẢ NGUYÊN VẸN (đứng một mình trong trang). Không bao giờ
// cắt bớt lịch hẹn bên trong một nhóm — cắt bớt = trả thiếu im lặng, FE không có cách nào biết.
func scheduleGroupPageWindow(counts []int64, page, perPage int) (int, int) {
	total := len(counts)
	start := 0
	for p := 1; ; p++ {
		if start >= total {
			return total, total
		}
		end := start
		var rows int64
		for end < total && end-start < perPage {
			// end == start: nhóm đầu tiên của trang luôn được nhận, kể cả khi một mình nó
			// đã vượt trần — nếu không sẽ sinh trang rỗng và vòng lặp không tiến được.
			if end > start && rows+counts[end] > scheduleGroupPageRowBudget {
				break
			}
			rows += counts[end]
			end++
		}
		if p == page {
			return start, end
		}
		start = end
	}
}

// AppointmentScheduleByHourGrouped GET /crm/appointment/appointment-schedule-byhour-grouped
// Trả lịch hẹn phân trang theo chi nhánh (không phải theo appointment) — FE dùng infinite scroll
// tải thêm N chi nhánh mỗi lần thay vì load hết toàn bộ appointment trong ngày cùng lúc (ngày
// đông có thể ~2000 dòng, render 1 lần rất nặng). page/per_page ở đây tính theo SỐ CHI NHÁNH,
// nhưng trang còn bị chặn thêm bởi scheduleGroupPageRowBudget: ngày quá đông thì một trang trả
// ÍT chi nhánh hơn per_page để payload không nở (xem scheduleGroupPageWindow). Chi nhánh nào đã
// nằm trong trang thì luôn có ĐỦ lịch hẹn của nó — không cắt bớt.
func (h *Handler) AppointmentScheduleByHourGrouped(c *gin.Context) {
	loc := h.scheduleLoc(c)
	day := c.DefaultQuery("appointment_date", time.Now().In(loc).Format("2006-01-02"))
	page := 1
	perPage := 8
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := c.Query("per_page"); v != "" {
		fmt.Sscanf(v, "%d", &perPage)
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 8
	}

	fromT, toT := tz.DayRange(loc, day)

	// S-10: phạm vi telesale — áp CẢ câu đếm theo chi nhánh lẫn câu lấy dòng (cùng một tập).
	teleCond, teleArgs, err := scope.CustomerIDCond(c, h.DB, "appointment.customer_id")
	if err != nil {
		response.ServerError(c, err)
		return
	}
	tele := func(q *gorm.DB) *gorm.DB {
		if teleCond == "" {
			return q
		}
		return q.Where(teleCond, teleArgs...)
	}

	// 1) Aggregate chi nhánh có lịch hẹn trong ngày — rẻ (không JOIN customer/service), dùng để
	// phân trang. Số chi nhánh/ngày thường nhỏ (vài chục) nên cắt trang bằng Go sau khi Find hết,
	// không cần LIMIT/OFFSET riêng ở DB.
	var branchGroups []branchAppointmentGroup
	aggTx := tele(tenant.ScopedDBNullBranch(c, h.DB, "appointment").
		Table("appointment")).
		Select("appointment.branch_id as branch_id, b.name as branch_name, COUNT(*) as count").
		Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
		Where("appointment.appointment_date BETWEEN ? AND ?", fromT, toT).
		Where("appointment.status != ?", define.AppointmentStatusCancel).
		Group("appointment.branch_id, b.name").
		// tiebreaker: 2 chi nhanh trung ten => thu tu nhay => ranh gioi trang nhay theo
		Order("b.name ASC, appointment.branch_id ASC").
		Find(&branchGroups)
	// Query aggregate này quyết định TOÀN BỘ nội dung trang: nuốt lỗi ở đây = trả lịch
	// RỖNG kèm HTTP 200, người dùng tin là "hôm nay không có lịch hẹn". Đã gặp thật khi
	// một bảng join vắng mặt (Error 1146) — chỉ log ERROR, response vẫn 200 groups:[].
	if pagination.IsQueryError(aggTx.Error) {
		pagination.FailQuery(c, aggTx.Error)
		return
	}

	total := len(branchGroups)
	counts := make([]int64, len(branchGroups))
	for i, g := range branchGroups {
		counts[i] = g.Count
	}
	offset, end := scheduleGroupPageWindow(counts, page, perPage)
	pageGroups := branchGroups[offset:end]

	branchIDs := make([]int64, 0, len(pageGroups))
	for _, g := range pageGroups {
		branchIDs = append(branchIDs, g.BranchID)
	}

	// 2) Full appointment rows chỉ cho các branch thuộc trang này.
	items := make([]map[string]interface{}, 0)
	if len(branchIDs) > 0 {
		tx := tele(tenant.ScopedDBNullBranch(c, h.DB, "appointment").
			Table("appointment")).
			Select(`appointment.id, appointment.customer_id, appointment.appointment_date,
				appointment.branch_id, appointment.status, appointment.content,
				IF(c.fullname IS NOT NULL AND c.fullname != '', c.fullname, c.nickname) as customer_name,
				b.name as branch_name,
				t.name as appointment_type_name,
				s.name as service_name`).
			Joins("INNER JOIN customer c ON c.id = appointment.customer_id").
			Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
			Joins("INNER JOIN appointment_type t ON t.id = appointment.appointment_type_id").
			Joins("LEFT JOIN service s ON s.id = appointment.service_id").
			Where("appointment.appointment_date BETWEEN ? AND ?", fromT, toT).
			Where("appointment.status != ?", define.AppointmentStatusCancel).
			Where("appointment.branch_id IN ?", branchIDs).
			// `appointment.id` là tiebreaker BẮT BUỘC, không phải trang trí: (branch_id,
			// appointment_date) KHÔNG duy nhất — đo trên DB dev (218k dòng), riêng ngày 2026-09-06 đã
			// có 261 nhóm trùng cả 2 cột, phủ 934/1064 dòng của ngày. Thiếu tiebreaker ⇒ thứ tự trong
			// nhóm do plan quyết định, đổi khi optimizer đổi index hoặc buffer pool đổi ⇒ danh sách
			// nhảy thứ tự giữa 2 lần load cùng ngày.
			Order("appointment.branch_id ASC, appointment.appointment_date ASC, appointment.id ASC").
			Find(&items)
		if pagination.IsQueryError(tx.Error) {
			pagination.FailQuery(c, tx.Error)
			return
		}
	}

	itemsByBranch := make(map[int64][]map[string]interface{}, len(pageGroups))
	for _, item := range items {
		bid := helpers.ToInt64(item["branch_id"])
		itemsByBranch[bid] = append(itemsByBranch[bid], item)
	}

	groups := make([]gin.H, 0, len(pageGroups))
	for _, g := range pageGroups {
		appointments := itemsByBranch[g.BranchID]
		if appointments == nil {
			appointments = make([]map[string]interface{}, 0)
		}
		groups = append(groups, gin.H{
			"branch_id":    g.BranchID,
			"branch_name":  g.BranchName,
			"appointments": appointments,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"groups":     groups,
		"page":       page,
		"perPage":    perPage,
		"totalCount": total,
		"hasMore":    end < total,
	})
}

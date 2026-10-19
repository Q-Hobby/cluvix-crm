package export

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"
)

// appointmentCol định nghĩa 1 cột export, key khớp 1-1 với `value` trong
// AppointmentInMenu.displayCols (frontend) — xem .skill/backend/modules/ops/export.md § colDef pattern.
type appointmentCol struct {
	key     string
	header  string
	extract func(row map[string]interface{}, showPhone bool) interface{}
}

func apptCol(key, header string, fn func(row map[string]interface{}) interface{}) appointmentCol {
	return appointmentCol{key, header, func(row map[string]interface{}, _ bool) interface{} { return fn(row) }}
}

// appointmentColsFor — thứ tự + key phải khớp AppointmentInMenu.displayCols (crm/appointment/list/in-menu).
// loc dùng cho các cột format ngày/giờ theo timezone company đang active.
func appointmentColsFor(loc *time.Location) []appointmentCol {
	return []appointmentCol{
		apptCol("id", "ID", func(r map[string]interface{}) interface{} { return r["id"] }),
		apptCol("codeCustomer", "Mã khách hàng", func(r map[string]interface{}) interface{} { return r["customer_code"] }),
		apptCol("codeOldCustomer", "Mã khách cũ", func(r map[string]interface{}) interface{} { return r["customer_old_code"] }),
		apptCol("appointment_time", "Thời gian", func(r map[string]interface{}) interface{} { return fmtUnixHHMM(r["appointment_date"], loc) }),
		apptCol("fullname", "Khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }),
		apptCol("appointment_code", "Mã lịch hẹn", func(r map[string]interface{}) interface{} { return r["code"] }),
		{"phone", "Số điện thoại", func(r map[string]interface{}, showPhone bool) interface{} {
			return maskPhoneExport(showPhone, r["customer_phone"])
		}},
		apptCol("bookType", "Phân loại", func(r map[string]interface{}) interface{} {
			return define.BookTypeLabels[helpers.ToInt64(r["bookType"])]
		}),
		apptCol("appointmentType", "Loại lịch hẹn", func(r map[string]interface{}) interface{} { return r["appointment_type_name"] }),
		apptCol("appointment_date", "Ngày hẹn", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["appointment_date"], loc) }),
		apptCol("appointment_date_first", "Ngày hẹn đầu", func(r map[string]interface{}) interface{} { return fmtUnixDate(r["appointment_date_first"], loc) }),
		apptCol("appointmentListDate", "Ngày hẹn đã đổi", func(r map[string]interface{}) interface{} {
			return joinAppointmentListDate(r["appointmentListDate"], loc)
		}),
		apptCol("confirmAppointment", "Xác nhận lịch", func(r map[string]interface{}) interface{} {
			return define.ConfirmAppointmentLabels[helpers.ToInt64(r["confirmAppointment"])]
		}),
		apptCol("appointment_number", "Lần hẹn", func(r map[string]interface{}) interface{} { return r["appointment_number"] }),
		apptCol("content", "Nội dung hẹn", func(r map[string]interface{}) interface{} { return stripHTML(r["content"]) }),
		apptCol("contentPrivate", "Ghi chú nội bộ", func(r map[string]interface{}) interface{} { return stripHTML(r["contentPrivate"]) }),
		apptCol("checkin_date", "Checkin", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["checkin_date"], loc) }),
		apptCol("checkout_date", "Checkout", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["checkout_date"], loc) }),
		apptCol("typeServiceInterested", "Nhóm dịch vụ quan tâm", func(r map[string]interface{}) interface{} { return r["type_service_interested_name"] }),
		apptCol("orderServiceUsed", "Dịch vụ hẹn", func(r map[string]interface{}) interface{} { return r["service_name"] }),
		apptCol("source", "Nguồn khách hàng", func(r map[string]interface{}) interface{} { return r["source_name"] }),
		apptCol("convert_id", "Convert", func(r map[string]interface{}) interface{} { return r["convert_fullname"] }),
		apptCol("revenue_id", "Tư vấn viên", func(r map[string]interface{}) interface{} { return r["revenue_fullname"] }),
		apptCol("appointmentIsAccepted", "Trạng thái", func(r map[string]interface{}) interface{} {
			status := define.AppointmentStatusLabels[helpers.ToInt64(r["appointment_status"])]
			if accepted := define.IsAcceptedLabels[helpers.ToInt64(r["is_accepted"])]; accepted != "" {
				return status + " - " + accepted
			}
			return status
		}),
		apptCol("noteFail", "Lý do", func(r map[string]interface{}) interface{} { return r["appointment_reason_name"] }),
		apptCol("type_service_accepted_id", "Dịch vụ đồng ý", func(r map[string]interface{}) interface{} { return r["type_service_accepted_name"] }),
		apptCol("province", "Tỉnh thành", func(r map[string]interface{}) interface{} { return r["province_name"] }),
		apptCol("branch", "Chi nhánh", func(r map[string]interface{}) interface{} { return r["branch_name"] }),
		apptCol("created_by", "Người tạo", func(r map[string]interface{}) interface{} { return r["user_created_fullname"] }),
		apptCol("created_at", "Ngày tạo", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["created_at"], loc) }),
		apptCol("noteAppointment", "Ghi chú lịch hẹn", func(r map[string]interface{}) interface{} { return stripHTML(r["noteAppointment"]) }),
	}
}

func ExportAppointment(c *gin.Context, q *gorm.DB) {
	loc := tz.LocFromContext(c, q.Session(&gorm.Session{NewDB: true}))
	showPhone := phoneutil.CanSeePhoneExport(c, q.Session(&gorm.Session{NewDB: true}))

	eq := q.Session(&gorm.Session{}).
		Select(`appointment.id, appointment.code,
			customer.code as customer_code, customer.old_code as customer_old_code,
			customer.fullname as customer_fullname, customer.phone as customer_phone,
			src.name as source_name,
			at.name as appointment_type_name,
			appointment.bookType, appointment.status as appointment_status,
			appointment.is_accepted, appointment.confirmAppointment,
			appointment.appointment_number, appointment.appointment_date_first,
			appointment.appointmentListDate,
			appointment.appointment_date, appointment.checkin_date, appointment.checkout_date,
			appointment.content, appointment.contentPrivate, appointment.noteAppointment,
			tsi.name as type_service_interested_name, svc.name as service_name,
			rev.fullname as revenue_fullname,
			cv.fullname as convert_fullname,
			tsa.name as type_service_accepted_name,
			ar.name as appointment_reason_name,
			prov.name as province_name,
			br.name as branch_name,
			appointment.created_at, uc.fullname as user_created_fullname`).
		Joins("LEFT JOIN source src ON src.id = appointment.source_id").
		Joins("LEFT JOIN appointment_type at ON at.id = appointment.appointment_type_id").
		Joins("LEFT JOIN service_type tsi ON tsi.id = appointment.type_service_id").
		Joins("LEFT JOIN service svc ON svc.id = appointment.service_id").
		Joins("LEFT JOIN user_profile rev ON rev.user_id = appointment.revenue_id").
		Joins("LEFT JOIN user_profile cv ON cv.user_id = appointment.convert_id").
		Joins("LEFT JOIN service_type tsa ON tsa.id = appointment.type_service_accepted_id").
		Joins("LEFT JOIN appointment_reason ar ON ar.id = appointment.appointment_reason_id").
		Joins("LEFT JOIN location_province prov ON prov.id = customer.province_id").
		Joins("LEFT JOIN branch br ON br.id = appointment.branch_id").
		Joins("LEFT JOIN user_profile uc ON uc.user_id = appointment.created_by")

	// Filter cột theo FE selectedColumns — ?columns=codeCustomer,fullname,...
	// SELECT luôn lấy ALL field (tránh alias mismatch) — chỉ filter ở bước build header/mapper.
	allCols := appointmentColsFor(loc)
	cols := allCols
	if colParam := c.Query("columns"); colParam != "" {
		selected := map[string]bool{}
		for _, k := range strings.Split(colParam, ",") {
			selected[strings.TrimSpace(k)] = true
		}
		var filtered []appointmentCol
		for _, col := range allCols {
			if selected[col.key] {
				filtered = append(filtered, col)
			}
		}
		if len(filtered) > 0 {
			cols = filtered
		}
	}

	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = col.header
	}

	url, err := doExport(c, eq, export.Config{
		FileName:    "Lich_hen",
		Headers:     headers,
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		data := make([]interface{}, len(cols))
		for i, col := range cols {
			data[i] = col.extract(row, showPhone)
		}
		return data
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

// joinAppointmentListDate render cột `appointmentListDate` (lịch sử đổi ngày hẹn) thành 1 cell Excel.
//
// Cột có HAI format cùng tồn tại trong DB:
//   - object  [{"old": <unix>, "new": <unix>}]        — `crm/appointment/service.Update` ghi
//   - chuỗi   ["04-10-2025 -> 05-01-2026"]            — data cũ từ hệ thống PHP
//
// Unmarshal thẳng vào []string fail ở format object và trả rỗng câm lặng, nên phải tách từng
// phần tử rồi thử theo kiểu. Entry không đọc được thì bỏ qua, KHÔNG bỏ cả cột.
func joinAppointmentListDate(v interface{}, loc *time.Location) string {
	var raw []byte
	switch val := v.(type) {
	case []byte:
		raw = val
	case string:
		raw = []byte(val)
	default:
		return ""
	}
	if len(raw) == 0 {
		return ""
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, it := range items {
		var legacy string
		if err := json.Unmarshal(it, &legacy); err == nil {
			if legacy != "" {
				parts = append(parts, legacy)
			}
			continue
		}
		var entry struct {
			Old interface{} `json:"old"`
			New interface{} `json:"new"`
		}
		if err := json.Unmarshal(it, &entry); err != nil {
			continue
		}
		oldStr, newStr := fmtUnixDate(entry.Old, loc), fmtUnixDate(entry.New, loc)
		if oldStr == "" && newStr == "" {
			continue
		}
		parts = append(parts, oldStr+" -> "+newStr)
	}
	return strings.Join(parts, "; ")
}

func fmtUnixDate(v interface{}, loc *time.Location) string {
	if v == nil {
		return ""
	}
	var ts int64
	switch val := v.(type) {
	case float64:
		ts = int64(val)
	case int64:
		ts = val
	case int:
		ts = int64(val)
	default:
		return fmt.Sprintf("%v", v)
	}
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).In(loc).Format("02/01/2006")
}

func fmtUnixDateTime(v interface{}, loc *time.Location) string {
	if v == nil {
		return ""
	}
	var ts int64
	switch val := v.(type) {
	case float64:
		ts = int64(val)
	case int64:
		ts = val
	case int:
		ts = int64(val)
	default:
		return fmt.Sprintf("%v", v)
	}
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).In(loc).Format("02/01/2006 15:04")
}

var reHTML = regexp.MustCompile(`<[^>]*>`)
var reSpaces = regexp.MustCompile(`[\s\x{00a0}]+`)

func stripHTML(v interface{}) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	// replace <br>, <p>, </p> with newline
	s = strings.NewReplacer("</p>", "\n", "<br>", "\n", "<br/>", "\n", "<br />", "\n").Replace(s)
	// remove remaining HTML tags
	s = reHTML.ReplaceAllString(s, "")
	// decode HTML entities
	s = html.UnescapeString(s)
	// normalize whitespace (including &nbsp; which is \u00a0)
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case uint:
		return float64(val)
	case uint64:
		return float64(val)
	case []uint8:
		f, err := strconv.ParseFloat(string(val), 64)
		if err != nil {
			return 0
		}
		return f
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		s := fmt.Sprintf("%v", v)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return f
	}
}

const apptExportLimit = 2000

// ExportAppointmentByhourList GET /api/print/appointment-byhour-list-export
func ExportAppointmentByhourList(c *gin.Context, db *gorm.DB) {
	loc := tz.LocFromContext(c, db)
	day := c.DefaultQuery("appointment_date", time.Now().In(loc).Format("2006-01-02"))
	fromT, toT := tz.DayRange(loc, day)

	var items []map[string]interface{}
	tx := tenant.ScopedDBNullBranch(c, db, "appointment").
		Table("appointment").
		Select(`appointment.appointment_date, appointment.status, appointment.content,
			appointment.branch_id,
			IF(cust.fullname IS NOT NULL AND cust.fullname != '', cust.fullname, cust.nickname) as customer_name,
			b.name as branch_name,
			t.name as appointment_type_name,
			s.name as service_name`).
		Joins("INNER JOIN customer cust ON cust.id = appointment.customer_id").
		Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
		Joins("INNER JOIN appointment_type t ON t.id = appointment.appointment_type_id").
		Joins("LEFT JOIN service s ON s.id = appointment.service_id").
		Where("appointment.appointment_date BETWEEN ? AND ?", fromT, toT).
		Where("appointment.status != 4").
		Order("appointment.branch_id ASC, appointment.appointment_date ASC").
		Limit(apptExportLimit).
		Find(&items)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	if len(items) == apptExportLimit {
		log.Printf("[ExportAppointmentByhourList] hit limit %d for day=%s — data may be truncated",
			apptExportLimit, day)
	}

	dayParsed, _ := time.ParseInLocation("2006-01-02", day, loc)
	buildApptExcel(c, items, loc, apptExcelCfg{
		title:          "DANH SÁCH LỊCH HẸN THEO GIỜ - NGÀY " + dayParsed.Format("02/01/2006"),
		sheetName:      "Lịch hẹn theo giờ",
		groupByIDField: "branch_id",
		groupByName:    "branch_name",
		groupLabel:     "Chi nhánh",
		fileName:       "Lich_hen_theo_gio_" + strings.ReplaceAll(day, "-", ""),
	})
}

type apptExcelCfg struct {
	title          string
	sheetName      string
	groupByIDField string // numeric ID column (group key)
	groupByName    string // display name column
	groupLabel     string
	fileName       string
}

func buildApptExcel(c *gin.Context, items []map[string]interface{}, loc *time.Location, cfg apptExcelCfg) {
	if len(items) == 0 {
		response.Validate(c, i18n.T(c, "export_no_data"))
		return
	}

	// Group by ID (avoid name collisions), display name. Preserve DB order.
	type group struct {
		name string
		rows []map[string]interface{}
	}
	var groups []group
	groupIdx := map[int]int{}
	for _, item := range items {
		id := toInt(item[cfg.groupByIDField])
		if idx, ok := groupIdx[id]; ok {
			groups[idx].rows = append(groups[idx].rows, item)
			continue
		}
		name := strVal(item[cfg.groupByName])
		if name == "" {
			name = "Chưa phân công"
		}
		groupIdx[id] = len(groups)
		groups = append(groups, group{name: name, rows: []map[string]interface{}{item}})
	}

	const numCols = 7
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", cfg.sheetName)

	colDefs := []struct {
		letter string
		width  float64
	}{
		{"A", 5}, {"B", 12}, {"C", 25}, {"D", 22},
		{"E", 25}, {"F", 16}, {"G", 35},
	}
	for _, col := range colDefs {
		_ = f.SetColWidth(cfg.sheetName, col.letter, col.letter, col.width)
	}

	makeBorder := func(color string, style int) []excelize.Border {
		return []excelize.Border{
			{Type: "left", Color: color, Style: style},
			{Type: "right", Color: color, Style: style},
			{Type: "top", Color: color, Style: style},
			{Type: "bottom", Color: color, Style: style},
		}
	}
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"5B2C8D"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 14, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"7B3F9E"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    makeBorder("9B59B6", 1),
	})
	groupStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F3E8FF"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "5B2C8D", Size: 11, Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    makeBorder("C4A8E8", 1),
	})
	totalStyle, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EDE9FE"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "5B2C8D", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "C4A8E8", Style: 1},
			{Type: "right", Color: "C4A8E8", Style: 1},
			{Type: "top", Color: "C4A8E8", Style: 2},
			{Type: "bottom", Color: "C4A8E8", Style: 1},
		},
	})
	makeDataStyle := func(bgColor, hAlign string) int {
		s, _ := f.NewStyle(&excelize.Style{
			Fill:      excelize.Fill{Type: "pattern", Color: []string{bgColor}, Pattern: 1},
			Font:      &excelize.Font{Size: 10, Family: "Calibri"},
			Alignment: &excelize.Alignment{Horizontal: hAlign, Vertical: "center", WrapText: true},
			Border:    makeBorder("E0E0E0", 1),
		})
		return s
	}
	centerStyle, centerAltStyle := makeDataStyle("FFFFFF", "center"), makeDataStyle("F9F5FF", "center")
	dataStyle, dataAltStyle := makeDataStyle("FFFFFF", "left"), makeDataStyle("F9F5FF", "left")

	// Row 1 — Title
	_ = f.SetRowHeight(cfg.sheetName, 1, 34)
	titleStart, _ := excelize.CoordinatesToCellName(1, 1)
	titleEnd, _ := excelize.CoordinatesToCellName(numCols, 1)
	_ = f.MergeCell(cfg.sheetName, titleStart, titleEnd)
	_ = f.SetCellValue(cfg.sheetName, titleStart, cfg.title)
	_ = f.SetCellStyle(cfg.sheetName, titleStart, titleEnd, titleStyle)

	// Row 2 — Headers
	_ = f.SetRowHeight(cfg.sheetName, 2, 22)
	headers := []string{"STT", "Giờ hẹn", "Khách hàng", "Loại lịch hẹn", "Dịch vụ", "Trạng thái", "Ghi chú"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(cfg.sheetName, cell, h)
	}
	hStart, _ := excelize.CoordinatesToCellName(1, 2)
	hEnd, _ := excelize.CoordinatesToCellName(numCols, 2)
	_ = f.SetCellStyle(cfg.sheetName, hStart, hEnd, headerStyle)

	// Data rows
	currentRow := 3
	globalSTT := 1
	for _, grp := range groups {
		_ = f.SetRowHeight(cfg.sheetName, currentRow, 20)
		gStart, _ := excelize.CoordinatesToCellName(1, currentRow)
		gEnd, _ := excelize.CoordinatesToCellName(numCols, currentRow)
		_ = f.MergeCell(cfg.sheetName, gStart, gEnd)
		_ = f.SetCellValue(cfg.sheetName, gStart, "  "+cfg.groupLabel+": "+grp.name)
		_ = f.SetCellStyle(cfg.sheetName, gStart, gEnd, groupStyle)
		currentRow++

		for j, row := range grp.rows {
			_ = f.SetRowHeight(cfg.sheetName, currentRow, 18)
			isAlt := j%2 == 1

			curCenter, curData := centerStyle, dataStyle
			if isAlt {
				curCenter, curData = centerAltStyle, dataAltStyle
			}

			sttCell, _ := excelize.CoordinatesToCellName(1, currentRow)
			_ = f.SetCellValue(cfg.sheetName, sttCell, globalSTT)
			_ = f.SetCellStyle(cfg.sheetName, sttCell, sttCell, curCenter)

			tCell, _ := excelize.CoordinatesToCellName(2, currentRow)
			_ = f.SetCellValue(cfg.sheetName, tCell, fmtUnixHHMM(row["appointment_date"], loc))
			_ = f.SetCellStyle(cfg.sheetName, tCell, tCell, curCenter)

			dataVals := []interface{}{
				strVal(row["customer_name"]),
				strVal(row["appointment_type_name"]),
				strVal(row["service_name"]),
				apptStatusLabel(toInt(row["status"])),
				stripHTML(row["content"]),
			}
			for k, val := range dataVals {
				cell, _ := excelize.CoordinatesToCellName(k+3, currentRow)
				_ = f.SetCellValue(cfg.sheetName, cell, val)
			}
			dStart, _ := excelize.CoordinatesToCellName(3, currentRow)
			dEnd, _ := excelize.CoordinatesToCellName(numCols, currentRow)
			_ = f.SetCellStyle(cfg.sheetName, dStart, dEnd, curData)

			globalSTT++
			currentRow++
		}
	}

	// Total row
	_ = f.SetRowHeight(cfg.sheetName, currentRow, 22)
	tStart, _ := excelize.CoordinatesToCellName(1, currentRow)
	tEnd, _ := excelize.CoordinatesToCellName(numCols, currentRow)
	_ = f.MergeCell(cfg.sheetName, tStart, tEnd)
	_ = f.SetCellValue(cfg.sheetName, tStart, fmt.Sprintf("Tổng cộng: %d lịch hẹn", len(items)))
	_ = f.SetCellStyle(cfg.sheetName, tStart, tEnd, totalStyle)

	url, err := export.SaveWorkbook(f, export.Config{
		FileName:    cfg.fileName,
		ExportDir:   export.DefaultDir,
		DownloadURL: DownloadURL,
		OwnerID:     middleware.GetUserID(c),
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

func fmtUnixDateInLoc(ts int64, loc *time.Location) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).In(loc).Format("02/01/2006")
}

func fmtUnixHHMM(v interface{}, loc *time.Location) string {
	if v == nil {
		return ""
	}
	var ts int64
	switch val := v.(type) {
	case float64:
		ts = int64(val)
	case int64:
		ts = val
	case int:
		ts = int64(val)
	default:
		return ""
	}
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).In(loc).Format("15:04")
}

func apptStatusLabel(status int) string {
	labels := map[int]string{
		0: "Chưa đến", 1: "Đã đến", 2: "Hoàn thành",
		4: "Hủy", 5: "Đang tư vấn", 6: "Đang phục vụ",
	}
	if l, ok := labels[status]; ok {
		return l
	}
	return ""
}

func strVal(v interface{}) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

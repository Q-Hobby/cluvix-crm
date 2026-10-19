package export

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tz"
)

type noteCol struct {
	key     string
	header  string
	extract func(r map[string]interface{}) interface{}
}

// noteColsFor — key khớp 1-1 với FE displayCols[].value (crm/report/note/note-list).
// showPhone (option 20 — CanSeePhoneExport) quyết định mask SĐT khách hàng.
// Note: cột `statusName` (nhãn trạng thái khách hàng) là FE-only phức tạp — enum FE lệch
// hẳn Go define (FE 1=Dừng chăm sóc vs Go 1=Trash) và chưa có label map phía Go → FOLLOW-UP,
// tạm bỏ khỏi colDef để tránh nhãn sai.
func noteColsFor(loc *time.Location, showPhone bool) []noteCol {
	return []noteCol{
		{"id", "ID", func(r map[string]interface{}) interface{} { return r["id"] }},
		{"code", "Mã khách hàng", func(r map[string]interface{}) interface{} { return r["customer_code"] }},
		{"fullname", "Khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }},
		{"phone", "Số điện thoại", func(r map[string]interface{}) interface{} { return maskPhoneExport(showPhone, r["customer_phone"]) }},
		{"note", "Nội dung ghi chú", func(r map[string]interface{}) interface{} { return stripHTML(r["note"]) }},
		{"created_by", "Người tạo", func(r map[string]interface{}) interface{} { return r["created_by_fullname"] }},
		{"created_at", "Ngày tạo", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["created_at"], loc) }},
	}
}

func ExportNote(c *gin.Context, q *gorm.DB) {
	loc := tz.LocFromContext(c, q.Session(&gorm.Session{NewDB: true}))
	showPhone := phoneutil.CanSeePhoneExport(c, q.Session(&gorm.Session{NewDB: true}))
	eq := q.Session(&gorm.Session{}).
		Select(`note.id, note.note, note.created_at,
			uc.fullname as created_by_fullname,
			cust.code as customer_code, cust.fullname as customer_fullname, cust.phone as customer_phone`).
		Joins("LEFT JOIN user_profile uc ON uc.user_id = note.created_by").
		Joins("LEFT JOIN customer cust ON cust.id = note.customer_id")

	allCols := noteColsFor(loc, showPhone)
	cols := allCols
	if colParam := c.Query("columns"); colParam != "" {
		selected := map[string]bool{}
		for _, k := range strings.Split(colParam, ",") {
			selected[strings.TrimSpace(k)] = true
		}
		var filtered []noteCol
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
		FileName:    "Ghi-chu_export",
		Headers:     headers,
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		data := make([]interface{}, len(cols))
		for i, col := range cols {
			data[i] = col.extract(row)
		}
		return data
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

func ExportServiceType(c *gin.Context, q *gorm.DB) {
	loc := tz.LocFromContext(c, q.Session(&gorm.Session{NewDB: true}))
	eq := q.Session(&gorm.Session{}).
		Select(`service_type.id, service_type.name, service_type.status, service_type.SortOrder, service_type.created_at,
			uc.fullname as user_created_fullname`).
		Joins("LEFT JOIN user_profile uc ON uc.user_id = service_type.created_by")

	url, err := doExport(c, eq, export.Config{
		FileName: "Nhom_dich_vu",
		Headers: []string{
			"id", "Nhóm dịch vụ", "Trạng thái", "Vị trí", "Người tạo", "Ngày tạo", "Trạng thái (text)",
		},
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		status := toInt(row["status"])
		statusText := "Ẩn"
		if status == 1 {
			statusText = "Hiển thị"
		}
		return []interface{}{
			row["id"], row["name"], status, row["SortOrder"],
			row["user_created_fullname"],
			fmtUnixDateTime(row["created_at"], loc), statusText,
		}
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

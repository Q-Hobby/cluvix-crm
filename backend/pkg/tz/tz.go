package tz

import (
	"fmt"
	"time"

	"erpCluvix/pkg/brand"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const DefaultTZ = "Asia/Ho_Chi_Minh"

// Story-40 (E6/D15 — 1 thương hiệu/DB): mọi hàm tz KHÔNG còn nhận companyID. Múi giờ có ĐÚNG MỘT
// nguồn là `pkg/brand` (cache + quy tắc "lỗi DB KHÔNG cache" — bài học story-09 — nay nằm ở đó, có
// test `pkg/brand` TestLoiDB_KhongCache). Cache/cảnh báo riêng theo company của package này đã bỏ.

// Location trả múi giờ của thương hiệu (mặc định Asia/Ho_Chi_Minh).
func Location(db *gorm.DB) *time.Location {
	return brand.Location(db)
}

// InvalidateAll xoá cache múi giờ (cache `pkg/brand`) — gọi sau khi sửa hồ sơ thương hiệu.
func InvalidateAll() {
	brand.Invalidate()
}

// Now returns current time in the brand timezone.
func Now(db *gorm.DB) time.Time {
	return time.Now().In(Location(db))
}

// MonthStart returns first second of a specific year/month in the brand timezone.
func MonthStart(db *gorm.DB, year, month int) int64 {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, Location(db)).Unix()
}

// MonthEnd returns last second of a specific year/month in the brand timezone.
func MonthEnd(db *gorm.DB, year, month int) int64 {
	return time.Date(year, time.Month(month+1), 0, 23, 59, 59, 0, Location(db)).Unix()
}

// Format formats a Unix timestamp in the brand timezone.
func Format(db *gorm.DB, unix int64, layout string) string {
	return time.Unix(unix, 0).In(Location(db)).Format(layout)
}

// YearMonth returns YYMM string for current time in the brand timezone.
func YearMonth(db *gorm.DB) string {
	return Now(db).Format("0601")
}

// LocFromContext trả về *time.Location của thương hiệu — dùng cho mọi handler cần tính time range.
//
// Story-35 (D15 — 1 thương hiệu/DB): KHÔNG còn đọc company từ session; múi giờ lấy từ
// `pkg/brand` (nguồn duy nhất). Giữ chữ ký (và tham số `c`) để caller không phải sửa.
func LocFromContext(_ *gin.Context, db *gorm.DB) *time.Location {
	return brand.Location(db)
}

// LocalDate trả ngày (hôm nay + days) theo timezone của loc, dạng `YYYY-MM-DD` — dùng để so với
// cột kiểu DATE (vd `order_installment.due_date`). KHÔNG dùng `CURDATE()` trong SQL: đó là
// timezone session MySQL, lệch ngày với company quanh nửa đêm.
func LocalDate(loc *time.Location, days int) string {
	if loc == nil {
		loc = mustLoad(DefaultTZ)
	}
	return time.Now().In(loc).AddDate(0, 0, days).Format(time.DateOnly)
}

// StartOfDay trả về Unix timestamp đầu ngày (00:00:00) theo timezone của loc.
func StartOfDay(loc *time.Location) int64 {
	if loc == nil {
		loc = mustLoad(DefaultTZ)
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Unix()
}

// StartOfMonth trả về Unix timestamp đầu tháng (ngày 1, 00:00:00) theo loc.
// offset: 0 = tháng hiện tại, -1 = tháng trước, 1 = tháng sau.
func StartOfMonth(loc *time.Location, offset int) int64 {
	if loc == nil {
		loc = mustLoad(DefaultTZ)
	}
	now := time.Now().In(loc)
	y, m, _ := now.Date()
	return time.Date(y, m+time.Month(offset), 1, 0, 0, 0, 0, loc).Unix()
}

// EndOfMonth trả về Unix timestamp cuối tháng (23:59:59 ngày cuối) theo loc.
func EndOfMonth(loc *time.Location, offset int) int64 {
	return StartOfMonth(loc, offset+1) - 1
}

// MonthRange trả về (start, end) của tháng theo loc.
func MonthRange(loc *time.Location, offset int) (start, end int64) {
	return StartOfMonth(loc, offset), EndOfMonth(loc, offset)
}

// DayRange trả về (start, end) Unix timestamp của 1 ngày theo timezone loc.
// dateStr format: "2006-01-02". end = 23:59:59 của ngày đó.
func DayRange(loc *time.Location, dateStr string) (start, end int64) {
	if loc == nil {
		loc = mustLoad(DefaultTZ)
	}
	t, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return 0, 0
	}
	start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).Unix()
	end = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, loc).Unix()
	return
}

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		loc, _ = time.LoadLocation(DefaultTZ)
	}
	return loc
}

// Offset trả về chuỗi MySQL TZ offset như "+07:00" từ loc — dùng cho CONVERT_TZ.
func Offset(loc *time.Location) string {
	if loc == nil {
		loc = mustLoad(DefaultTZ)
	}
	_, sec := time.Now().In(loc).Zone()
	h := sec / 3600
	m := (sec % 3600) / 60
	if m < 0 {
		m = -m
	}
	return fmt.Sprintf("%+03d:%02d", h, m)
}

// OffsetSec trả về số giây lệch giữa company TZ và UTC — dùng cho FLOOR((unix + offsetSec) / 86400).
func OffsetSec(loc *time.Location) int {
	if loc == nil {
		loc = mustLoad(DefaultTZ)
	}
	_, sec := time.Now().In(loc).Zone()
	return sec
}

// SqlFromUnix trả về SQL fragment chuyển unix timestamp sang DATETIME đúng TZ company,
// để dùng tiếp với DAY/MONTH/DATE/DATE_FORMAT mà không bị lệch ngày khi MySQL session TZ
// khác company TZ. col là tên column unix timestamp (vd "a.appointment_date").
//
// Khác biệt với plain FROM_UNIXTIME: pattern này dùng @@session.time_zone làm source,
// chuyển sang offset string của company. Trả về kết quả vẫn là MySQL DATETIME.
//
// Usage:
//
//	tz.SqlFromUnix("a.appointment_date", tz.Offset(loc))
//	→ "CONVERT_TZ(FROM_UNIXTIME(a.appointment_date), @@session.time_zone, '+07:00')"
func SqlFromUnix(col, offset string) string {
	return fmt.Sprintf("CONVERT_TZ(FROM_UNIXTIME(%s), @@session.time_zone, '%s')", col, offset)
}

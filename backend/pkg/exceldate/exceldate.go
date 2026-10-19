// Package exceldate parse giá trị ngày/ngày-giờ khi import từ Excel — dùng chung
// cho mọi handler import có cột date, thay cho các hàm parse riêng lẻ từng module.
//
// Vì sao cần dùng chung:
//   - Handler PHẢI đọc file bằng f.GetRows(sheet, excelize.Options{RawCellValue: true}):
//     Excel tự nhận ô như "6/3/1990" (ngày ≤ 12) là date → lưu serial; GetRows mặc định
//     render lại theo locale ("06-03-90") khiến parse fail và bị bỏ âm thầm. Đọc raw →
//     ô date trả serial number, độc lập locale.
//   - Go time.Parse("02/01/2006", ...) bắt buộc zero-pad → fail với ngày/tháng 1 chữ số.
//     Package này tách tay, chấp nhận không zero-pad + nhiều separator.
package exceldate

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// SerialMin: số nguyên dưới ngưỡng này ở cột ngày coi là năm gõ nhầm cột
	// (1900–2100) hoặc rác, KHÔNG đọc thành serial (tránh lưu ngày rác quanh 1905).
	// Serial thật của ngày sinh (sinh từ ~1908) đều ≥ 3000; thực tế đều ≥ ~10000 (1927+).
	SerialMin = 3000
	// SerialMax: số ≥ ngưỡng này (year > ~3269) coi là Unix timestamp (giây), không phải serial.
	SerialMax = 500000
)

var reDateSep = regexp.MustCompile(`\s*[/.\-]\s*`)

// SerialToTime đổi serial number Excel (số ngày kể từ epoch 1899-12-30, phần thập
// phân = phần trong ngày) sang time.Time tại loc. loc nil → UTC.
func SerialToTime(serial float64, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	days := int(serial)
	frac := serial - float64(days)
	t := time.Date(1899, 12, 30, 0, 0, 0, 0, loc).AddDate(0, 0, days)
	return t.Add(time.Duration(frac * 24 * float64(time.Hour)))
}

// ParseDate parse ngày (không giờ) từ giá trị cell Excel đọc raw:
//   - text: separator "/", "-", "."; không bắt zero-pad; thứ tự DD-MM-YYYY (chuẩn VN)
//     hoặc YYYY-MM-DD (khi phần đầu 4 chữ số);
//   - serial number Excel (ô kiểu Date), SerialMin ≤ n < SerialMax.
//
// Trả time.Time lúc 00:00 tại loc (loc nil → UTC). ok=false nếu rỗng/rác/năm 2 chữ số/
// năm gõ nhầm cột/ngày không hợp lệ (vd 31/02).
func ParseDate(s string, loc *time.Location) (time.Time, bool) {
	if loc == nil {
		loc = time.UTC
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// Dạng text có separator
	if parts := reDateSep.Split(s, -1); len(parts) == 3 {
		a, e1 := strconv.Atoi(parts[0])
		b, e2 := strconv.Atoi(parts[1])
		c, e3 := strconv.Atoi(parts[2])
		if e1 == nil && e2 == nil && e3 == nil {
			var year, month, day int
			if len(parts[0]) == 4 { // YYYY-MM-DD
				year, month, day = a, b, c
			} else { // DD-MM-YYYY
				day, month, year = a, b, c
			}
			if year >= 1900 && year <= 3000 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
				t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
				// loại ngày tràn (vd 31/02 → 03/03): so lại sau khi normalize
				if t.Year() == year && int(t.Month()) == month && t.Day() == day {
					return t, true
				}
			}
		}
		return time.Time{}, false
	}
	// Dạng serial number Excel (ô kiểu Date)
	if serial, err := strconv.ParseFloat(s, 64); err == nil && serial >= SerialMin && serial < SerialMax {
		return SerialToTime(serial, loc), true
	}
	return time.Time{}, false
}

// ParseDateString như ParseDate nhưng ở UTC và trả chuỗi "2006-01-02".
// Dùng khi lưu ngày dạng string (vd cột birthday) — timezone không ảnh hưởng chuỗi ngày.
func ParseDateString(s string) (string, bool) {
	t, ok := ParseDate(s, time.UTC)
	if !ok {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

// ParseDateTime parse ngày-giờ từ giá trị cell Excel đọc raw:
//   - "DD/MM/YYYY HH:MM:SS";
//   - mọi dạng của ParseDate (ngày không giờ);
//   - serial Excel có phần giờ (phần thập phân);
//   - Unix timestamp (số ≥ SerialMax).
//
// Trả time.Time tại loc (loc nil → UTC). Dùng cho cột như created_day (lưu qua .Unix()).
func ParseDateTime(s string, loc *time.Location) (time.Time, bool) {
	if loc == nil {
		loc = time.UTC
	}
	s = strings.TrimSpace(s)
	if t, err := time.ParseInLocation("02/01/2006 15:04:05", s, loc); err == nil {
		return t, true
	}
	if t, ok := ParseDate(s, loc); ok {
		return t, true
	}
	// Numeric: Excel serial (nhỏ) vs Unix timestamp (lớn)
	if fval, err := strconv.ParseFloat(s, 64); err == nil {
		if fval >= SerialMin && fval < SerialMax {
			return SerialToTime(fval, loc), true
		}
		if fval >= SerialMax {
			return time.Unix(int64(fval), 0).In(loc), true
		}
	}
	return time.Time{}, false
}

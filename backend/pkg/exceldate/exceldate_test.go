package exceldate

import (
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestParseDateString(t *testing.T) {
	cases := []struct {
		in, want string // want="" => phải drop (ok=false)
	}{
		// text — không zero-pad, nhiều separator, cả 2 thứ tự
		{"19/12/1995", "1995-12-19"},
		{"6/3/1990", "1990-03-06"},
		{"19/3/1990", "1990-03-19"},
		{"06/03/1990", "1990-03-06"},
		{"1990-3-6", "1990-03-06"},
		{"1990/03/06", "1990-03-06"},
		{"06-03-1990", "1990-03-06"},
		{"6.3.1990", "1990-03-06"},
		{"5/12/2999", "2999-12-05"},
		{" 6 / 3 / 1990 ", "1990-03-06"},
		// serial Excel (ô Date đọc raw)
		{"32938", "1990-03-06"},
		{"32938.5", "1990-03-06"},
		{"45000", "2023-03-15"},
		// phải drop
		{"1990", ""},       // năm gõ nhầm cột → không thành serial rác 1905
		{"6/3/90", ""},     // năm 2 chữ số mơ hồ
		{"31/02/1990", ""}, // ngày tràn
		{"13/13/1990", ""}, // tháng 13
		{"n/a", ""},
		{"", ""},
	}
	for _, c := range cases {
		got, ok := ParseDateString(c.in)
		if c.want == "" {
			if ok {
				t.Errorf("%q: muốn DROP, nhận %q", c.in, got)
			}
			continue
		}
		if !ok || got != c.want {
			t.Errorf("%q: muốn %q, nhận %q (ok=%v)", c.in, c.want, got, ok)
		}
	}
}

func TestParseDateTime(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		in      string
		wantOK  bool
		wantStr string // format "2006-01-02 15:04:05" nếu wantOK
	}{
		{"27/06/2024 10:30:00", true, "2024-06-27 10:30:00"},
		{"27/06/2024", true, "2024-06-27 00:00:00"},
		{"6/3/2024", true, "2024-03-06 00:00:00"},
		{"2024-06-27", true, "2024-06-27 00:00:00"},
		{"1700000000", true, "2023-11-14 22:13:20"}, // Unix timestamp (UTC)
		{"45469", true, "2024-06-26 00:00:00"},       // Excel serial
		{"abc", false, ""},
		{"500", false, ""}, // quá nhỏ → drop
	}
	for _, c := range cases {
		got, ok := ParseDateTime(c.in, loc)
		if ok != c.wantOK {
			t.Errorf("%q: ok=%v muốn %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && got.Format("2006-01-02 15:04:05") != c.wantStr {
			t.Errorf("%q: nhận %s muốn %s", c.in, got.Format("2006-01-02 15:04:05"), c.wantStr)
		}
	}
}

// TestExcelizeRoundTrip mô phỏng file người dùng: ngày ≤ 12 bị Excel auto-convert
// thành date cell; đọc RawCellValue → serial → ParseDateString ra đúng ngày.
func TestExcelizeRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	sheet := "Sheet1"
	styleDate, _ := f.NewStyle(&excelize.Style{NumFmt: 14})
	dates := []time.Time{
		time.Date(1990, 3, 6, 0, 0, 0, 0, time.UTC),
		time.Date(1992, 5, 12, 0, 0, 0, 0, time.UTC),
	}
	for i, d := range dates {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		f.SetCellValue(sheet, cell, d)
		f.SetCellStyle(sheet, cell, cell, styleDate)
	}
	rows, _ := f.GetRows(sheet, excelize.Options{RawCellValue: true})
	want := []string{"1990-03-06", "1992-05-12"}
	for i := range dates {
		got, ok := ParseDateString(rows[i][0])
		if !ok || got != want[i] {
			t.Errorf("row%d raw=%q -> %q(ok=%v), muốn %s", i, rows[i][0], got, ok, want[i])
		}
	}
}

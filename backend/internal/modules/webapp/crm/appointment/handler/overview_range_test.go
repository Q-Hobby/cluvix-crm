package handler

import (
	"testing"
	"time"
)

func TestAppointmentOverviewRange(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	at := func(y int, m time.Month, d int) int64 { return time.Date(y, m, d, 0, 0, 0, 0, loc).Unix() }
	// "Bây giờ" = 20/10/2026 01:00 giờ VN (= 19/10 18:00 UTC): năm hiện tại tính theo giờ thương hiệu.
	now := time.Date(2026, 10, 19, 18, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		year        int
		wantStart   int64
		wantEndExcl int64
	}{
		{"năm hiện tại ⇒ tới hết tháng hiện tại", 2026, at(2026, 1, 1), at(2026, 11, 1)},
		{"năm đã qua ⇒ trọn năm (gồm cả 31/12)", 2025, at(2025, 1, 1), at(2026, 1, 1)},
	}
	for _, tc := range cases {
		start, endExcl, months := appointmentOverviewRange(loc, tc.year, now)
		if start != tc.wantStart || endExcl != tc.wantEndExcl {
			t.Fatalf("%s: got [%d,%d) want [%d,%d)", tc.name, start, endExcl, tc.wantStart, tc.wantEndExcl)
		}
		if len(months) != 11 || months[0] != at(tc.year, 2, 1) || months[10] != at(tc.year, 12, 1) {
			t.Fatalf("%s: mốc tháng sai: %v", tc.name, months)
		}
	}
	// Rìa năm theo giờ thương hiệu: 31/12/2026 18:00 UTC = 01/01/2027 01:00 VN ⇒ năm hiện tại là 2027.
	start, endExcl, _ := appointmentOverviewRange(loc, 2027, time.Date(2026, 12, 31, 18, 0, 0, 0, time.UTC))
	if start != at(2027, 1, 1) || endExcl != at(2027, 2, 1) {
		t.Fatalf("rìa năm: got [%d,%d)", start, endExcl)
	}
}

package handler

import (
	"strings"
	"testing"
)

// buildHourTimeSlots phải NỚI khung 7:00–20:30 đủ chứa mọi slot có trong dữ liệu.
// Trước đây khung bị hardcode 7–20 nên pivot bỏ qua im lặng lịch hẹn ngoài khung
// (đo trên DB dev 2026-09-06: 25/160.253 lịch hẹn năm 2026 nằm ngoài 7h–20h; riêng
// ngày 20627 có 6 lịch hẹn lúc 22h/23h bị mất khỏi bảng).
func TestBuildHourTimeSlots(t *testing.T) {
	row := func(slot string) map[string]interface{} {
		return map[string]interface{}{"time_slot": slot}
	}

	cases := []struct {
		name      string
		rows      []map[string]interface{}
		first     string
		last      string
		mustHave  []string
		mustCount int
	}{
		{
			name:      "không có dòng nào → giữ nguyên khung mặc định",
			rows:      nil,
			first:     "7:00",
			last:      "20:30",
			mustCount: 28,
		},
		{
			name:      "toàn bộ trong khung → không nới",
			rows:      []map[string]interface{}{row("7:00"), row("13:30"), row("20:30")},
			first:     "7:00",
			last:      "20:30",
			mustCount: 28,
		},
		{
			name:      "có lịch lúc 1:30 → nới đầu khung",
			rows:      []map[string]interface{}{row("1:30"), row("9:00")},
			first:     "1:00",
			last:      "20:30",
			mustHave:  []string{"1:30"},
			mustCount: 40,
		},
		{
			name:      "có lịch lúc 22h/23h → nới cuối khung",
			rows:      []map[string]interface{}{row("22:00"), row("23:30")},
			first:     "7:00",
			last:      "23:30",
			mustHave:  []string{"22:00", "23:30"},
			mustCount: 34,
		},
		{
			name:      "nới cả hai đầu",
			rows:      []map[string]interface{}{row("0:00"), row("23:00")},
			first:     "0:00",
			last:      "23:30",
			mustCount: 48,
		},
		{
			name:      "dòng rác không parse được thì bỏ qua, không nới bừa",
			rows:      []map[string]interface{}{row(""), row("abc"), {"time_slot": nil}, row("99:00")},
			first:     "7:00",
			last:      "20:30",
			mustCount: 28,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildHourTimeSlots(tc.rows)
			if len(got) != tc.mustCount {
				t.Fatalf("số slot = %d, muốn %d (%s)", len(got), tc.mustCount, strings.Join(got, ","))
			}
			if got[0] != tc.first {
				t.Errorf("slot đầu = %q, muốn %q", got[0], tc.first)
			}
			if got[len(got)-1] != tc.last {
				t.Errorf("slot cuối = %q, muốn %q", got[len(got)-1], tc.last)
			}
			for _, want := range tc.mustHave {
				found := false
				for _, s := range got {
					if s == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("thiếu slot %q — dòng dữ liệu ở giờ này sẽ bị pivot bỏ im lặng", want)
				}
			}
		})
	}
}

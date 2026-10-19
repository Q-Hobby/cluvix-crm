package export

import (
	"testing"
	"time"
)

// Cột "Ngày hẹn đã đổi" phải đọc được CẢ HAI format cùng tồn tại trong DB:
// object {old,new} unix (service.Update ghi) và chuỗi cũ từ hệ thống PHP.
func TestJoinAppointmentListDate(t *testing.T) {
	// 1759510800 = 2025-10-04 00:00 +07 · 1767546000 = 2026-01-05 00:00 +07
	loc := time.FixedZone("ICT", 7*3600)

	cases := []struct {
		name string
		in   interface{}
		want string
	}{
		{"object format", []byte(`[{"old":1759510800,"new":1767546000}]`), "04/10/2025 -> 05/01/2026"},
		{"legacy string format", []byte(`["04-10-2025 -> 05-01-2026"]`), "04-10-2025 -> 05-01-2026"},
		{"lẫn 2 format", []byte(`["04-10-2025 -> 05-01-2026",{"old":1759510800,"new":1767546000}]`),
			"04-10-2025 -> 05-01-2026; 04/10/2025 -> 05/01/2026"},
		{"nhiều entry object", []byte(`[{"old":1759510800,"new":1767546000},{"old":1767546000,"new":1759510800}]`),
			"04/10/2025 -> 05/01/2026; 05/01/2026 -> 04/10/2025"},
		{"string input (driver trả string)", `[{"old":1759510800,"new":1767546000}]`, "04/10/2025 -> 05/01/2026"},
		{"entry rác bị bỏ qua, không mất cả cột", []byte(`[123,{"old":1759510800,"new":1767546000}]`), "04/10/2025 -> 05/01/2026"},
		{"mảng rỗng", []byte(`[]`), ""},
		{"NULL", nil, ""},
		{"JSON hỏng", []byte(`{oops`), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinAppointmentListDate(tc.in, loc); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

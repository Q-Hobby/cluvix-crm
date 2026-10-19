package helpers

import "testing"

func TestIPInAllowlist(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		list []string
		want bool
	}{
		{"exact match", "203.0.113.5", []string{"203.0.113.5"}, true},
		{"exact no match", "203.0.113.6", []string{"203.0.113.5"}, false},
		{"cidr match", "203.0.113.42", []string{"203.0.113.0/24"}, true},
		{"cidr no match", "203.0.114.1", []string{"203.0.113.0/24"}, false},
		{"mixed list hits cidr", "10.0.0.9", []string{"1.2.3.4", "10.0.0.0/8"}, true},
		{"empty list", "10.0.0.9", nil, false},
		{"whitespace + empty entries", "10.0.0.9", []string{" ", "", "10.0.0.9 "}, true},
		{"ipv6 exact normalized", "2001:db8::1", []string{"2001:0db8:0000:0000:0000:0000:0000:0001"}, true},
		{"ipv6 cidr", "2001:db8::abcd", []string{"2001:db8::/32"}, true},
		{"invalid cidr ignored", "10.0.0.1", []string{"not-a-cidr/xx", "10.0.0.1"}, true},
		{"invalid ip input", "garbage", []string{"10.0.0.0/8"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IPInAllowlist(c.ip, c.list); got != c.want {
				t.Fatalf("IPInAllowlist(%q, %v) = %v, want %v", c.ip, c.list, got, c.want)
			}
		})
	}
}

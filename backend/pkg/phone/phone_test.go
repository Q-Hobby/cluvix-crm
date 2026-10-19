package phone_test

import (
	"testing"

	"erpCluvix/pkg/phone"

	"github.com/stretchr/testify/assert"
)

func TestIsValidMobile(t *testing.T) {
	valid := []string{
		// local format
		"0912345678", // Vinaphone 091
		"0332345678", // Viettel 033
		"0762345678", // Mobifone 076
		"0812345678", // Vinaphone 081
		"0562345678", // Vietnamobile 056
		"0592345678", // Gmobile 059
		"0552345678", // Reddi 055
		"0702345678", // Mobifone 070
		"0982345678", // Viettel 098
		"0992345678", // Gmobile 099
		// E.164 with +
		"+84912345678",
		"+84332345678",
		// E.164 without +
		"84912345678",
		"84762345678",
		// whitespace trimmed
		"  0912345678  ",
	}
	for _, p := range valid {
		assert.True(t, phone.IsValidMobile(p), "should be valid: %q", p)
	}

	invalid := []string{
		// old 01x numbers (deactivated Nov 2018)
		"0123456789",
		"0162345678",
		// not 10 digits
		"091234567",  // 9 digits
		"09123456789", // 11 digits
		// wrong country code
		"+1234567890",
		// empty
		"",
		// landline (not mobile)
		"0243456789", // Hanoi landline — 024 not a mobile prefix
		// random string
		"abcdefghij",
		// 80 not allocated
		"0801234567",
		// 071-075 not allocated
		"0711234567",
	}
	for _, p := range invalid {
		assert.False(t, phone.IsValidMobile(p), "should be invalid: %q", p)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantOK  bool
	}{
		{"0912345678", "84912345678", true},
		{"+84912345678", "84912345678", true},
		{"84912345678", "84912345678", true},
		{"0332345678", "84332345678", true},
		{"  0912345678  ", "84912345678", true},
		// invalid → ("", false)
		{"0123456789", "", false},
		{"", "", false},
		{"abcdefghij", "", false},
		{"091234567", "", false}, // 9 digits
	}
	for _, tc := range cases {
		got, ok := phone.Normalize(tc.input)
		assert.Equal(t, tc.wantOK, ok, "Normalize(%q) ok", tc.input)
		assert.Equal(t, tc.want, got, "Normalize(%q) result", tc.input)
	}
}

func TestNormalizeMust(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"0912345678", "84912345678"},
		{"+84912345678", "84912345678"},
		{"84912345678", "84912345678"},
		{"0332345678", "84332345678"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, phone.NormalizeMust(tc.input), "NormalizeMust(%q)", tc.input)
	}
}

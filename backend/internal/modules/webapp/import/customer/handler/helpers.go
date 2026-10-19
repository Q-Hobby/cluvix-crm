package handler

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultCountryID = 237
)

var reDigitsOnly = regexp.MustCompile(`^\d+$`)

func stripThousands(s string) string {
	return strings.ReplaceAll(s, ",", "")
}

func formatVND(s string) string {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return s
	}
	str := strconv.FormatInt(n, 10)
	result := []byte{}
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}
	return string(result) + " ₫"
}

func allEmpty(row []string) bool {
	for _, v := range row {
		if v != "" {
			return false
		}
	}
	return true
}

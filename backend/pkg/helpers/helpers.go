package helpers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"

	"erpCluvix/pkg/brand"
)

// randomStringFrom builds a length-N string by uniformly sampling chars.
// Dùng crypto/rand vì RandomString được dùng để generate password (xem
// hr/user.go ChangePassword + GenerateMissingPasswords) — math/rand sẽ
// predictable theo seed time.
func randomStringFrom(chars string, length int) string {
	b := make([]byte, length)
	max := big.NewInt(int64(len(chars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand failure cực hiếm (entropy pool exhausted) — fall back
			// vào timestamp byte để không panic. Caller check length sau cùng.
			b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
			continue
		}
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// RandomString generates random lowercase alphanumeric string (crypto/rand-backed).
func RandomString(length int) string {
	return randomStringFrom("abcdefghijklmnopqrstuvwxyz0123456789", length)
}

// RandomStringMixed generates random mixed case alphanumeric string (crypto/rand-backed).
func RandomStringMixed(length int) string {
	return randomStringFrom("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", length)
}

// FormatVND formats number as Vietnamese currency: 1500000 → "1.500.000 ₫"
func FormatVND(amount int64) string {
	return FormatCurrency(float64(amount), "vi-VN", "₫")
}

// FormatAmount format số tiền theo currency_locale + currency_symbol của hồ sơ doanh nghiệp
// (`pkg/brand`, bảng `business_profile`). Ví dụ: FormatAmount(db, 1500000) → "1.500.000 ₫" (vi-VN).
// Sửa hồ sơ ⇒ caller gọi `brand.Invalidate()`.
func FormatAmount(db *gorm.DB, amount float64) string {
	p := brand.Profile(db)
	return FormatCurrency(amount, p.CurrencyLocale, p.CurrencySymbol)
}

// FormatCurrency formats amount theo locale và symbol của từng quốc gia.
// locale quyết định dấu phân cách: vi-VN/de-DE/... dùng "." ngăn và "," thập phân;
// en-US/ja-JP/ko-KR/... dùng "," ngăn và "." thập phân.
// Ví dụ: FormatCurrency(1500000, "vi-VN", "₫") → "1.500.000 ₫"
//
//	FormatCurrency(1500000, "en-US", "$")  → "$ 1,500,000"
//	FormatCurrency(1500000, "ja-JP", "¥")  → "¥ 1,500,000"
func FormatCurrency(amount float64, locale string, symbol string) string {
	// Xác định dấu phân cách theo locale
	thousandSep, decimalSep := ",", "."
	switch locale {
	case "vi-VN", "de-DE", "fr-FR", "nl-NL", "es-ES", "pt-BR", "id-ID":
		thousandSep, decimalSep = ".", ","
	}

	// Tách phần nguyên và thập phân
	neg := amount < 0
	if neg {
		amount = -amount
	}
	intPart := int64(amount)
	decPart := amount - float64(intPart)

	// Format phần nguyên với dấu ngăn ngàn
	s := fmt.Sprintf("%d", intPart)
	n := len(s)
	var parts []string
	for n > 3 {
		parts = append([]string{s[n-3 : n]}, parts...)
		n -= 3
	}
	parts = append([]string{s[:n]}, parts...)
	result := strings.Join(parts, thousandSep)

	// Thêm phần thập phân nếu có (làm tròn 2 chữ số)
	if decPart > 0.001 {
		result += decimalSep + fmt.Sprintf("%02.0f", decPart*100)
	}

	if neg {
		result = "-" + result
	}

	// Symbol đặt sau với vi-VN/de-DE, đặt trước với các locale khác
	switch locale {
	case "vi-VN", "de-DE", "fr-FR", "nl-NL":
		return result + " " + symbol
	default:
		return symbol + " " + result
	}
}

// ConvertToTitleCase converts string to title case (Vietnamese-safe)
func ConvertToTitleCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	return cases.Title(language.Vietnamese).String(strings.ToLower(s))
}

// CreateAlias creates URL-friendly slug from Vietnamese string
func CreateAlias(s string) string {
	s = strings.TrimSpace(s)
	s = RemoveVietnameseDiacritics(s)
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9\s-]`)
	s = reg.ReplaceAllString(s, " ")
	reg = regexp.MustCompile(`\s+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// SmsKhongDau removes Vietnamese diacritics but keeps original case and spaces
func SmsKhongDau(s string) string {
	return RemoveVietnameseDiacritics(s)
}

// RemoveVietnameseDiacritics removes all Vietnamese diacritics
func RemoveVietnameseDiacritics(s string) string {
	r := strings.NewReplacer(
		"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a",
		"â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a",
		"ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
		"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e",
		"ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
		"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
		"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o",
		"ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o",
		"ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
		"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u",
		"ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
		"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y",
		"đ", "d",
		"À", "A", "Á", "A", "Ạ", "A", "Ả", "A", "Ã", "A",
		"Â", "A", "Ầ", "A", "Ấ", "A", "Ậ", "A", "Ẩ", "A", "Ẫ", "A",
		"Ă", "A", "Ằ", "A", "Ắ", "A", "Ặ", "A", "Ẳ", "A", "Ẵ", "A",
		"È", "E", "É", "E", "Ẹ", "E", "Ẻ", "E", "Ẽ", "E",
		"Ê", "E", "Ề", "E", "Ế", "E", "Ệ", "E", "Ể", "E", "Ễ", "E",
		"Ì", "I", "Í", "I", "Ị", "I", "Ỉ", "I", "Ĩ", "I",
		"Ò", "O", "Ó", "O", "Ọ", "O", "Ỏ", "O", "Õ", "O",
		"Ô", "O", "Ồ", "O", "Ố", "O", "Ộ", "O", "Ổ", "O", "Ỗ", "O",
		"Ơ", "O", "Ờ", "O", "Ớ", "O", "Ợ", "O", "Ở", "O", "Ỡ", "O",
		"Ù", "U", "Ú", "U", "Ụ", "U", "Ủ", "U", "Ũ", "U",
		"Ư", "U", "Ừ", "U", "Ứ", "U", "Ự", "U", "Ử", "U", "Ữ", "U",
		"Ỳ", "Y", "Ý", "Y", "Ỵ", "Y", "Ỷ", "Y", "Ỹ", "Y",
		"Đ", "D",
	)
	return r.Replace(s)
}

// IsValidPhoneNumber checks if string contains only digits and spaces
func IsValidPhoneNumber(s string) bool {
	matched, _ := regexp.MatchString(`^[0-9\s]+$`, s)
	return matched
}

// FormatPhoneNumber formats phone number for display
func FormatPhoneNumber(phone string) string {
	reg := regexp.MustCompile(`[^0-9]`)
	phone = reg.ReplaceAllString(phone, "")
	switch len(phone) {
	case 11:
		return fmt.Sprintf("(+%s) %s %s %s", phone[:2], phone[2:4], phone[4:7], phone[7:])
	case 10:
		return fmt.Sprintf("(%s) %s %s", phone[:3], phone[3:6], phone[6:])
	case 9:
		return fmt.Sprintf("(+84) %s %s %s", phone[:3], phone[3:6], phone[6:])
	default:
		return phone
	}
}

// GetUserNamesFromList extracts user names from JSON ID list using a map
func GetUserNamesFromList(jsonIDs string, userMap map[int]string) string {
	if jsonIDs == "" || jsonIDs == "null" {
		return ""
	}
	// Simple parse: "[1, 2, 3]"
	jsonIDs = strings.Trim(jsonIDs, "[]")
	parts := strings.Split(jsonIDs, ",")
	var names []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var id int
		fmt.Sscanf(p, "%d", &id)
		if name, ok := userMap[id]; ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// StringInSlice returns true if target is an exact element of s.
func StringInSlice(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// ContainsWord checks if word exists as whole word in string (case-insensitive)
func ContainsWord(s, word string) bool {
	pattern := `(?i)\b` + regexp.QuoteMeta(word) + `\b`
	matched, _ := regexp.MatchString(pattern, s)
	return matched
}

// NumDayToYear converts number of days to "X năm Y tháng Z ngày" format
func NumDayToYear(numDay int) string {
	if numDay <= 0 {
		return ""
	}
	var parts []string
	year := numDay / 365
	if year > 0 {
		parts = append(parts, fmt.Sprintf("%d năm", year))
	}
	numDay -= year * 365
	month := numDay / 30
	if month > 0 {
		parts = append(parts, fmt.Sprintf("%d tháng", month))
	}
	numDay -= month * 30
	if numDay > 0 {
		parts = append(parts, fmt.Sprintf("%d ngày", numDay))
	}
	return strings.Join(parts, " ")
}

// NumberToByte formats bytes to human readable: 1048576 → "1 Megabyte"
func NumberToByte(num float64) string {
	units := []string{"Byte", "KB", "MB", "GB", "TB", "PB"}
	i := 0
	for num >= 1024 && i < len(units)-1 {
		num /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", num, units[i])
	}
	return fmt.Sprintf("%.2f %s", num, units[i])
}

// MbUcfirst uppercases first character (UTF-8 safe)
func MbUcfirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// GetNumbers extracts only digits and dots from string
func GetNumbers(s string) string {
	reg := regexp.MustCompile(`[^0-9.]`)
	return reg.ReplaceAllString(s, "")
}

// RemoveNumbers removes all digits from string
func RemoveNumbers(s string) string {
	reg := regexp.MustCompile(`[0-9]+`)
	return reg.ReplaceAllString(s, "")
}

// TimeElapsedString returns "X phút trước", "X giờ trước" etc.
func TimeElapsedString(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "vừa đây"
	case diff < time.Hour:
		return fmt.Sprintf("%d phút trước", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d giờ trước", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%d ngày trước", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%d tuần trước", int(diff.Hours()/(24*7)))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%d tháng trước", int(diff.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%d năm trước", int(diff.Hours()/(24*365)))
	}
}

// QueryArrayIndexed parses FE array query params like key[0]=a&key[1]=b
// Also handles key[]=a&key[]=b and key=a formats
func QueryArrayIndexed(c interface {
	Query(string) string
	QueryArray(string) []string
}, key string) []string {
	if vals := c.QueryArray(key + "[]"); len(vals) > 0 {
		return vals
	}
	if vals := c.QueryArray(key); len(vals) > 0 {
		return vals
	}
	var vals []string
	for i := 0; i < 100; i++ {
		v := c.Query(fmt.Sprintf("%s[%d]", key, i))
		if v == "" {
			break
		}
		vals = append(vals, v)
	}
	return vals
}

// ToInt64 normalizes MySQL driver int types to int64 for map key comparison.
// MySQL Go driver returns int32 for INT, int64 for BIGINT/TINYINT in map[string]interface{}.
func ToInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case uint:
		return int64(n)
	case uint32:
		return int64(n)
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	case decimal.Decimal:
		return n.IntPart()
	case *decimal.Decimal:
		if n == nil {
			return 0
		}
		return n.IntPart()
	case json.Number:
		i, _ := n.Int64()
		return i
	case string:
		val, _ := strconv.ParseInt(n, 10, 64)
		return val
	case []uint8:
		val, _ := strconv.ParseInt(string(n), 10, 64)
		return val
	}
	return 0
}

// ToFloat64 convert interface{} sang float64.
// MySQL decimal tra ve []uint8 khi dung map[string]interface{}.
func ToFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case int:
		return float64(n)
	case uint:
		return float64(n)
	case uint64:
		return float64(n)
	case decimal.Decimal:
		f, _ := n.Float64()
		return f
	case *decimal.Decimal:
		if n == nil {
			return 0
		}
		f, _ := n.Float64()
		return f
	case json.Number:
		f, _ := n.Float64()
		return f
	case []uint8:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

// ParseDecimalFields convert cac field decimal trong map tu []uint8 sang decimal.Decimal
// de JSON serialize thanh number thay vi string, giu precision chinh xac.
func ParseDecimalFields(row map[string]interface{}, fields ...string) {
	for _, f := range fields {
		if v, ok := row[f]; ok && v != nil {
			row[f] = ToDecimal(v)
		}
	}
}

// ToString converts interface{} to string, trả "" nếu nil hoặc không phải string.
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []uint8:
		return string(s)
	}
	return fmt.Sprintf("%v", v)
}

// ParseJSONFields converts string JSON values in map rows to json.RawMessage
// so gin serializes them as raw JSON (array/object) instead of escaped string.
// MySQL Go driver returns JSON columns as string in map[string]interface{}.
func ParseJSONFields(rows []map[string]interface{}) {
	for _, row := range rows {
		for k, v := range row {
			s, ok := v.(string)
			if !ok || len(s) < 2 {
				continue
			}
			// json.Valid bắt buộc: hàm này đoán theo ký tự đầu/cuối nên quét cả cột văn bản tự do
			// (note, desc...). Người dùng gõ "[11, 21, , 14]" → RawMessage hỏng → c.JSON marshal lỗi,
			// Gin trả 200 body rỗng không log gì. Không hợp lệ thì giữ nguyên là string.
			if ((s[0] == '[' && s[len(s)-1] == ']') || (s[0] == '{' && s[len(s)-1] == '}')) && json.Valid([]byte(s)) {
				row[k] = json.RawMessage(s)
			}
		}
	}
}

// GetIDParam reads "id" from path param first, falls back to query param.
// Yii2 passes id as query param (?id=9), Gin uses path param (/:id).
func GetIDParam(c interface {
	Param(string) string
	Query(string) string
}) uint {
	s := c.Param("id")
	if s == "" {
		s = c.Query("id")
	}
	id, _ := strconv.ParseUint(s, 10, 64)
	return uint(id)
}

// SerializeJSONFields converts map/slice values in a map to JSON strings
// so GORM can insert them into JSON columns without "unsupported type" errors.
// FE-only fields that should never be sent to DB
var feOnlyFields = map[string]bool{
	"customer_data": true,
}

func SerializeJSONFields(body map[string]interface{}) {
	for k, v := range body {
		if feOnlyFields[k] {
			delete(body, k)
			continue
		}
		switch v.(type) {
		case map[string]interface{}, []interface{}:
			b, _ := json.Marshal(v)
			body[k] = string(b)
		}
	}
}

// SerializeJSONFieldsForTable serialize các JSON columns của table dựa theo schema.
// Đảm bảo mọi giá trị (number, bool, map, slice) đều được convert sang JSON string
// trước khi insert/update vào MySQL JSON column.
func SerializeJSONFieldsForTable(body map[string]interface{}, table string) {
	if jsonColumnMap == nil {
		return
	}
	cols := jsonColumnMap[table]
	if len(cols) == 0 {
		return
	}
	for col := range cols {
		v, ok := body[col]
		if !ok {
			continue
		}
		if v == nil {
			continue
		}
		// Nếu đã là string hợp lệ JSON → giữ nguyên
		if s, ok := v.(string); ok {
			if len(s) > 0 && (s[0] == '[' || s[0] == '{' || s[0] == '"' || s == "null") {
				continue
			}
		}
		// Nếu là json.RawMessage → convert sang string
		if raw, ok := v.(json.RawMessage); ok {
			body[col] = string(raw)
			continue
		}
		// Mọi type khác (number, bool, map, slice) → marshal sang JSON string
		b, err := json.Marshal(v)
		if err == nil {
			body[col] = string(b)
		}
	}
}

// ParseID lấy param "id" từ gin.Context, trả về uint.
// Nếu không phải số hoặc = 0 → trả 0, false.
func ParseID(c interface{ Param(string) string }) (uint, bool) {
	return ParseParamID(c, "id")
}

// ParseParamID như ParseID nhưng cho path param KHÔNG tên "id"
// (assignmentId, customerId, rid, version_number…).
//
// Vì sao phải parse: MySQL so cột INT với chuỗi bằng cách ÉP chuỗi → số, nên
// Where("id = ?", "79738abc") khớp đúng dòng 79738 và chỉ sinh warning 1292 —
// GORM nuốt warning ⇒ build xanh, test xanh, request trả 200. Trả kiểu uint để
// COMPILER gác đường cũ thay vì dựa vào việc nhớ.
func ParseParamID(c interface{ Param(string) string }, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

// FlexFloat64 unmarshal được cả JSON number lẫn string số.
// FE đôi khi gửi discount/total dạng string "500000" thay vì number 500000.
type FlexFloat64 float64

func (f *FlexFloat64) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexFloat64(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = FlexFloat64(n)
	return nil
}

func (f FlexFloat64) Float64() float64 { return float64(f) }
func (f FlexFloat64) Ptr() *float64    { ; return new(float64(f)) }

// IntPtr returns a pointer to the given int value
func IntPtr(v int) *int {
	return &v
}

// NilIfEmpty chuyển *string trỏ tới chuỗi rỗng thành nil — dùng trước khi Create/Save
// cho cột nullable + unique index, tránh nhiều row "" cùng vi phạm unique constraint
// (empty string khác NULL trong MySQL unique index).
func NilIfEmpty(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// IntPtrEqual so sánh giá trị của hai *int, tránh so sánh pointer address.
func IntPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// ParseUint parses string to uint (returns 0 on error).
func ParseUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

// ParseIntSlice converts []string sang []int, bỏ qua phần tử parse lỗi.
// Dùng với query.GetFilterValues(c, key) — FE gửi "in" filter dạng
// filter[key][0]=a&filter[key][1]=b (KHÔNG phải Gin form tag "filter[key][]"
// literal, xem query.getArrayValues) nên field slice không thể bind trực tiếp
// qua struct tag, phải parse tay bằng cặp hàm này.
func ParseIntSlice(vals []string) []int {
	out := make([]int, 0, len(vals))
	for _, v := range vals {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// JoinUintsSorted trả chuỗi các id đã sort tăng dần, nối bằng "-" (rỗng → "").
// Dùng cho cache key theo tập id: KHÔNG dùng tổng (SUM) vì 2 tập khác nhau có thể
// cùng tổng ({1,4} vs {2,3}) → collision key → phục vụ nhầm cache khác scope.
func JoinUintsSorted(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	sorted := make([]uint, len(ids))
	copy(sorted, ids)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	parts := make([]string, len(sorted))
	for i, id := range sorted {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return strings.Join(parts, "-")
}

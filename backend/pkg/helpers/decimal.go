package helpers

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Money helpers dùng cho các field tiền đã migrate sang decimal.Decimal.
// Mục tiêu: arithmetic nil-safe + conversion từ input đa dạng (request body,
// query string, DB raw map[string]interface{}).

// ToDecimal convert interface{} bất kỳ sang decimal.Decimal.
// Trả decimal.Zero nếu không parse được. Dùng khi đọc từ map[string]interface{}
// (MySQL driver trả decimal(20,2) dạng []uint8).
func ToDecimal(v interface{}) decimal.Decimal {
	if v == nil {
		return decimal.Zero
	}
	switch n := v.(type) {
	case decimal.Decimal:
		return n
	case *decimal.Decimal:
		if n == nil {
			return decimal.Zero
		}
		return *n
	case float64:
		return decimal.NewFromFloat(n)
	case float32:
		return decimal.NewFromFloat(float64(n))
	case int:
		return decimal.NewFromInt(int64(n))
	case int32:
		return decimal.NewFromInt(int64(n))
	case int64:
		return decimal.NewFromInt(n)
	case uint:
		return decimal.NewFromInt(int64(n))
	case uint32:
		return decimal.NewFromInt(int64(n))
	case uint64:
		return decimal.NewFromInt(int64(n))
	case json.Number:
		d, err := decimal.NewFromString(string(n))
		if err != nil {
			return decimal.Zero
		}
		return d
	case []uint8:
		d, err := decimal.NewFromString(string(n))
		if err != nil {
			return decimal.Zero
		}
		return d
	case string:
		d, err := decimal.NewFromString(n)
		if err != nil {
			if f, err := strconv.ParseFloat(n, 64); err == nil {
				return decimal.NewFromFloat(f)
			}
			return decimal.Zero
		}
		return d
	}
	return decimal.Zero
}

// DerefDecimal trả decimal.Zero nếu pointer nil, ngược lại trả giá trị.
// Dùng khi arithmetic trên các field *decimal.Decimal.
func DerefDecimal(p *decimal.Decimal) decimal.Decimal {
	if p == nil {
		return decimal.Zero
	}
	return *p
}

// DecimalPtr trả pointer tới decimal.Decimal (tiện khi gán vào model field *decimal.Decimal).
func DecimalPtr(d decimal.Decimal) *decimal.Decimal {
	return &d
}

// NormalizeMapDecimals walk map và convert []uint8 → decimal.Decimal cho các value
// numeric. MySQL driver trả DECIMAL dưới dạng []uint8 khi scan vào map[string]interface{}
// → encoding/json marshal thành base64 string. Cần convert trước khi c.JSON.
// Non-numeric []uint8 (vd "abc") fallback string.
func NormalizeMapDecimals(m map[string]interface{}) {
	for k, v := range m {
		if b, ok := v.([]uint8); ok {
			s := string(b)
			if d, err := decimal.NewFromString(s); err == nil {
				m[k] = d
			} else {
				m[k] = s
			}
		}
	}
}

// NormalizeSliceDecimals áp NormalizeMapDecimals cho từng item trong slice.
func NormalizeSliceDecimals(items []map[string]interface{}) {
	for _, item := range items {
		NormalizeMapDecimals(item)
	}
}

// FormatDecimal format decimal.Decimal theo locale + symbol của hồ sơ doanh nghiệp (qua pkg/brand).
func FormatDecimal(db *gorm.DB, amount decimal.Decimal) string {
	// Với số tiền thông thường (VND tới hàng tỷ, USD tới triệu), float64 không mất precision
	// ở bước format UI. Reuse FormatAmount để giữ logic locale ở 1 chỗ.
	f, _ := amount.Float64()
	return FormatAmount(db, f)
}

// ParseExcelMoney parse chuỗi tiền/số đọc từ file Excel, trả (giá trị, ok).
//
// KHÔNG dùng decimal.NewFromString thẳng cho dữ liệu Excel: "1.000" theo quy ước VN/MISA là một
// nghìn, nhưng NewFromString đọc thành 1.0 — lệch 1000 lần, build vẫn xanh và không có lỗi nào nổi lên.
// Quy tắc: không có dấu phân cách → parse thẳng; có cả "." lẫn "," → dấu XUẤT HIỆN SAU là thập phân;
// chỉ một loại → nhóm ≤2 chữ số cuối là thập phân, ngược lại là dấu ngăn nghìn (bỏ đi).
// Scientific notation trả ok=false — đoán ở đây nguy hiểm hơn là bắt user sửa file.
// Ô trống trả (0, true): caller quyết định ô trống nghĩa là gì trong ngữ cảnh của mình.
func ParseExcelMoney(s string) (decimal.Decimal, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, true
	}
	s = strings.ReplaceAll(s, " ", "")
	if strings.ContainsAny(s, "eE") {
		return decimal.Zero, false
	}
	hasDot, hasComma := strings.Contains(s, "."), strings.Contains(s, ",")
	if !hasDot && !hasComma {
		if d, err := decimal.NewFromString(s); err == nil {
			return d, true
		}
		return decimal.Zero, false
	}
	switch {
	case hasDot && hasComma:
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") { // "," là dấu thập phân (VN)
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else { // "." là dấu thập phân (US)
			s = strings.ReplaceAll(s, ",", "")
		}
	case hasComma:
		if p := strings.Split(s, ","); len(p) == 2 && len(p[1]) <= 2 {
			s = p[0] + "." + p[1]
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case hasDot:
		if p := strings.Split(s, "."); !(len(p) == 2 && len(p[1]) <= 2) {
			s = strings.ReplaceAll(s, ".", "")
		}
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, false
	}
	return d, true
}

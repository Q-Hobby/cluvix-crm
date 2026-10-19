package query

import "net/url"

// Params — nguồn đọc tham số lọc/sắp xếp/từ khoá.
//
// `*gin.Context` THOẢ MÃN SẴN interface này (đã có đúng 2 method với đúng chữ ký), nên mọi
// call site cũ truyền `c` vào các hàm dưới compile nguyên — không sửa dòng nào ở 848 chỗ gọi.
//
// Lý do tồn tại: cron/worker cần dựng lại cùng một query danh sách từ bộ lọc đã lưu trong DB,
// nhưng KHÔNG có *gin.Context. Đường cũ duy nhất là bịa một context giả — cách đó nguy hiểm vì
// context còn mang PHẠM VI TENANT (chi nhánh từ session): context giả thiếu phạm vi thì
// query chạy xuyên chi nhánh mà không có lỗi nào báo. Tách nguồn-tham-số (Params) khỏi
// phạm-vi-tenant (tenant.Bounds) làm điều đó thành không thể diễn đạt được.
type Params interface {
	Query(key string) string
	QueryArray(key string) []string
}

// Values — cài đặt Params trên url.Values, cho cron/worker.
// Dựng từ chuỗi query đã lưu: query.ParseValues(sub.FilterQuery).
type Values struct{ v url.Values }

// FromValues bọc url.Values thành Params.
func FromValues(v url.Values) Values { return Values{v: v} }

// ParseValues dựng Params từ chuỗi query dạng `filter[branch_id]=3&status[]=1`.
// Chuỗi hỏng → Params rỗng (mọi filter coi như không có), KHÔNG panic.
func ParseValues(raw string) Values {
	v, err := url.ParseQuery(raw)
	if err != nil {
		return Values{v: url.Values{}}
	}
	return Values{v: v}
}

// Query trả giá trị đầu tiên của key, "" nếu không có — khớp hành vi gin.Context.Query.
func (p Values) Query(key string) string {
	if p.v == nil {
		return ""
	}
	return p.v.Get(key)
}

// QueryArray trả mọi giá trị của key, slice rỗng nếu không có — khớp gin.Context.QueryArray.
func (p Values) QueryArray(key string) []string {
	if p.v == nil {
		return nil
	}
	return p.v[key]
}

// Encode trả lại chuỗi query — dùng khi lưu bộ lọc của một đăng ký xuống DB.
func (p Values) Encode() string {
	if p.v == nil {
		return ""
	}
	return p.v.Encode()
}

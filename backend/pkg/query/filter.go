package query

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// fieldRegex accepts table.column or column — no spaces, semicolons, quotes, etc.
var fieldRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

func isAllowedField(f string, allowed []string) bool {
	if !fieldRegex.MatchString(f) {
		return false
	}
	for _, a := range allowed {
		if f == a {
			return true
		}
	}
	return false
}

// --- Operator definitions ---
// FE gửi filter theo Yii2 format: filter[field][operator]=value hoặc _field[operator]=value
// Supported operators: eq, neq, like, in, gt, gte, lt, lte, is (NULL), nis (NOT NULL)

// neq bị tách khỏi map này — three-valued logic fix (Story 03): column != ?
// loại âm thầm record NULL, cần format riêng "(column != ? OR column IS NULL)"
// (cần column 2 lần, map value chỉ nối được 1 fragment sau column).
var operatorSQL = map[string]string{
	"eq":  " = ?",
	"gt":  " > ?",
	"gte": " >= ?",
	"lt":  " < ?",
	"lte": " <= ?",
}

// getOperatorValue tìm giá trị cho 1 operator từ cả 2 format:
//   - filter[key][op]=val  (regular fields)
//   - key[op]=val          (prefix _ fields)
func getOperatorValue(c Params, key, op string) string {
	if v := c.Query(fmt.Sprintf("filter[%s][%s]", key, op)); v != "" {
		return v
	}
	if v := c.Query(fmt.Sprintf("%s[%s]", key, op)); v != "" {
		return v
	}
	return ""
}

// GetOperatorValue là bản export của hàm trên. Handler nào cần biết GIÁ TRỊ của dải lọc (vd để
// quyết định plan/index hint theo độ rộng cửa sổ ngày) phải dùng hàm này thay vì tự ghép chuỗi —
// ghép tay sẽ trượt một trong hai format và IM LẶNG trả rỗng.
func GetOperatorValue(c Params, key, op string) string { return getOperatorValue(c, key, op) }

// getArrayValues tìm giá trị array cho in/nin từ cả 2 format:
//   - filter[key][0]=a&filter[key][1]=b
//   - filter[key][in][0]=a&filter[key][in][1]=b
//   - key[0]=a&key[1]=b
func getArrayValues(c Params, key string) []string {
	// filter[key][0], filter[key][1] ...
	var vals []string
	for i := 0; i < 100; i++ {
		v := c.Query(fmt.Sprintf("filter[%s][%d]", key, i))
		if v == "" {
			break
		}
		vals = append(vals, v)
	}
	if len(vals) > 0 {
		return vals
	}
	// filter[key][in][0], filter[key][in][1] ...
	for i := 0; i < 100; i++ {
		v := c.Query(fmt.Sprintf("filter[%s][in][%d]", key, i))
		if v == "" {
			break
		}
		vals = append(vals, v)
	}
	if len(vals) > 0 {
		return vals
	}
	// key[0], key[1] ... (prefix _ fields)
	for i := 0; i < 100; i++ {
		v := c.Query(fmt.Sprintf("%s[%d]", key, i))
		if v == "" {
			break
		}
		vals = append(vals, v)
	}
	return vals
}

// GetFilterValues trả về danh sách giá trị cho 1 filter key.
// Parse tất cả format: key=a, key[]=a, key[0]=a, filter[key]=a, filter[key][eq]=a, filter[key][0]=a
func GetFilterValues(c Params, key string) []string {
	// key[] format
	if vals := c.QueryArray(key + "[]"); len(vals) > 0 {
		return vals
	}
	// key format (single or multiple)
	if vals := c.QueryArray(key); len(vals) > 0 {
		return vals
	}
	// Array values: key[0], filter[key][0], filter[key][in][0]
	if vals := getArrayValues(c, key); len(vals) > 0 {
		return vals
	}
	// filter[key] (Yii2 single value)
	if v := c.Query("filter[" + key + "]"); v != "" {
		return []string{v}
	}
	// filter[key][eq] hoặc key[eq]
	if v := getOperatorValue(c, key, "eq"); v != "" {
		return []string{v}
	}
	return nil
}

// GetFilterValue trả về 1 giá trị filter đầu tiên
func GetFilterValue(c Params, key string) string {
	vals := GetFilterValues(c, key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// ApplyFilter áp dụng tất cả operators cho 1 field.
// Parse: eq, neq, like, in (array), gt, gte, lt, lte, is (NULL), nis (NOT NULL)
// Signature giữ nguyên — backward compatible với 246 handler calls.
func ApplyFilter(c Params, q *gorm.DB, paramKey, column string) *gorm.DB {
	// Cột ID thuần số ⇒ giá trị KHÔNG parse được phải trả VỀ RỖNG, không được để MySQL ép kiểu
	// (xem numeric_id_filter.go: 'filter[customer_id]=888002abc' từng đọc trúng khách 888002).
	// Cột không nằm trong allowlist giữ nguyên hành vi cũ.
	idCol := isNumericIDColumn(column)

	// eq / gt / gte / lt / lte
	hasOperator := false
	for op, sql := range operatorSQL {
		if v := getOperatorValue(c, paramKey, op); v != "" {
			if idCol && !isValidIDValue(v) {
				// So sánh với một id không tồn tại được ⇒ không dòng nào khớp. Trả rỗng là
				// hướng AN TOÀN: bỏ qua điều kiện sẽ nới rộng phạm vi (trả dữ liệu khách khác).
				return q.Where("1 = 0")
			}
			q = q.Where(column+sql, v)
			hasOperator = true
		}
	}

	// neq — three-valued logic fix (Story 03): "(column != ? OR column IS NULL)"
	// thay vì "column != ?" (loại âm thầm record NULL). Bọc ngoặc để không phá
	// AND/OR bên ngoài khi caller nối thêm điều kiện khác (raw SQL fragment rule).
	if v := getOperatorValue(c, paramKey, "neq"); v != "" {
		// neq đi hướng NGƯỢC LẠI với eq: "khác một id không hợp lệ" đúng với MỌI dòng, nên bỏ
		// hẳn mệnh đề mới là đúng ngữ nghĩa — trả rỗng ở đây sẽ giấu mất dữ liệu hợp lệ.
		if idCol && !isValidIDValue(v) {
			hasOperator = true
		} else {
			q = q.Where("("+column+" != ? OR "+column+" IS NULL)", v)
			hasOperator = true
		}
	}

	// like
	if v := getOperatorValue(c, paramKey, "like"); v != "" {
		q = q.Where(column+" LIKE ?", "%"+v+"%")
		hasOperator = true
	}

	// is (NULL) / nis (NOT NULL)
	if v := getOperatorValue(c, paramKey, "is"); v != "" {
		q = q.Where(column + " IS NULL")
		hasOperator = true
	}
	if v := getOperatorValue(c, paramKey, "nis"); v != "" {
		q = q.Where(column + " IS NOT NULL")
		hasOperator = true
	}

	// in (array) / exact match — chỉ khi chưa có operator nào match ở trên
	if !hasOperator {
		if vals := getArrayValues(c, paramKey); len(vals) > 0 {
			if idCol {
				// Lọc bỏ phần tử rác; rác HẾT ⇒ không dòng nào khớp.
				if vals = filterNumericIDValues(vals); len(vals) == 0 {
					return q.Where("1 = 0")
				}
			}
			if len(vals) == 1 {
				q = q.Where(column+" = ?", vals[0])
			} else {
				q = q.Where(column+" IN ?", vals)
			}
		} else if v := c.Query("filter[" + paramKey + "]"); v != "" {
			// filter[key]=val (Yii2 single, no operator)
			if idCol && !isValidIDValue(v) {
				return q.Where("1 = 0")
			}
			q = q.Where(column+" = ?", v)
		} else if v := c.Query(paramKey); v != "" {
			// key=val (direct)
			if idCol && !isValidIDValue(v) {
				return q.Where("1 = 0")
			}
			q = q.Where(column+" = ?", v)
		}
	}

	return q
}

// allowedJSONColumns whitelists columns permitted for JSON_CONTAINS to prevent SQL injection.
var allowedJSONColumns = map[string]bool{
	"u.branchId":       true,
	"p.departmentList": true,
}

// ApplyJSONContainsFilter áp dụng JSON_CONTAINS cho JSON array columns.
// column phải nằm trong allowedJSONColumns — unknown columns bị skip silent.
func ApplyJSONContainsFilter(c Params, q *gorm.DB, paramKey, column string) *gorm.DB {
	if !allowedJSONColumns[column] {
		return q
	}
	vals := GetFilterValues(c, paramKey)
	if len(vals) == 0 {
		return q
	}
	for _, v := range vals {
		q = q.Where("JSON_CONTAINS("+column+", ?)", v)
	}
	return q
}

// ApplyKeywordFilter applies LIKE search across multiple columns
func ApplyKeywordFilter(c Params, q *gorm.DB, columns ...string) *gorm.DB {
	keyword := strings.TrimSpace(c.Query("_keyword"))
	if keyword == "" {
		return q
	}
	like := "%" + keyword + "%"
	conditions := make([]string, len(columns))
	args := make([]interface{}, len(columns))
	for i, col := range columns {
		conditions[i] = col + " LIKE ?"
		args[i] = like
	}
	return q.Where(strings.Join(conditions, " OR "), args...)
}

// sortRejectedSeen — dedup once-per-process cho log SortRejected (key = path+field).
// Tránh spam Telegram khi mỗi page load đều trigger silent fallback.
var sortRejectedSeen sync.Map

// ApplySort parses Yii2 sort param: sort=-field (DESC) or sort=field (ASC).
// Falls back to defaultOrder if no sort param provided.
// allowed: whitelist of permitted sort fields (e.g. "payment.id", "payment.created_day").
// When allowed is non-empty, field must be in the whitelist; regex format is always enforced.
//
// Khi sort param có nhưng bị reject (không match whitelist hoặc fail regex) →
// log Error 1 lần per (request_path, field) để FE/BE dev biết FE đang sort field
// mà BE chưa hỗ trợ — tránh silent fallback gây kết quả khác mong đợi.
//
// Khi sort hợp lệ, ORDER BY luôn được nối thêm tiebreaker `<bảng gốc>.id DESC` (xem
// sortTiebreaker). ⚠ Nếu sau này thêm query GROUP BY **không** theo primary key bảng gốc
// (vd `GROUP BY st.id, st.name` ở report tổng hợp) mà defaultOrder vẫn có prefix bảng:
// `sql_mode=ONLY_FULL_GROUP_BY` sẽ báo lỗi 1055 vì cột tiebreaker không nằm trong GROUP BY.
// Rà 2026-08-13: 71 call site không có ca nào như vậy — 3 query GROUP BY đều group theo pk,
// 2 query DISTINCT đều là clone dựng trước ApplySort nên không nhận ORDER BY này.
func ApplySort(c Params, q *gorm.DB, defaultOrder string, allowed ...string) *gorm.DB {
	return q.Order(SortClause(c, defaultOrder, allowed...))
}

// SortClause trả về ĐÚNG mệnh đề ORDER BY mà ApplySort sẽ áp — kể cả nhánh từ chối + ghi log.
//
// Vì sao tách ra: handler nào tự phân trang bằng "tra id trước rồi mới nạp cột" (deferred join,
// xem `pagination.PageParams`/`ResultForPage`) cần CHÍNH mệnh đề đó cho câu tra id, và cần bản đảo
// chiều (ReverseOrderClause) khi đọc ngược từ cuối. Chép lại luật thứ tự ra chỗ thứ hai là mời hai
// bên lệch nhau về sau — mà lệch ở đây nghĩa là trang trả dòng khác, im lặng.
//
// Gọi nhiều lần trong một request là an toàn: `logSortRejected` tự khử trùng theo (path, field).
func SortClause(c Params, defaultOrder string, allowed ...string) string {
	sort := strings.TrimSpace(c.Query("sort"))
	if sort == "" {
		return withTiebreaker(defaultOrder)
	}
	field := sort
	dir := " ASC"
	if strings.HasPrefix(sort, "-") {
		field = sort[1:]
		dir = " DESC"
	}
	// Add table prefix if field has no dot and defaultOrder has a table prefix
	if !strings.Contains(field, ".") {
		if idx := strings.Index(defaultOrder, "."); idx > 0 {
			field = defaultOrder[:idx+1] + field
		}
	}
	// Always validate format; check whitelist when provided.
	rejected := false
	if len(allowed) > 0 {
		if !isAllowedField(field, allowed) {
			rejected = true
		}
	} else if !fieldRegex.MatchString(field) {
		rejected = true
	}
	if rejected {
		logSortRejected(c, sort, field, allowed)
		return withTiebreaker(defaultOrder)
	}
	// Tiebreaker: sort theo cột KHÔNG unique (appointment_date, created_day, confirm_time...)
	// cho thứ tự không xác định giữa các row bằng nhau → LIMIT/OFFSET không ổn định, row có thể
	// lặp hoặc mất giữa hai trang list, và giữa hai batch 5000 dòng của export
	// (`pkg/export/export.go` fetch bằng LIMIT/OFFSET trên cùng query này).
	// Thêm pk bảng gốc để ORDER BY thành thứ tự toàn phần.
	if tb := sortTiebreaker(defaultOrder); tb != "" && field != tb {
		return field + dir + ", " + tb + " DESC"
	}
	return field + dir
}

// ReverseOrderClause đảo chiều TỪNG VẾ của một mệnh đề ORDER BY: "a DESC, b DESC" → "a ASC, b ASC".
//
// Dùng cho phân trang sâu đọc ngược từ cuối (trang nằm nửa sau ⇒ OFFSET đối xứng trên thứ tự đảo,
// rẻ hơn hàng nghìn lần). CHỈ đúng khi mệnh đề là thứ tự TOÀN PHẦN — `SortClause` bảo đảm điều đó
// bằng tiebreaker pk.
//
// Trả "" khi gặp vế không phải dạng `<cột> [ASC|DESC]` đơn giản (biểu thức, hàm, alias có dấu
// ngoặc): caller phải hiểu đó là "không đảo được" và đi đường thường, KHÔNG đoán bừa — đoán sai ở
// đây là trả nhầm dòng mà không có lỗi nào bật ra.
func ReverseOrderClause(clause string) string {
	terms := strings.Split(clause, ",")
	out := make([]string, 0, len(terms))
	for _, t := range terms {
		f := strings.Fields(strings.TrimSpace(t))
		if len(f) == 0 || len(f) > 2 || !fieldRegex.MatchString(f[0]) {
			return ""
		}
		dir := "ASC"
		if len(f) == 2 {
			switch strings.ToUpper(f[1]) {
			case "DESC":
				dir = "ASC"
			case "ASC":
				dir = "DESC"
			default:
				return ""
			}
		} else {
			dir = "DESC" // không ghi chiều = ASC ⇒ đảo thành DESC
		}
		out = append(out, f[0]+" "+dir)
	}
	return strings.Join(out, ", ")
}

// withTiebreaker nối `<bảng gốc>.id DESC` vào defaultOrder khi defaultOrder chưa có cột đó.
//
// Vì sao cần: `sort` param VẮNG MẶT là ca THƯỜNG GẶP NHẤT — mọi lần mở màn list lần đầu, mọi
// list mà người dùng không tự chọn cột sắp xếp, và mọi vòng lặp export
// (`pkg/export/export.go` fetch theo batch 5000 bằng LIMIT/OFFSET trên cùng query này).
// Trước 2026-09-06, tiebreaker CHỈ được thêm khi người dùng bấm sort ⇒ đúng đường đi phổ biến
// nhất lại là đường KHÔNG có thứ tự toàn phần: defaultOrder kiểu
// "order_service_used.created_day DESC" hay "appointment.appointment_date DESC" không unique,
// LIMIT/OFFSET trên đó cho phép MySQL trả thứ tự khác nhau giữa 2 query ⇒ một dòng ra 2 lần ở
// trang này và biến mất khỏi trang kia, KHÔNG có lỗi nào được ném ra.
//
// Không suy được bảng gốc (defaultOrder theo alias tổng hợp: "records DESC", "count DESC")
// ⇒ giữ nguyên. Đây cũng chính là lưới chặn lỗi 1055: các query GROUP BY không-theo-pk trong
// repo (emr/report) đều order theo alias nên không bao giờ nhận tiebreaker.
func withTiebreaker(defaultOrder string) string {
	tb := sortTiebreaker(defaultOrder)
	if tb == "" || orderHasColumn(defaultOrder, tb) {
		return defaultOrder
	}
	return defaultOrder + ", " + tb + " DESC"
}

// orderHasColumn báo defaultOrder đã chứa đúng cột col như MỘT vế ORDER BY chưa.
//
// So khớp theo VẾ, không phải substring: "payment.id_old DESC" chứa chuỗi "payment.id" nhưng
// KHÔNG phải cột đó — nhận nhầm ⇒ bỏ qua tiebreaker ⇒ bug im lặng quay lại đúng chỗ đã vá.
func orderHasColumn(defaultOrder, col string) bool {
	for _, term := range strings.Split(defaultOrder, ",") {
		term = strings.TrimSpace(term)
		if i := strings.IndexAny(term, " \t"); i > 0 {
			term = term[:i]
		}
		if term == col {
			return true
		}
	}
	return false
}

// sortTiebreaker suy ra cột phá hoà `<bảng gốc>.id` từ defaultOrder.
//
// Lấy prefix từ defaultOrder (bảng gốc) chứ KHÔNG từ field đang sort: field có thể trỏ sang
// bảng JOIN (vd sort `customer.fullname` trong list lịch hẹn) — `customer.id` không phá hoà được
// hai lịch hẹn của cùng một khách, chỉ pk bảng gốc mới đảm bảo thứ tự toàn phần.
//
// defaultOrder không có prefix bảng (report tổng hợp order theo alias: "records DESC",
// "count DESC") → trả "" = không thêm tiebreaker, vì không suy được pk.
func sortTiebreaker(defaultOrder string) string {
	head := strings.TrimSpace(defaultOrder)
	if i := strings.IndexAny(head, " ,"); i > 0 {
		head = head[:i]
	}
	idx := strings.Index(head, ".")
	if idx <= 0 {
		return ""
	}
	table := head[:idx]
	if !fieldRegex.MatchString(table) {
		return ""
	}
	return table + ".id"
}

func logSortRejected(c Params, sortParam, normalized string, allowed []string) {
	// Chỉ đường có HTTP request mới có path để ghi log; đường cron (Values) không có.
	path := "background"
	if gc, ok := c.(*gin.Context); ok && gc.Request != nil {
		path = gc.Request.Method + " " + gc.Request.URL.Path
	}
	key := path + "|" + normalized
	if _, loaded := sortRejectedSeen.LoadOrStore(key, struct{}{}); loaded {
		return // đã log rồi, bỏ qua
	}
	logger.L.Error(fmt.Sprintf("SortRejected @ %s: sort=%q → field=%q (whitelist=%v) — fallback to default",
		path, sortParam, normalized, allowed))
}

// GetCountryIdsByGroupCountry reads locationCountryId JSON arrays from group_country
// and returns flattened list of country IDs
func GetCountryIdsByGroupCountry(dbConn *gorm.DB, groupCountryIds []string) []int {
	var rows []struct {
		LocationCountryId string `gorm:"column:locationCountryId"`
	}
	dbConn.Table("group_country").Select("locationCountryId").Where("id IN ?", groupCountryIds).Find(&rows)

	var result []int
	for _, r := range rows {
		if r.LocationCountryId == "" {
			continue
		}
		var ids []int
		if err := json.Unmarshal([]byte(r.LocationCountryId), &ids); err == nil {
			result = append(result, ids...)
		}
	}
	return result
}

// GetSourceIdsByGroupSource reads sourceId JSON arrays from group_source
// and returns flattened list of source IDs
func GetSourceIdsByGroupSource(dbConn *gorm.DB, groupSourceIds []string) []int {
	var rows []struct {
		SourceId string `gorm:"column:sourceId"`
	}
	dbConn.Table("group_source").Select("sourceId").Where("id IN ?", groupSourceIds).Find(&rows)

	var result []int
	for _, r := range rows {
		if r.SourceId == "" {
			continue
		}
		var ids []int
		if err := json.Unmarshal([]byte(r.SourceId), &ids); err == nil {
			result = append(result, ids...)
		}
	}
	return result
}

// GetProvinceIdsByGroupLocal reads locationProvinceId JSON arrays from group_local
// and returns flattened list of province IDs
func GetProvinceIdsByGroupLocal(dbConn *gorm.DB, groupLocalIds []string) []int {
	var rows []struct {
		LocationProvinceId string `gorm:"column:locationProvinceId"`
	}
	dbConn.Table("group_local").Select("locationProvinceId").Where("id IN ?", groupLocalIds).Find(&rows)

	var result []int
	for _, r := range rows {
		if r.LocationProvinceId == "" {
			continue
		}
		var ids []int
		if err := json.Unmarshal([]byte(r.LocationProvinceId), &ids); err == nil {
			result = append(result, ids...)
		}
	}
	return result
}

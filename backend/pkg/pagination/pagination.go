package pagination

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"erpCluvix/db"
	"erpCluvix/pkg/helpers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Meta struct {
	TotalCount  int64 `json:"totalCount"`
	PageCount   int   `json:"pageCount"`
	CurrentPage int   `json:"currentPage"`
	PerPage     int   `json:"perPage"`
}

type Result struct {
	Items interface{} `json:"items"`
	Meta  Meta        `json:"_meta"`
}

func Paginate(c *gin.Context, query *gorm.DB, dest interface{}) Result {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per-page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}

	var total int64
	cnt := db.DB.Raw("SELECT COUNT(*) FROM (?) AS cnt", query.Session(&gorm.Session{})).Scan(&total)
	if IsQueryError(cnt.Error) {
		return failList(c, cnt.Error, dest, page, perPage)
	}

	offset := (page - 1) * perPage
	tx := query.Offset(offset).Limit(perPage).Find(dest)
	if IsQueryError(tx.Error) {
		return failList(c, tx.Error, dest, page, perPage)
	}

	// Ensure empty slice returns [] not null in JSON
	emptySliceNotNull(dest)

	// Auto-parse JSON + decimal columns using schema map
	if rows, ok := dest.(*[]map[string]interface{}); ok && len(*rows) > 0 {
		tables := extractTables(tx, query)
		helpers.ParseJSONFieldsForTables(*rows, tables...)
		helpers.ParseDecimalFieldsForTables(*rows, tables...)
	}

	pageCount := int(math.Ceil(float64(total) / float64(perPage)))

	c.Header("X-Pagination-Total-Count", strconv.FormatInt(total, 10))
	c.Header("X-Pagination-Page-Count", strconv.Itoa(pageCount))
	c.Header("X-Pagination-Current-Page", strconv.Itoa(page))
	c.Header("X-Pagination-Per-Page", strconv.Itoa(perPage))

	return Result{
		Items: dest,
		Meta: Meta{
			TotalCount:  total,
			PageCount:   pageCount,
			CurrentPage: page,
			PerPage:     perPage,
		},
	}
}

// PaginateWithTotal — giống Paginate nhưng nhận total từ bên ngoài, tránh chạy COUNT thừa.
func PaginateWithTotal(c *gin.Context, query *gorm.DB, dest interface{}, total int64) Result {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per-page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}

	offset := (page - 1) * perPage
	tx := query.Offset(offset).Limit(perPage).Find(dest)
	if IsQueryError(tx.Error) {
		return failList(c, tx.Error, dest, page, perPage)
	}

	emptySliceNotNull(dest)

	if rows, ok := dest.(*[]map[string]interface{}); ok && len(*rows) > 0 {
		tables := extractTables(tx, query)
		helpers.ParseJSONFieldsForTables(*rows, tables...)
		helpers.ParseDecimalFieldsForTables(*rows, tables...)
	}

	pageCount := int(math.Ceil(float64(total) / float64(perPage)))

	c.Header("X-Pagination-Total-Count", strconv.FormatInt(total, 10))
	c.Header("X-Pagination-Page-Count", strconv.Itoa(pageCount))
	c.Header("X-Pagination-Current-Page", strconv.Itoa(page))
	c.Header("X-Pagination-Per-Page", strconv.Itoa(perPage))

	return Result{
		Items: dest,
		Meta: Meta{
			TotalCount:  total,
			PageCount:   pageCount,
			CurrentPage: page,
			PerPage:     perPage,
		},
	}
}

// extractTables gets main table + joined tables from GORM statement (call after Find).
//
// story-16: KHÔNG dùng `Statement.SQL` để trích bảng JOIN — GORM v1.31.1 gọi `stmt.SQL.Reset()`
// sau khi execute (gorm@v1.31.1/callbacks.go:150) nên sau `Find` thì `SQL.Len() == 0` ở CẢ `tx`
// lẫn `original` (đo runtime, mọi kịch bản) ⇒ nhánh cũ là dead code, cột DECIMAL của bảng JOIN
// không bao giờ được parse. `Statement.Joins` KHÔNG bị reset ⇒ đó mới là nguồn tin cậy.
func extractTables(tx *gorm.DB, original *gorm.DB) []string {
	stmt := tx.Statement
	if stmt == nil {
		stmt = original.Statement
	}
	if stmt == nil {
		return nil
	}

	var tables []string
	seen := make(map[string]bool, 4)
	add := func(t string) {
		if t != "" && !seen[t] {
			seen[t] = true
			tables = append(tables, t)
		}
	}

	add(mainTableName(stmt))

	// Bảng JOIN: `Joins[i].Name` là raw string của mệnh đề JOIN
	// (vd "LEFT JOIN `order` o ON o.customer_id = c.id"). Join theo tên quan hệ (Joins("Customer"))
	// không chứa "JOIN " nên tự động bị bỏ qua.
	for _, s := range []*gorm.DB{tx, original} {
		if s == nil || s.Statement == nil {
			continue
		}
		for _, j := range s.Statement.Joins {
			for _, t := range joinedTables(j.Name) {
				add(t)
			}
		}
	}
	return tables
}

// mainTableName trả TÊN BẢNG thật của bảng chính.
// `Statement.Table` KHÔNG đáng tin: với `Table("customer c")`, GORM `tableRegexp`
// (chainable_api.go:58) đặt `Statement.Table = "c"` — tức ALIAS, không phải tên bảng.
// `Statement.TableExpr.SQL` giữ nguyên chuỗi gốc nên lấy token đầu của nó trước.
func mainTableName(stmt *gorm.Statement) string {
	if stmt.TableExpr != nil {
		if fields := strings.Fields(stmt.TableExpr.SQL); len(fields) > 0 {
			if t := cleanTableToken(fields[0]); t != "" {
				return t
			}
		}
	}
	return cleanTableToken(stmt.Table)
}

// joinedTables trích tên bảng từ raw string của một mệnh đề JOIN.
// joinKeywordRe khớp "JOIN" theo sau bởi BẤT KỲ whitespace nào (space/tab/newline/…), không chỉ
// đúng 1 dấu cách — `Joins("LEFT JOIN\n\tcustomer c ON …")` (SQL multi-dòng) phải khớp được.
var joinKeywordRe = regexp.MustCompile(`(?i)JOIN\s+`)

func joinedTables(clause string) []string {
	if clause == "" {
		return nil
	}
	var out []string
	for idx := 0; idx < len(clause); {
		loc := joinKeywordRe.FindStringIndex(clause[idx:])
		if loc == nil {
			break
		}
		start := idx + loc[1]
		if start >= len(clause) {
			break
		}
		if clause[start] == '(' {
			// Join-subquery (`LEFT JOIN (SELECT …) s ON …`) — nhảy qua HẾT cặp ngoặc tương ứng rồi
			// mới tiếp tục quét, KHÔNG đi vào thân subquery (bảng JOIN lồng bên trong không được
			// chiếu ra response nên không được tính là bảng của câu query ngoài).
			closeIdx := matchingParenIndex(clause, start)
			if closeIdx < 0 {
				break // ngoặc không cân — dừng an toàn, tránh đọc sai phần còn lại
			}
			idx = closeIdx + 1
			continue
		}
		end := start
		for end < len(clause) && clause[end] != ' ' && clause[end] != '\n' && clause[end] != '\t' &&
			clause[end] != '\r' && clause[end] != '(' && clause[end] != ',' {
			end++
		}
		if end <= start {
			idx = start + 1
			continue
		}
		if t := cleanTableToken(clause[start:end]); t != "" {
			out = append(out, t)
		}
		idx = end
	}
	return out
}

// matchingParenIndex trả index của ')' khớp với '(' tại openIdx (đếm độ sâu lồng nhau qua
// ngoặc con bên trong subquery). Trả -1 nếu ngoặc không cân (không nên xảy ra với SQL hợp lệ).
func matchingParenIndex(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// cleanTableToken bỏ backtick/quote, bỏ tiền tố schema (`db`.`tbl`), và loại token không phải
// tên bảng (join subquery `LEFT JOIN (SELECT …) s ON …`).
func cleanTableToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" || strings.ContainsAny(tok, "(),") {
		return ""
	}
	if i := strings.LastIndex(tok, "."); i >= 0 {
		tok = tok[i+1:]
	}
	return strings.Trim(tok, "`\"")
}

// PageParams đọc `page` / `per-page` theo ĐÚNG luật Paginate/PaginateWithTotal (kể cả các
// mức kẹp: page < 1 ⇒ 1; per-page ngoài [1,200] ⇒ 20) và trả kèm offset đã tính.
//
// Dùng khi handler phải tự áp LIMIT/OFFSET — điển hình là **deferred join**: tra danh sách id
// của trang bằng một query gọn (không join, không cột rộng) rồi mới nạp đủ cột cho đúng ≤N id
// đó. Cần vì `PaginateWithTotal` tự gọi `Offset(...).Limit(...)`, mà đường deferred join đã
// tiêu thụ offset ở bước tra id ⇒ để nó áp offset lần nữa thì trang 2 trở đi trả RỖNG.
//
// ⚠ Ba mức kẹp này phải khớp Paginate. Đổi ở đây mà quên đổi ở kia ⇒ handler dùng
// PageParams và handler dùng Paginate phân trang lệch nhau trên cùng một tham số URL.
func PageParams(c *gin.Context) (page, perPage, offset int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ = strconv.Atoi(c.DefaultQuery("per-page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}
	return page, perPage, (page - 1) * perPage
}

// ResultForPage đóng gói `Result` + header phân trang cho một trang ĐÃ được handler tự nạp
// (không áp thêm LIMIT/OFFSET, không chạy COUNT). Đi cùng PageParams cho đường deferred join.
//
// `tables` là danh sách bảng dùng để parse JSON/decimal — Paginate tự suy ra từ statement, ở đây
// handler phải nói rõ vì query nạp dữ liệu không còn là query gốc.
func ResultForPage(c *gin.Context, dest interface{}, total int64, page, perPage int, tables ...string) Result {
	emptySliceNotNull(dest)

	if rows, ok := dest.(*[]map[string]interface{}); ok && len(*rows) > 0 && len(tables) > 0 {
		helpers.ParseJSONFieldsForTables(*rows, tables...)
		helpers.ParseDecimalFieldsForTables(*rows, tables...)
	}

	pageCount := int(math.Ceil(float64(total) / float64(perPage)))

	c.Header("X-Pagination-Total-Count", strconv.FormatInt(total, 10))
	c.Header("X-Pagination-Page-Count", strconv.Itoa(pageCount))
	c.Header("X-Pagination-Current-Page", strconv.Itoa(page))
	c.Header("X-Pagination-Per-Page", strconv.Itoa(perPage))

	return Result{
		Items: dest,
		Meta: Meta{
			TotalCount:  total,
			PageCount:   pageCount,
			CurrentPage: page,
			PerPage:     perPage,
		},
	}
}

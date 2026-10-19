// Package staff — báo cáo theo NHÂN VIÊN của CRM: doanh số tư vấn viên (`order_revenue_telesale`) và hoa hồng
// tư vấn viên (`order_commission_telesale`). Mỗi báo cáo có màn chi tiết (list) + màn tổng hợp (GROUP BY nhân viên).
//
// Port từ ERP `accountant/revenue_telesale` + `accountant/commission_telesale` + spec chung
// `accountant/revenue_staff`. Khác ERP (CRM = 1 thương hiệu/DB, SME):
//   - KHÔNG còn `company_id` (bộ lọc + cột select);
//   - từ khoá đi LIKE thô qua `LEFT JOIN customer` (CRM không có `customer.search_text`/FULLTEXT ngram);
//   - bỏ đường phân trang deferred-join + `FORCE INDEX` (tối ưu cho bảng ERP ≥600k dòng) — dùng
//     PaginateWithTotal thường. Cần lại khi đo được chậm: xem ERP `revenue_staff.go`.
package staff

import (
	"fmt"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erpCluvix/internal/modules/webapp/crm/customer/scope"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
)

// sumCol — một cột tiền được cộng ở dòng tổng (list) và ở từng nhóm (tổng hợp).
type sumCol struct {
	Col   string // cột trên bảng, vd "revenue"
	Alias string // tên trường ở màn tổng hợp, vd "totalRevenue"
}

// spec mô tả 1 bảng doanh số / hoa hồng theo nhân viên.
type spec struct {
	Table     string   // vd "order_revenue_telesale"
	MemberCol string   // cột nhân viên, cũng là tên bộ lọc — vd "revenue_id"
	Sums      []sumCol // Sums[0] là cột CHÍNH: dòng tổng list, ORDER BY và dòng tổng màn tổng hợp
}

var (
	revenueTelesale = spec{Table: "order_revenue_telesale", MemberCol: "revenue_id",
		Sums: []sumCol{{"revenue", "totalRevenue"}}}
	commissionTelesale = spec{Table: "order_commission_telesale", MemberCol: "revenue_id",
		Sums: []sumCol{{"commissionTelesale", "totalCommission"}, {"amount", "totalAmount"}}}
)

// keywordCols — cột từ khoá màn list (LIKE thô): khách (qua LEFT JOIN customer) + mã dịch vụ sử dụng.
var keywordCols = []string{"customer.fullname", "customer.code", "customer.phone", "order_service_used.code"}

func (s spec) col(name string) string { return s.Table + "." + name }

func (s spec) joinServiceUsed() string {
	return "LEFT JOIN order_service_used ON order_service_used.id = " + s.col("order_service_used_id")
}

func (s spec) joinCustomer() string {
	return "LEFT JOIN customer ON customer.id = " + s.col("customer_id")
}

func hasKeyword(c *gin.Context) bool { return c.Query("_keyword") != "" }

// listQuery — query nền của màn list (câu trang + export): scope chi nhánh, JOIN order_service_used, bộ lọc.
func (s spec) listQuery(c *gin.Context, db *gorm.DB) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, db, s.Table).Table(s.Table).Joins(s.joinServiceUsed())
	return s.listFilters(c, db, q)
}

// listFilters — bộ lọc DUY NHẤT của màn list, dùng chung cho câu tổng, câu trang và export.
// Có từ khoá ⇒ thêm LEFT JOIN customer; caller phải đã JOIN order_service_used (vế `order_service_used.code`).
// S-10: kèm phạm vi telesale (khách mình phụ trách); lỗi đọc phạm vi ⇒ query mang lỗi, không trả dòng.
func (s spec) listFilters(c *gin.Context, db *gorm.DB, q *gorm.DB) *gorm.DB {
	q = scope.WhereCustomerID(c, db, q, s.col("customer_id"))
	q = query.ApplyFilter(c, q, "confirm_time", s.col("confirm_time"))
	q = query.ApplyFilter(c, q, s.MemberCol, s.col(s.MemberCol))
	q = query.ApplyFilter(c, q, "branch_id", s.col("branch_id"))
	if !hasKeyword(c) {
		return q
	}
	q = q.Joins(s.joinCustomer())
	return query.ApplyKeywordFilter(c, q, keywordCols...)
}

// listOrder — ORDER BY của màn list (mặc định confirm_time DESC + tiebreaker id).
func (s spec) listOrder(c *gin.Context) string {
	return query.SortClause(c, s.col("confirm_time")+" DESC", s.col("confirm_time"), s.col("id"))
}

// listTotals — dòng tổng của màn list: số dòng + tổng cột chính (Sums[0]). Một câu, không ORDER BY;
// JOIN order_service_used chỉ khi từ khoá cần.
type listTotals struct {
	Total int64           `gorm:"column:total"`
	Sum   decimal.Decimal `gorm:"column:total_sum"`
}

func (s spec) listTotals(c *gin.Context, db *gorm.DB) (listTotals, error) {
	var t listTotals
	aq := tenant.ScopedDBNullBranch(c, db, s.Table).Table(s.Table)
	if hasKeyword(c) {
		aq = aq.Joins(s.joinServiceUsed())
	}
	err := s.listFilters(c, db, aq).
		Select("COUNT(*) AS total, COALESCE(SUM(" + s.col(s.Sums[0].Col) + "), 0) AS total_sum").
		Scan(&t).Error
	return t, err
}

// ── Màn tổng hợp ──

// summaryBase — query nền màn tổng hợp (export dùng): scope chi nhánh + bộ lọc.
func (s spec) summaryBase(c *gin.Context, db *gorm.DB) *gorm.DB {
	q := tenant.ScopedDBNullBranch(c, db, s.Table).Table(s.Table)
	// S-10: phạm vi telesale — tổng hợp chỉ cộng dòng của khách mình phụ trách (cùng tập với màn list).
	q = scope.WhereCustomerID(c, db, q, s.col("customer_id"))
	q = query.ApplyFilter(c, q, "confirm_time", s.col("confirm_time"))
	q = query.ApplyFilter(c, q, "branch_id", s.col("branch_id"))
	return query.ApplyFilter(c, q, s.MemberCol, s.col(s.MemberCol))
}

// summaryPage — một trang màn tổng hợp. totals = tổng từng cột SUM (theo Alias) trên MỌI nhóm; sum = cột chính.
type summaryPage struct {
	Items  []map[string]interface{}
	Meta   pagination.Meta
	Sum    decimal.Decimal
	Totals map[string]decimal.Decimal
}

// summaryRows chạy câu GROUP BY <nhân viên> MỘT lần; dòng tổng + phân trang làm trong Go (mỗi nhân viên một
// dòng ⇒ tải hết về vẫn nhỏ).
func (s spec) summaryRows(c *gin.Context, db *gorm.DB) (summaryPage, error) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per-page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 200 {
		perPage = 20
	}

	m := s.col(s.MemberCol)
	sel := m + ", COUNT(*) as count"
	for _, sc := range s.Sums {
		sel += ", SUM(" + s.col(sc.Col) + ") as " + sc.Alias
	}
	primary := s.Sums[0].Alias
	rows := make([]map[string]interface{}, 0)
	err := s.summaryBase(c, db).
		Select(sel).
		Group(m).
		Order(primary + " DESC, " + m + " ASC").
		Find(&rows).Error
	if err != nil {
		return summaryPage{}, err
	}

	totals := make(map[string]decimal.Decimal, len(s.Sums))
	for _, sc := range s.Sums {
		totals[sc.Alias] = decimal.Zero
	}
	for _, r := range rows {
		for _, sc := range s.Sums {
			v, err := toDecimal(r[sc.Alias])
			if err != nil {
				return summaryPage{}, fmt.Errorf("%s %s: %w", s.Table, sc.Alias, err)
			}
			r[sc.Alias] = v
			totals[sc.Alias] = totals[sc.Alias].Add(v)
		}
	}

	totalCount := int64(len(rows))
	start := min((page-1)*perPage, len(rows))
	end := min(start+perPage, len(rows))
	items := make([]map[string]interface{}, 0, end-start)
	items = append(items, rows[start:end]...)

	pageCount := int(math.Ceil(float64(totalCount) / float64(perPage)))
	c.Header("X-Pagination-Total-Count", strconv.FormatInt(totalCount, 10))
	c.Header("X-Pagination-Page-Count", strconv.Itoa(pageCount))
	c.Header("X-Pagination-Current-Page", strconv.Itoa(page))
	c.Header("X-Pagination-Per-Page", strconv.Itoa(perPage))

	return summaryPage{
		Items:  items,
		Meta:   pagination.Meta{TotalCount: totalCount, PageCount: pageCount, CurrentPage: page, PerPage: perPage},
		Sum:    totals[primary],
		Totals: totals,
	}, nil
}

// toDecimal — giá trị SUM đọc qua map; NULL tính 0.
func toDecimal(v interface{}) (decimal.Decimal, error) {
	switch x := v.(type) {
	case nil:
		return decimal.Zero, nil
	case []byte:
		return decimal.NewFromString(string(x))
	case string:
		return decimal.NewFromString(x)
	case int64:
		return decimal.NewFromInt(x), nil
	case decimal.Decimal:
		return x, nil
	}
	return decimal.Zero, fmt.Errorf("kiểu không hỗ trợ %T", v)
}

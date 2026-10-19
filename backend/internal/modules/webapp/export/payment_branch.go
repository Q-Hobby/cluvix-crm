package export

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/export"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tz"
)

type branchReceiptCol struct {
	key     string
	header  string
	extract func(row map[string]interface{}) interface{}
}

// branchReceiptColsFor defines the exportable columns for ExportPaymentBranch,
// keyed by FE branch-receipts displayCols[].value for column-selection qua ?columns=.
func branchReceiptColsFor(loc *time.Location) []branchReceiptCol {
	return []branchReceiptCol{
		{"customer_code", "Mã khách hàng", func(r map[string]interface{}) interface{} { return r["customer_code"] }},
		{"customer_id", "Tên khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }},
		{"serviceUsedCode", "Mã dịch vụ sử dụng", func(r map[string]interface{}) interface{} { return r["ose_code"] }},
		{"service", "Tên dịch vụ", func(r map[string]interface{}) interface{} { return r["service_name"] }},
		{"quantity", "Số lượng", func(r map[string]interface{}) interface{} { return r["ose_quantity"] }},
		{"method", "Hình thức", func(r map[string]interface{}) interface{} { return r["payment_method_name"] }},
		{"amount", "Tiền thu/chi", func(r map[string]interface{}) interface{} { return toFloat(r["amount"]) }},
		{"created_at", "Ngày lên dịch vụ", func(r map[string]interface{}) interface{} { return fmtUnixDateTime(r["created_at"], loc) }},
		{"branch_id", "Chi nhánh", func(r map[string]interface{}) interface{} { return r["branch_name"] }},
		{"address", "Địa chỉ khách hàng", func(r map[string]interface{}) interface{} { return r["customer_address"] }},
	}
}

// ExportPaymentBranch exports branch receipts.
// Sheet 1 = tổng hợp. Nếu có >= 2 chi nhánh → thêm 1 sheet/chi nhánh.
// Fallback sang CSV 1 sheet khi tổng row >= 10 000.
// countRows đếm ĐÚNG tập dòng của q (handler dùng câu tổng COUNT+SUM không JOIN, rẻ hơn COUNT bọc q).
func ExportPaymentBranch(c *gin.Context, q *gorm.DB, countRows func() (int64, error)) {
	loc := tz.LocFromContext(c, q.Session(&gorm.Session{NewDB: true}))
	// customer và ose đã join sẵn bởi buildFilters của caller
	eq := q.Session(&gorm.Session{}).
		Select(`customer.code AS customer_code,
			customer.fullname AS customer_fullname,
			customer.address AS customer_address,
			ose.code AS ose_code,
			svc.name AS service_name,
			ose.quantity AS ose_quantity,
			pm.name AS payment_method_name,
			payment.amount,
			payment.created_at,
			br.name AS branch_name`).
		Joins("LEFT JOIN service svc ON svc.id = ose.service_id").
		Joins("LEFT JOIN payment_method pm ON pm.id = payment.method").
		Joins("LEFT JOIN branch br ON br.id = payment.branch_id")

	allCols := branchReceiptColsFor(loc)
	cols := allCols
	if colParam := c.Query("columns"); colParam != "" {
		selected := map[string]bool{}
		for _, k := range strings.Split(colParam, ",") {
			selected[strings.TrimSpace(k)] = true
		}
		var filtered []branchReceiptCol
		for _, col := range allCols {
			if selected[col.key] {
				filtered = append(filtered, col)
			}
		}
		if len(filtered) > 0 {
			cols = filtered
		}
	}

	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = col.header
	}

	url, err := export.ExportMultiSheet(eq, export.MultiSheetConfig{
		Config: export.Config{
			FileName:    "BaoCao_ChiNhanh",
			CountRows:   countRows,
			Headers:     headers,
			ExportDir:   "./storage/export/",
			DownloadURL: DownloadURL,
			OwnerID:     middleware.GetUserID(c),
		},
		TotalSheetName:        "Tổng hợp",
		MinGroupsForSubSheets: 2,
	}, func(row map[string]interface{}) string {
		// Group rows theo tên chi nhánh
		if name, ok := row["branch_name"].(string); ok && name != "" {
			return export.SafeSheetName(name)
		}
		return "Chưa có chi nhánh"
	}, func(row map[string]interface{}) []interface{} {
		data := make([]interface{}, len(cols))
		for i, col := range cols {
			data[i] = col.extract(row)
		}
		return data
	})

	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

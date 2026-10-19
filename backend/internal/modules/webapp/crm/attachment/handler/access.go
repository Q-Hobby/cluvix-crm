package handler

import (
	"strconv"

	customerEntity "erpCluvix/internal/modules/webapp/crm/customer/dao"
	customerScope "erpCluvix/internal/modules/webapp/crm/customer/scope"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// customerVisible — user hiện tại có XEM được khách customerID không, CÙNG phạm vi với
// GetCustomer (ScopedDBNullBranchSkipBranch(..., true) + scope telesale "khách mình phụ trách").
//
// File đính kèm là dữ liệu của khách: ai không mở được hồ sơ khách thì cũng không được liệt kê,
// tải, tải lên hay xoá file của khách đó. Lỗi đọc scope ⇒ trả error (fail-closed, caller trả 500).
func (h *Handler) customerVisible(c *gin.Context, customerID uint) (bool, error) {
	if customerID == 0 {
		return false, nil
	}
	q := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "customer", true).Table("customer").
		Where("customer.id = ?", customerID)
	q, err := customerScope.Apply(c, h.DB, q)
	if err != nil {
		return false, err
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// downloadPath — đường tải file, TƯƠNG ĐỐI so với gốc API (environment.apiUrl của FE đã kết thúc
// bằng `/api`): FE gọi `${apiUrl}${download_path}` bằng HttpClient (kèm JWT), responseType blob.
func downloadPath(id uint) string {
	return "/crm/attachment/" + strconv.FormatUint(uint64(id), 10) + "/download"
}

// present — JSON trả FE cho 1 attachment. KHÔNG trả `file_url` (key nội bộ của kho private /
// URL public cũ) — FE chỉ dùng download_path.
func present(att customerEntity.Attachment) map[string]interface{} {
	return map[string]interface{}{
		"id":            att.ID,
		"ref_type":      att.RefType,
		"ref_id":        att.RefId,
		"customer_id":   att.CustomerID,
		"branch_id":     att.BranchID,
		"file_name":     att.FileName,
		"file_type":     att.FileType,
		"file_size":     att.FileSize,
		"note":          att.Note,
		"sort_order":    att.SortOrder,
		"created_by":    att.CreatedBy,
		"created_at":    att.CreatedAt,
		"download_path": downloadPath(att.ID),
	}
}

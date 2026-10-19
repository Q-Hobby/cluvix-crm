package handler

import (
	"net/http"
	"strconv"

	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// ListByCustomer GET /crm/customer/:id/attachments?page=1&per-page=20&file_type=file|audio|video&ref_type=&ref_id=
// Trả attachment của 1 khách hàng, phân trang, filter theo nhóm file_type — thêm filter tuỳ chọn
// ref_type/ref_id (emr-print-forms-digitalization story-06) để lấy đúng attachment gắn với 1
// resource cụ thể (vd 1 lịch hẹn) thay vì toàn bộ file của khách hàng.
//
// Mỗi item có `download_path` (tải qua backend có auth); KHÔNG còn `file_url`.
func (h *Handler) ListByCustomer(c *gin.Context) {
	customerID, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	visible, err := h.customerVisible(c, customerID)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	if !visible {
		response.NotFound(c)
		return
	}

	q := tenant.ScopedDBNullBranch(c, h.DB, "attachment").
		Table("attachment").
		Select("id, ref_type, ref_id, customer_id, branch_id, file_name, file_type, file_size, note, sort_order, created_by, created_at").
		Where("customer_id = ?", customerID).
		Order("id DESC")

	// Filter nhóm: file | audio | video
	switch c.Query("file_type") {
	case "audio":
		q = q.Where("file_type IN ?", audioTypes)
	case "video":
		q = q.Where("file_type IN ?", videoTypes)
	case "file":
		q = q.Where("file_type NOT IN ?", append(audioTypes, videoTypes...))
	}

	if refType := c.Query("ref_type"); refType != "" {
		q = q.Where("ref_type = ?", refType)
	}
	if refIDStr := c.Query("ref_id"); refIDStr != "" {
		// Parse ra số TRƯỚC khi vào query: MySQL ép '5abc' → 5 (warning 1292 bị nuốt) ⇒ id rác lọc
		// trúng file của đối tượng 5. Không parse được ⇒ 404 (nhất quán với helpers.ParseID).
		refID, err := strconv.ParseUint(refIDStr, 10, 31)
		if err != nil || refID == 0 {
			response.NotFound(c)
			return
		}
		q = q.Where("ref_id = ?", refID)
	}

	items := make([]map[string]interface{}, 0)
	result := pagination.Paginate(c, q, &items)
	for _, it := range items {
		if id := helpers.ToInt64(it["id"]); id > 0 {
			it["download_path"] = downloadPath(uint(id))
		}
	}
	c.JSON(http.StatusOK, result)
}

var audioTypes = []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/aac", "audio/flac"}
var videoTypes = []string{"video/mp4", "video/quicktime", "video/x-msvideo", "video/webm", "video/x-matroska"}

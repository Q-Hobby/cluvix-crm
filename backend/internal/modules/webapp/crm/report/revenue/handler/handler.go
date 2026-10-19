// Package handler — nhóm báo cáo DOANH THU của CRM (port từ ERP accountant/report + payment +
// payment_branch): doanh thu theo ngày thu tiền, hình thức thanh toán, phiếu thu theo chi nhánh,
// giảm trừ doanh số. CRM = 1 thương hiệu/DB ⇒ không còn vế company; branch là trục scope duy nhất.
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/define"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

// requireDateRange đọc dải ngày [from, to] (unix giây) từ 2 tham số query và áp CÙNG luật với các
// báo cáo CRM khác (crm/report/handler): bắt buộc có đủ 2 đầu, to >= from, độ rộng ≤
// define.ReportMaxRangeSec. Vi phạm ⇒ tự trả 422 và ok=false.
//
// Vượt trần ⇒ 422 tường minh, KHÔNG tự cắt dải ngày: cắt im lặng = trả thiếu số liệu mà người xem
// báo cáo không có cách nào biết. Guard này đứng TRƯỚC nhánh export — export chạy đúng câu của
// list, không có lý do được miễn trần.
func requireDateRange(c *gin.Context, gteKey, lteKey string) (from, to int64, ok bool) {
	f, errF := strconv.ParseInt(c.Query(gteKey), 10, 64)
	t, errT := strconv.ParseInt(c.Query(lteKey), 10, 64)
	if errF != nil || errT != nil || t < f {
		response.Validate(c, i18n.T(c, "report_date_range_required"))
		return 0, 0, false
	}
	if t-f > define.ReportMaxRangeSec {
		response.Validate(c, i18n.T(c, "report_date_range_max_one_year"))
		return 0, 0, false
	}
	return f, t, true
}

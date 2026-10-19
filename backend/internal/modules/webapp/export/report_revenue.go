package export

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/export"
	"erpCluvix/pkg/response"
)

func ExportPaymentOrderByService(c *gin.Context, q *gorm.DB) {
	eq := q.Session(&gorm.Session{}).
		Select("t.service_id, t.serviceName, t.numService, t.totalOrderPlan, t.totalPayment, t.totalActualPayment, t.created_at")

	url, err := doExport(c, eq, export.Config{
		FileName: "Bao_cao_doanh_thu_dich_vu",
		// 1 dòng/dịch vụ ⇒ luôn nhỏ: bỏ COUNT bọc baseSQL (chạy lại cả 2 bảng dẫn xuất payment lần thứ hai).
		BoundedRows: true,
		Headers: []string{
			"Dịch vụ", "Số lượng", "Doanh số", "Doanh thu", "Thực thu", "Dự kiến thu", "Trung bình/khách",
		},
		ExportDir:   "./storage/export/",
		DownloadURL: DownloadURL,
	}, func(row map[string]interface{}) []interface{} {
		totalOrderPlan := toFloat(row["totalOrderPlan"])
		totalPayment := toFloat(row["totalPayment"])
		totalActualPayment := toFloat(row["totalActualPayment"])
		numService := toInt(row["numService"])
		var avgPerCustomer float64
		if numService > 0 {
			avgPerCustomer = totalOrderPlan / float64(numService)
		}
		return []interface{}{
			row["serviceName"],
			numService,
			totalOrderPlan,
			totalPayment,
			totalActualPayment,
			totalOrderPlan - totalPayment,
			avgPerCustomer,
		}
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, url)
}

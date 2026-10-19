package handler

import (
	"fmt"
	"net/http"

	paymentDao "erpCluvix/internal/modules/webapp/crm/payment/dao"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// GetPayment GET /payment/:id
func (h *Handler) GetPayment(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item paymentDao.Payment
	if err := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "payment", tenant.IsCrossBranchServiceEnabled(c, h.DB)).
		Where("id = ?", id).Take(&item).Error; err != nil {
		response.NotFound(c)
		return
	}
	response.Data(c, item)
}

// GetOrderPlan GET /payment/get-order-plan/:id
// Lấy danh sách dịch vụ chưa thanh toán hết của customer thuộc payment.
func (h *Handler) GetOrderPlan(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}

	var customerID int
	if err := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "payment", tenant.IsCrossBranchServiceEnabled(c, h.DB)).Table("payment").
		Select("customer_id").Where("id = ?", id).Scan(&customerID).Error; err != nil || customerID == 0 {
		response.Validate(c, i18n.T(c, "deposit_not_found"))
		return
	}

	var rows []map[string]interface{}
	tx := tenant.ScopedDBNullBranchSkipBranch(c, h.DB, "order_service_used", tenant.IsCrossBranchServiceEnabled(c, h.DB), "osu").
		Table("order_service_used AS osu").
		Select("osu.id, s.name as service_name, osu.totalEnd, COALESCE(osu.payment, 0) as payment").
		Joins("LEFT JOIN service s ON s.id = osu.service_id").
		Where("osu.customer_id = ? AND osu.isLock = 1", customerID).Scan(&rows)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	helpers.ParseDecimalFieldsForTables(rows, "order_service_used")

	var data []gin.H
	for _, r := range rows {
		totalEnd := helpers.ToDecimal(r["totalEnd"])
		payment := helpers.ToDecimal(r["payment"])
		money := totalEnd.Sub(payment)
		if !money.IsPositive() {
			continue
		}
		serviceName := helpers.ToString(r["service_name"])
		data = append(data, gin.H{
			"label":  fmt.Sprintf("%s - %s", serviceName, helpers.FormatDecimal(h.DB, money)),
			"value":  r["id"],
			"amount": money,
		})
	}

	if len(data) == 0 {
		response.Validate(c, i18n.T(c, "customer_no_orders"))
		return
	}
	c.JSON(http.StatusOK, data)
}

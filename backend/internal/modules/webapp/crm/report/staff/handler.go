package staff

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/internal/deps"
	"erpCluvix/internal/middleware"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/internal/phoneutil"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/handlerutil"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
)

// Handler — báo cáo nhân viên (doanh số + hoa hồng tư vấn viên).
type Handler struct {
	DB *gorm.DB
}

// Register gắn route dưới group `/crm` đã có auth: `/crm/report/staff/...`.
func Register(rg *gin.RouterGroup, d *deps.Deps) {
	h := &Handler{DB: d.DB}
	// Export (`?_export=1`) chặn thật theo manageCrmReport (S-7) — xem define.PermManageCrmReport.
	g := rg.Group("/report/staff", middleware.RequireExportPerm(d.RbacSvc, define.PermManageCrmReport))
	g.GET("/revenue-telesale", h.RevenueTelesale)
	g.GET("/revenue-telesale/summary", h.RevenueTelesaleSummary)
	g.GET("/commission-telesale", h.CommissionTelesale)
	g.GET("/commission-telesale/summary", h.CommissionTelesaleSummary)
}

// listSelectRevenue / listSelectCommission — cột FE màn chi tiết cần; customer + dịch vụ attach sau.
const listSelectRevenue = `order_revenue_telesale.id,
	order_revenue_telesale.customer_id,
	order_revenue_telesale.order_plan_id,
	order_revenue_telesale.order_service_used_id,
	order_revenue_telesale.payment_id,
	order_revenue_telesale.revenue_id,
	order_revenue_telesale.branch_id,
	order_revenue_telesale.amount,
	order_revenue_telesale.revenue,
	order_revenue_telesale.confirm_time,
	order_service_used.service_id,
	order_service_used.code as service_used_code`

const listSelectCommission = `order_commission_telesale.id,
	order_commission_telesale.customer_id,
	order_commission_telesale.order_plan_id,
	order_commission_telesale.order_service_used_id,
	order_commission_telesale.revenue_id,
	order_commission_telesale.branch_id,
	order_commission_telesale.commissionTelesale,
	order_commission_telesale.amount,
	order_commission_telesale.confirm_time,
	order_service_used.service_id,
	order_service_used.code as service_used_code`

// RevenueTelesale GET /crm/report/staff/revenue-telesale — chi tiết doanh số tư vấn viên (ghi lúc thanh toán,
// độc lập với hoa hồng). `_export=1` ⇒ Excel.
func (h *Handler) RevenueTelesale(c *gin.Context) {
	h.list(c, revenueTelesale, listSelectRevenue, "totalRevenue", exportHandlers.ExportRevenueTelesale)
}

// CommissionTelesale GET /crm/report/staff/commission-telesale — chi tiết hoa hồng tư vấn viên.
func (h *Handler) CommissionTelesale(c *gin.Context) {
	h.list(c, commissionTelesale, listSelectCommission, "totalCommission", exportHandlers.ExportTelesaleList)
}

// RevenueTelesaleSummary GET /crm/report/staff/revenue-telesale/summary — tổng hợp doanh số theo tư vấn viên.
func (h *Handler) RevenueTelesaleSummary(c *gin.Context) {
	if c.Query("_export") == "1" {
		exportHandlers.ExportRevenueTelesaleSummary(c, revenueTelesale.summaryBase(c, h.DB))
		return
	}
	sp, ok := h.summary(c, revenueTelesale)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":        sp.Items,
		"_meta":        sp.Meta,
		"totalRevenue": sp.Sum,
	})
}

// CommissionTelesaleSummary GET /crm/report/staff/commission-telesale/summary — tổng hợp hoa hồng theo tư vấn viên.
func (h *Handler) CommissionTelesaleSummary(c *gin.Context) {
	if c.Query("_export") == "1" {
		exportHandlers.ExportTelesaleCommission(c, commissionTelesale.summaryBase(c, h.DB))
		return
	}
	sp, ok := h.summary(c, commissionTelesale)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":           sp.Items,
		"_meta":           sp.Meta,
		"totalCommission": sp.Sum,
		"totalAmount":     sp.Totals["totalAmount"],
	})
}

func (h *Handler) list(c *gin.Context, s spec, selectCols, totalKey string, export func(*gin.Context, *gorm.DB)) {
	q := s.listQuery(c, h.DB).Order(s.listOrder(c))
	if c.Query("_export") == "1" {
		export(c, q)
		return
	}

	totals, err := s.listTotals(c, h.DB)
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return
	}

	items := make([]map[string]interface{}, 0)
	result := pagination.PaginateWithTotal(c, q.Select(selectCols), &items, totals.Total)
	if c.IsAborted() {
		return
	}
	helpers.ParseDecimalFieldsForTables(items, s.Table)

	if len(items) > 0 {
		// Khách: FK lookup CỐ Ý không lọc chi nhánh (khách đăng ký ở chi nhánh khác vẫn phải hiện).
		handlerutil.AttachRelation(c, h.DB, items, "customer_id", "customer",
			"customer", "id", "id, code, old_code, fullname, nickname, phone, source_id",
			handlerutil.CollectIDs(items, "customer_id"))
		phoneutil.MaskNestedCustomerPhone(c, h.DB, items, "customer")
		handlerutil.AttachRelation(c, h.DB, items, "order_service_used_id", "orderServiceUsed",
			"order_service_used", "id", "id, code, service_id",
			handlerutil.CollectIDs(items, "order_service_used_id"))
		// revenue_id KHÔNG attach user_profile — FE gắn tên/ảnh tư vấn viên từ cache IndexedDB.
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"_meta":  result.Meta,
		totalKey: totals.Sum,
	})
}

func (h *Handler) summary(c *gin.Context, s spec) (summaryPage, bool) {
	sp, err := s.summaryRows(c, h.DB)
	if pagination.IsQueryError(err) {
		pagination.FailQuery(c, err)
		return summaryPage{}, false
	}
	if len(sp.Items) > 0 {
		handlerutil.AttachRelation(c, h.DB, sp.Items, s.MemberCol, "telesale",
			"user_profile", "user_id", "user_id, fullname, avatar",
			handlerutil.CollectIDs(sp.Items, s.MemberCol))
	}
	return sp, true
}

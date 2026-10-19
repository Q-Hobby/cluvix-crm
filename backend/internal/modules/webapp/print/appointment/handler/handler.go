package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/tenant"
	"erpCluvix/pkg/tz"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

const apptPrintLimit = 2000

// Cột FE print template thực sự dùng (audit từ print-appointment-byhour-list.html,
// plus applyData() trong .ts)
const apptPrintSelect = `appointment.id, appointment.appointment_date, appointment.status,
	appointment.content, appointment.branch_id,
	IF(c.fullname IS NOT NULL AND c.fullname != '', c.fullname, c.nickname) as customer_name,
	b.name as branch_name,
	t.name as appointment_type_name,
	s.name as service_name`

// PrintAppointmentByhourList GET /api/print/appointment-byhour-list
func (h *Handler) PrintAppointmentByhourList(c *gin.Context) {
	loc := tz.LocFromContext(c, h.DB)
	day := c.DefaultQuery("appointment_date", time.Now().In(loc).Format("2006-01-02"))
	fromT, toT := tz.DayRange(loc, day)

	q := tenant.ScopedDBNullBranch(c, h.DB, "appointment").
		Table("appointment").
		Select(apptPrintSelect).
		Joins("INNER JOIN customer c ON c.id = appointment.customer_id").
		Joins("INNER JOIN branch b ON b.id = appointment.branch_id").
		Joins("INNER JOIN appointment_type t ON t.id = appointment.appointment_type_id").
		Joins("LEFT JOIN service s ON s.id = appointment.service_id").
		Where("appointment.appointment_date BETWEEN ? AND ?", fromT, toT).
		Where("appointment.status != 4")

	var items []map[string]interface{}
	tx := q.Order("appointment.branch_id ASC, appointment.appointment_date ASC").Limit(apptPrintLimit).Find(&items)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}

	if len(items) == apptPrintLimit {
		log.Printf("[PrintAppointmentByhourList] hit limit %d for day=%s — data may be truncated",
			apptPrintLimit, day)
	}
	if items == nil {
		items = make([]map[string]interface{}, 0)
	}

	helpers.ParseJSONFields(items)

	c.JSON(http.StatusOK, gin.H{
		"data": items,
	})
}

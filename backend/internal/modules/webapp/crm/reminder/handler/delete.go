package handler

import (
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// DeleteReminder DELETE /crm/reminder/:id
// Tenant-scoped DELETE: out-of-scope → RowsAffected=0 → 404 (silent IDOR).
func (h *Handler) DeleteReminder(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	result := tenant.ScopedDBNullBranch(c, h.DB, "reminder").Table("reminder").
		Where("id = ?", id).Delete(nil)
	if result.Error != nil {
		response.Validate(c, result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		response.NotFound(c)
		return
	}
	response.Deleted(c)
}

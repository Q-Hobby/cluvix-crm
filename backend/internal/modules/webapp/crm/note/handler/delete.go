package handler

import (
	"erpCluvix/internal/middleware"
	noteDao "erpCluvix/internal/modules/webapp/crm/note/dao"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// DeleteNote DELETE /crm/note/:id
func (h *Handler) DeleteNote(c *gin.Context) {
	// Parse id ra số — không parse được thì 404, KHÔNG để MySQL tự ép kiểu trên đường XOÁ.
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item noteDao.Note
	if err := tenant.ScopedDBNullBranch(c, h.DB, "note").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if !h.canDeleteNote(middleware.GetUserID(c), &item) {
		response.Forbidden(c)
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Deleted(c)
}

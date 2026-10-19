package handler

import (
	"net/http"

	dao "erpCluvix/internal/modules/webapp/history/history_export/dao"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/query"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) ListHistoryExport(c *gin.Context) {
	q := h.DB.Model(&dao.HistoryExport{}).Order("id DESC")
	if s := c.Query("filter[positionExport]"); s != "" {
		q = q.Where("positionExport = ?", s)
	}
	// D-1: FE có ô tìm kiếm _keyword nhưng trước đây không có cột nào đọc nó — gõ vào
	// không đổi kết quả. positionExport là cột text duy nhất, hiện ở cột "Vị trí" trên UI.
	q = query.ApplyKeywordFilter(c, q, "positionExport")
	var items []dao.HistoryExport
	c.JSON(http.StatusOK, pagination.Paginate(c, q, &items))
}

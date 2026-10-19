package handler

import (
	"net/http"
	"strconv"
	"strings"

	"erpCluvix/internal/middleware"
	auditDao "erpCluvix/internal/modules/webapp/auth/audit/dao"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/pagination"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	DB      *gorm.DB
	RbacSvc *authSvc.RbacService
}

type Handler struct{ Deps }

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) dao() *auditDao.Dao { return &auditDao.Dao{DB: h.DB} }

// List trả nhật ký bảo mật (paginated + filter). Gate super-user server-side (config module không
// có permission middleware). Story-58: bỏ scope company — nhật ký toàn DB (1 thương hiệu/DB).
func (h *Handler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == 0 || h.RbacSvc == nil || !h.RbacSvc.IsSuperUser(userID) {
		c.JSON(http.StatusForbidden, gin.H{"message": i18n.T(c, "forbidden")})
		return
	}

	f := auditDao.AuditFilter{
		UserID: parseUintQuery(c, "user_id"),
		Event:  c.Query("event"),
		Events: parseEventsQuery(c, "events"),
		From:   parseInt64Query(c, "from"),
		To:     parseInt64Query(c, "to"),
	}

	var rows []map[string]any
	result := pagination.Paginate(c, h.dao().Query(f), &rows)
	c.JSON(http.StatusOK, result)
}

func parseUintQuery(c *gin.Context, key string) uint {
	if v, err := strconv.ParseUint(c.Query(key), 10, 64); err == nil {
		return uint(v)
	}
	return 0
}

func parseInt64Query(c *gin.Context, key string) int64 {
	if v, err := strconv.ParseInt(c.Query(key), 10, 64); err == nil {
		return v
	}
	return 0
}

// parseEventsQuery tách "a,b,c" → []string, trim, bỏ rỗng.
func parseEventsQuery(c *gin.Context, key string) []string {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

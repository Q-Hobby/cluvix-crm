package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"net/http"
	"strconv"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/config/options/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

// Deps + Handler — module-local (Plan 06). Full Deps để preserve h.Cache compatibility.
type Deps struct {
	DB    *gorm.DB
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

// GET /config/options/get-options?ids=24,25,26 → lấy options; ?ids= filter theo id (optional,
// backward-compatible — không truyền vẫn trả toàn bộ như cũ vì option-crm.ts cần full list để
// match theo cả `name` lẫn `id`, không thể filter trước theo id cố định).
func (h *Handler) GetOptions(c *gin.Context) {
	q := h.DB.Order("SortOrder DESC")
	if idsParam := strings.TrimSpace(c.Query("ids")); idsParam != "" {
		ids := make([]int, 0)
		for _, s := range strings.Split(idsParam, ",") {
			if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			q = q.Where("id IN ?", ids)
		}
	}
	var items []dao.UserOption
	q.Find(&items)
	c.JSON(http.StatusOK, items)
}

// GET /config/options/get-option/:id → lấy 1 option theo name (id = name constant)
func (h *Handler) GetOption(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.UserOption
	if err := h.DB.Where("name = ?", id).First(&item).Error; err != nil {
		response.Validate(c, i18n.T(c, "option_not_found"))
		return
	}
	response.Data(c, item)
}

// PUT /config/options/update-option/:id → update option value
func (h *Handler) UpdateOption(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}

	// FE có thể gửi {options: 0} hoặc trực tiếp body là object
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}

	// Lấy giá trị options từ body
	var optVal int
	if v, ok := body["options"]; ok {
		optVal = toIntValue(v)
	} else {
		response.Validate(c, i18n.T(c, "options_required"))
		return
	}

	var item dao.UserOption
	if err := h.DB.First(&item, id).Error; err != nil {
		response.Validate(c, i18n.T(c, "option_not_found"))
		return
	}

	now := time.Now().Unix()
	item.Options = optVal
	item.UpdatedAt = now
	item.UpdatedBy = new(int(middleware.GetUserID(c)))

	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	dexie.BumpDexieTable(h.DB, "userOptions")

	c.JSON(http.StatusOK, item)
}

func toIntValue(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case bool:
		if val {
			return 1
		}
		return 0
	case int:
		return val
	default:
		return 0
	}
}

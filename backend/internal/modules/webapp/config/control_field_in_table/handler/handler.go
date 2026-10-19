package handler

import (
	"erpCluvix/pkg/cache"

	"gorm.io/gorm"

	"encoding/json"

	"erpCluvix/internal/modules/webapp/config/control_field_in_table/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
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

var defaultCustomerFields = []map[string]interface{}{
	{"field_name": "fullname", "field_label": "Họ và tên", "edit": false, "create": false, "update": false},
	{"field_name": "nickname", "field_label": "Biệt danh", "edit": false, "create": false, "update": false},
	{"field_name": "gender", "field_label": "Giới tính", "edit": false, "create": false, "update": false},
	{"field_name": "phone", "field_label": "Số điện thoại", "edit": false, "create": false, "update": false},
	{"field_name": "category", "field_label": "Nhóm khách hàng", "edit": false, "create": false, "update": false},
	{"field_name": "type_service_id", "field_label": "Nhu cầu dịch vụ", "edit": false, "create": false, "update": false},
	{"field_name": "branch_id", "field_label": "Chi nhánh", "edit": false, "create": false, "update": false},
	{"field_name": "source_id", "field_label": "Nguồn", "edit": false, "create": false, "update": false},
	{"field_name": "payment_user_id", "field_label": "Tư vấn viên", "edit": false, "create": false, "update": false},
	{"field_name": "birthday", "field_label": "Ngày sinh", "edit": false, "create": false, "update": false},
	{"field_name": "facebook", "field_label": "Facebook", "edit": false, "create": false, "update": false},
	{"field_name": "zalo", "field_label": "Zalo", "edit": false, "create": false, "update": false},
	{"field_name": "viber", "field_label": "Viber", "edit": false, "create": false, "update": false},
	{"field_name": "email", "field_label": "Email", "edit": false, "create": false, "update": false},
	{"field_name": "instagram", "field_label": "Instagram", "edit": false, "create": false, "update": false},
	{"field_name": "chat_message", "field_label": "Chat message", "edit": false, "create": false, "update": false},
	{"field_name": "loai_message", "field_label": "Loại message", "edit": false, "create": false, "update": false},
	{"field_name": "country_id", "field_label": "Quốc gia", "edit": false, "create": false, "update": false},
	{"field_name": "province_id", "field_label": "Tỉnh/Thành phố", "edit": false, "create": false, "update": false},
	{"field_name": "ward_id", "field_label": "Phường/Xã", "edit": false, "create": false, "update": false},
	{"field_name": "address", "field_label": "Địa chỉ", "edit": false, "create": false, "update": false},
	{"field_name": "created_day", "field_label": "Ngày tạo", "edit": false, "create": false, "update": false},
}

// GET /config/control-field-in-table/:id/get-field-customer
func (h *Handler) GetFieldCustomer(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.ControlFieldInTable
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if len(item.Control) > 2 {
		// DB có thể lưu {"control": [...]} hoặc [...] trực tiếp
		raw := []byte(item.Control)
		var wrapper map[string]json.RawMessage
		if json.Unmarshal(item.Control, &wrapper) == nil {
			if inner, ok := wrapper["control"]; ok {
				raw = inner
			}
		}
		var fields []map[string]interface{}
		if err := json.Unmarshal(raw, &fields); err != nil {
			response.ServerError(c, err)
			return
		}
		response.Data(c, fields)
		return
	}
	response.Data(c, defaultCustomerFields)
}

// POST /config/control-field-in-table/:id/update-field-customer
func (h *Handler) UpdateFieldCustomer(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.ControlFieldInTable
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	// FE gửi {"control": [...]} → chỉ lưu array bên trong
	var body map[string]json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	if ctrl, ok := body["control"]; ok {
		item.Control = datatypes.JSON(ctrl)
	} else {
		raw, _ := json.Marshal(body)
		item.Control = datatypes.JSON(raw)
	}
	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "controlFieldInTables")
	response.Updated(c, nil)
}

// POST /config/control-field-in-table/:id/re-build
func (h *Handler) ReBuildFieldCustomer(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.ControlFieldInTable
	if err := h.DB.First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	item.Control = nil
	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	dexie.BumpDexieTable(h.DB, "controlFieldInTables")
	response.DataWithMessage(c, defaultCustomerFields, i18n.T(c, "deleted"))
}

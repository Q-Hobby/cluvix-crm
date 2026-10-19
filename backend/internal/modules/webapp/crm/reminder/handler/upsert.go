package handler

import (
	"encoding/json"
	"errors"
	"time"

	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/codegen"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/optlock"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// serializeJSONInMap encode mọi map/slice value thành JSON string để insert/update.
func serializeJSONInMap(body map[string]interface{}) {
	for k, v := range body {
		switch v.(type) {
		case map[string]interface{}, []interface{}:
			b, _ := json.Marshal(v)
			body[k] = string(b)
		}
	}
}

// CreateReminder POST /crm/reminder
// Generate code qua config_code lookup nếu FE không gửi, set audit fields,
// update customer.last_contact_date.
func (h *Handler) CreateReminder(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	if !response.RequireFields(c, [][2]any{
		{body["type"], "Loại nhắc"},
		{body["start_time"], "Thời gian"},
		{body["relatedUser"], "Người nhận"},
		{body["note"], "Nội dung"},
	}) {
		return
	}
	// FE cũ (tới story-43) còn gửi key thương hiệu — bảng không có cột ⇒ bỏ để không lỗi 1054.
	delete(body, "company_id")
	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))
	body["created_at"] = now
	body["updated_at"] = now
	body["created_by"] = uid
	body["updated_by"] = uid

	// Generate code nếu FE không gửi.
	if body["code"] == nil || body["code"] == "" {
		var bidPtr *uint
		if bid := uint(helpers.ToInt64(body["branch_id"])); bid > 0 {
			bidPtr = &bid
		} else {
			// FE không gửi branch_id → lấy branch_id đầu tiên có cấu hình mã nhắc việc.
			// ScopedDB enforce tenant (chi nhánh) để tránh leak chéo nếu body bị forge.
			var firstBranch uint
			tenant.ScopedDBNullBranch(c, h.DB, "config_code").Table("config_code").
				Select("branch_id").
				Where("parent_table = ?", "reminder").
				Order("branch_id ASC").Limit(1).Scan(&firstBranch)
			if firstBranch > 0 {
				bidPtr = &firstBranch
			}
		}
		code := codegen.Generate(h.DB, "reminder", bidPtr)
		if code == "" {
			response.Validate(c, i18n.T(c, "reminder_code_config_error"))
			return
		}
		body["code"] = code
	}

	// reminder KHÔNG có cột branch_id (D26e) — chỉ dùng để chọn chi nhánh sinh mã ở trên.
	delete(body, "branch_id")

	serializeJSONInMap(body)

	if err := h.DB.Table("reminder").Create(&body).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	// Update customer.last_contact_date — scoped theo tenant.
	if cid := helpers.ToInt64(body["customer_id"]); cid > 0 {
		tenant.ScopedDBNullBranch(c, h.DB, "customer").Table("customer").
			Where("id = ?", cid).Update("last_contact_date", now)
	}

	response.Created(c, body)
}

// UpdateReminder PUT /crm/reminder/:id
// Optimistic lock theo updated_at (D6, pkg/optlock): load updated_at → validate → update WHERE updated_at=? + bump.
// Ownership: chỉ created_by hoặc user nằm trong relatedUser mới được sửa — permission
// permissionUpdateReminder chỉ gate hiển thị nút ở FE, không phân biệt reminder của ai,
// nên phải enforce ownership ở đây để tránh 1 nhân viên sửa/mark-done reminder của người khác.
func (h *Handler) UpdateReminder(c *gin.Context) {
	id, okID := helpers.ParseID(c)
	if !okID {
		response.NotFound(c)
		return
	}
	userID := middleware.GetUserID(c)

	var current struct {
		UpdatedAt int64 `gorm:"column:updated_at"`
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "reminder").Table("reminder").
		Select("updated_at").
		Where("id = ? AND (created_by = ? OR JSON_CONTAINS(relatedUser, CAST(? AS JSON)))", id, userID, userID).
		First(&current).Error; err != nil {
		response.NotFound(c)
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}

	v, ok := body["updated_at"]
	if !ok {
		response.Validate(c, i18n.T(c, "version_missing"))
		return
	}
	expectedUpdatedAt := helpers.ToInt64(v)
	if expectedUpdatedAt != current.UpdatedAt {
		response.Conflict(c, errs.ErrVersionConflict.Error())
		return
	}

	delete(body, "id")
	delete(body, "created_at")
	delete(body, "created_by")

	delete(body, "company_id") // FE cũ còn gửi; bảng không có cột (xem Create)

	serializeJSONInMap(body)

	delete(body, "updated_at") // callback optlock bump GREATEST(now, updated_at+1)
	body["updated_by"] = int(userID)

	if _, err := optlock.Updates(tenant.ScopedDBNullBranch(c, h.DB, "reminder").Table("reminder").
		Where("id = ? AND (created_by = ? OR JSON_CONTAINS(relatedUser, CAST(? AS JSON)))", id, userID, userID),
		expectedUpdatedAt, body); err != nil {
		if errors.Is(err, errs.ErrVersionConflict) {
			response.Conflict(c, errs.ErrVersionConflict.Error())
			return
		}
		response.ServerError(c, err)
		return
	}

	var item map[string]interface{}
	tx := tenant.ScopedDBNullBranch(c, h.DB, "reminder").Table("reminder").
		Where("id = ?", id).Take(&item)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	h.attachCustomerToReminder(c, item)
	response.Updated(c, item)
}

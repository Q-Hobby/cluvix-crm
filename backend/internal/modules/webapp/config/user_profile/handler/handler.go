package handler

import (
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/storage"
	"errors"

	"gorm.io/gorm"

	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"erpCluvix/internal/middleware"
	hrUserDao "erpCluvix/internal/modules/webapp/hr/user/dao"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/imaging"
	"erpCluvix/pkg/optlock"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
)

// Deps + Handler — module-local (Plan 06). Full Deps để preserve h.Store/h.Cache compatibility.
type Deps struct {
	DB    *gorm.DB
	Store storage.Store
	Cache *cache.RedisCache
}

type Handler struct {
	Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

// GET /config/user-profile/:id
//
// SELF-ONLY. `:id` chỉ giữ lại để tương thích URL cũ; id khác user hiện tại → 403.
//
// Trước đây handler query thẳng `WHERE user_id = ?` từ param, KHÔNG tenant scope, KHÔNG permission
// ⇒ bất kỳ user đã đăng nhập nào cũng đọc được hồ sơ NGƯỜI KHÁC (SĐT/email/địa chỉ/ngày sinh).
//
// ⚠ Permission middleware KHÔNG cứu được endpoint này, ĐỪNG dựa vào nó:
//   - entry trong `route_perm_map.json` cho path này có `enforce_phase1: false` ⇒ `permission_enforcer`
//     KHÔNG gắn handler enforcement cho nó;
//   - và `permEnforce` mặc định = `false` (audit-mode: chỉ log `perm_would_deny`, KHÔNG chặn).
//
// Vì vậy chặn phải nằm NGAY trong handler.
//
// Cần xem hồ sơ nhân sự khác → làm endpoint riêng ở module HR với permission gate tường minh,
// KHÔNG nới lại endpoint này.
func (h *Handler) GetUserProfile(c *gin.Context) {
	self := middleware.GetUserID(c)
	// Giữ nguyên ngữ nghĩa cũ: id rỗng hoặc "0" = hồ sơ của CHÍNH MÌNH; id rác → 403.
	// Parse ra số để chuỗi kiểu "12abc" không bị MySQL ép thành 12 (warning 1292 bị nuốt).
	id, ok := helpers.ParseID(c)
	if raw := c.Param("id"); raw == "" || raw == "0" {
		id, ok = self, true
	}
	if !ok || id != self {
		response.Forbidden(c)
		return
	}
	var item hrUserDao.UserProfile
	if err := h.DB.Where("user_id = ?", id).First(&item).Error; err != nil {
		response.Validate(c, i18n.T(c, "user_not_found"))
		return
	}
	response.Data(c, withURL(item))
}

// GET /config/user-profile/get-user-profile
//
// SELF-SERVICE THUẦN: user_id LUÔN lấy từ JWT, KHÔNG nhận `?id=` từ client.
// Trước đây handler đọc `c.Query("id")` và chỉ fallback về user hiện tại khi
// rỗng ⇒ bất kỳ user đã đăng nhập nào cũng đọc được hồ sơ NGƯỜI KHÁC
// (SĐT/email/địa chỉ/ngày sinh) vì không có tenant scope lẫn permission check.
// `/config/user-profile/:id` cũng SELF-ONLY (xem docblock ở trên) — không phải đường xem hồ sơ
// người khác.
func (h *Handler) GetUserProfileSelf(c *gin.Context) {
	id := uintToStr(middleware.GetUserID(c))
	var item hrUserDao.UserProfile
	if err := h.DB.Where("user_id = ?", id).First(&item).Error; err != nil {
		response.Validate(c, i18n.T(c, "user_not_found"))
		return
	}
	response.Data(c, withURL(item))
}

// POST /config/user-profile/switch-profile
//
// Story-58 (E6/D15, 1 thương hiệu): CHỈ đổi chi nhánh đang xem. `companyActive_id` trong body (FE
// cũ còn gửi tới story-42) bị BỎ QUA — không kiểm, không ghi. Chi nhánh:
//   - `branchActive_id` > 0 PHẢI ∈ `branchIds` của user (user không có chi nhánh nào ⇒ mọi lựa chọn
//     đều bị từ chối — rỗng KHÔNG phải "không giới hạn" ở cửa này) VÀ phải là chi nhánh có thật.
//     Vế "có thật" thay cho kiểm cũ `JSON_CONTAINS(branch.companyId, companyActive_id)` (fail-closed
//     tương đương khi mọi chi nhánh thuộc thương hiệu duy nhất).
//   - vắng mặt / null / 0 ⇒ bỏ trống chi nhánh (ghi NULL) — chi nhánh KHÔNG bắt buộc.
func (h *Handler) SwitchProfile(c *gin.Context) {
	var body struct {
		BranchActiveID *int `json:"branchActive_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ValidateErr(c, err)
		return
	}

	branchIDs := middleware.GetBranchIDs(c)

	userID := middleware.GetUserID(c)
	updates := map[string]interface{}{}
	if body.BranchActiveID != nil && *body.BranchActiveID > 0 {
		if !uintSliceContains(branchIDs, uint(*body.BranchActiveID)) {
			response.Validate(c, i18n.T(c, "forbidden"))
			return
		}
		var count int64
		if err := h.DB.Table("branch").Where("id = ?", *body.BranchActiveID).Count(&count).Error; err != nil {
			response.ServerError(c, err)
			return
		}
		if count == 0 {
			response.Validate(c, i18n.T(c, "forbidden"))
			return
		}
		updates["branchActive_id"] = *body.BranchActiveID
	} else {
		// Khi pointer != nil nhưng value = 0 → set NULL (tránh FK violation)
		updates["branchActive_id"] = gorm.Expr("NULL")
	}

	if err := h.DB.Table("user_profile").Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	middleware.InvalidateUserScope(h.Cache, userID)
	response.Updated(c, nil)
}

// userProfileEditableFields — whitelist cột `user_profile` mà self-service được sửa.
//
// CỐ Ý KHÔNG có `user_id`: đó là PRIMARY KEY nhưng vẫn có json tag `user_id`, nên
// client gửi kèm sẽ dời hồ sơ của mình sang id khác. Cũng không có `updated_at`
// (khoá lạc quan — callback optlock tự bump), `code`, `departmentList`, `branchActive_id`
// (đổi qua /switch-profile, có validate quyền riêng), `companyActive_id` (story-58: bỏ khỏi session).
var userProfileEditableFields = map[string]bool{
	"fullname":        true,
	"nickname":        true,
	"birthday":        true,
	"about":           true,
	"address":         true,
	"phone":           true,
	"email":           true,
	"id_pancake":      true,
	"facebook":        true,
	"telegram":        true,
	"avatar":          true,
	"locale":          true,
	"gender":          true,
	"branchOffice_id": true,
}

// normalizeProfileValue map giá trị client gửi (JSON: string/number/nil —
// multipart: luôn string) về đúng kiểu cột.
//
// Cột pointer trong entity (`birthday`, `phone`, `email`, `gender`,
// `branchOffice_id`) → rỗng ghi NULL, KHÔNG ghi "":
//   - `phone` có UNIQUE INDEX ⇒ nhiều user cùng "" sẽ đụng nhau.
//   - `birthday` là DATE ⇒ MySQL reject "".
//
// Cột string NOT NULL còn lại → "" chính là giá trị "đã xoá".
func normalizeProfileValue(key string, v interface{}) interface{} {
	switch key {
	case "gender", "branchOffice_id":
		s := strings.TrimSpace(helpers.ToString(v))
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		return n
	case "birthday":
		s := helpers.NormalizeDateOnly(strings.TrimSpace(helpers.ToString(v)))
		if s == "" {
			return nil
		}
		return s
	case "phone", "email":
		s := strings.TrimSpace(helpers.ToString(v))
		if s == "" {
			return nil
		}
		return s
	default:
		return helpers.ToString(v)
	}
}

// POST /config/user-profile/update-user-profile
//
// Partial update THẬT theo khuôn `crm/customer` (PUT /crm/customer/:id): bind vào
// map rồi `Updates(map)` — key VẮNG = không đụng, key CÓ MẶT (kể cả ""/null) =
// ghi thật ⇒ xoá được field. Trước đây handler bind đè lên struct rồi
// `Updates(&item)`: GORM bỏ qua MỌI zero-value nên không xoá được field nào
// dù toast vẫn báo thành công.
func (h *Handler) UpdateUserProfile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var current struct {
		UpdatedAt int64  `gorm:"column:updated_at"`
		Avatar    string `gorm:"column:avatar"`
	}
	if err := h.DB.Model(&hrUserDao.UserProfile{}).
		Select("updated_at, avatar").Where("user_id = ?", userID).
		First(&current).Error; err != nil {
		response.NotFound(c)
		return
	}
	dbUpdatedAt := current.UpdatedAt
	oldAvatar := current.Avatar

	isMultipart := strings.Contains(c.ContentType(), "multipart")
	raw := make(map[string]interface{})
	if isMultipart {
		for key := range userProfileEditableFields {
			if v, ok := c.GetPostForm(key); ok {
				raw[key] = v
			}
		}
		if v, ok := c.GetPostForm("updated_at"); ok {
			raw["updated_at"] = v
		}
	} else {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			response.HandleError(c, err)
			return
		}
		raw = body
	}

	// Khoá lạc quan theo updated_at (D6): client phải gửi đúng updated_at hiện tại. Không gửi →
	// coi như khớp (giữ nguyên hành vi cũ của nhánh version: màn đổi ngôn ngữ của web không gửi).
	clientUpdatedAt := dbUpdatedAt
	if v, ok := raw["updated_at"]; ok {
		if s := strings.TrimSpace(helpers.ToString(v)); s != "" {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				response.Conflict(c, errs.ErrVersionConflict.Error())
				return
			}
			clientUpdatedAt = n
		}
	}
	if clientUpdatedAt != dbUpdatedAt {
		response.Conflict(c, errs.ErrVersionConflict.Error())
		return
	}

	updates := make(map[string]interface{}, len(raw))
	for k, v := range raw {
		if !userProfileEditableFields[k] {
			continue
		}
		updates[k] = normalizeProfileValue(k, v)
	}

	// `fullname` là NOT NULL và required ở cả web lẫn mobile — chặn xoá trắng.
	if v, ok := updates["fullname"]; ok && helpers.ToString(v) == "" {
		response.Validate(c, i18n.TF(c, "required", i18n.FieldLabel(c, "fullname")))
		return
	}

	// Upload avatar (multipart, field `avatarFile`) đè lên giá trị client gửi.
	newAvatar := oldAvatar
	if v, ok := updates["avatar"]; ok {
		// Chuỗi trong payload CHỈ được là ảnh hiện tại (FE gửi lại nguyên URL GET đã trả ⇒ so sau
		// ToKey) hoặc rỗng (xoá ảnh). Key khác ⇒ 422: trước đây nhận mọi key ⇒ gửi key logo/ảnh
		// người khác rồi lần sau gửi rỗng là xoá được file của họ. Ảnh mới chỉ đến từ `avatarFile`.
		resolved, err := imaging.ResolvePayloadImage(h.Store, oldAvatar, helpers.ToString(v))
		if err != nil {
			response.Validate(c, i18n.T(c, err.Error()))
			return
		}
		newAvatar = resolved
	}
	// OptAvatar (KHÔNG phải nil/default): avatar nhân viên phải vuông 500×500 như
	// avatar khách hàng — mọi chỗ hiển thị đều là khung tròn/vuông. Dùng chung
	// preset với CreateAvatarUser để 2 đường ghi avatar cho ra CÙNG một kết quả.
	uploadedAvatar, err := imaging.HandleFormImageRaw(c, h.Store, "avatarFile", avatarPrefix(userID), imaging.OptAvatar())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if uploadedAvatar != "" {
		newAvatar = uploadedAvatar
	}
	if newAvatar != oldAvatar {
		updates["avatar"] = newAvatar
	} else {
		// Không đổi → không SET, và cũng KHÔNG được xoá file S3 ở dưới.
		delete(updates, "avatar")
	}
	_, avatarChanged := updates["avatar"]

	if _, err := optlock.Updates(h.DB.Model(&hrUserDao.UserProfile{}).Where("user_id = ?", userID),
		dbUpdatedAt, updates); err != nil {
		// Cả 409 lẫn lỗi SQL: DB vẫn trỏ ảnh cũ ⇒ dọn ảnh mới vừa upload (nếu có), không đụng ảnh cũ.
		imaging.CleanupNewOnError(h.Store, oldAvatar, uploadedAvatar, avatarPrefix(userID))
		if errors.Is(err, errs.ErrVersionConflict) {
			response.Conflict(c, errs.ErrVersionConflict.Error())
			return
		}
		response.ServerError(c, err)
		return
	}
	// Chỉ xoá ảnh cũ khi cột `avatar` THẬT SỰ đổi trong DB. Trước đây gọi theo
	// giá trị bind nên `avatar: ""` xoá file S3 trong khi cột không đổi ⇒ DB trỏ
	// tới key vừa bị xoá, avatar hỏng vĩnh viễn.
	if avatarChanged {
		imaging.DeleteOldIfChanged(h.Store, oldAvatar, newAvatar, avatarPrefix(userID))
	}

	var item hrUserDao.UserProfile
	if err := h.DB.Where("user_id = ?", userID).First(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}

	dexie.BumpDexieTable(h.DB, "users")
	middleware.BumpUserAuthVersion(h.Cache, userID)
	response.Updated(c, withURL(item))
}

// withURL — bản sao để trả FE: avatar (KEY kho public) ghép thành URL công khai. KHÔNG dùng để lưu.
// (`cover` không upload qua kho nên giữ nguyên.)
func withURL(item hrUserDao.UserProfile) hrUserDao.UserProfile {
	item.Avatar = storage.ResolveURL(item.Avatar)
	return item
}

// POST /config/user-profile/create-avatar-user
//
// SELF-SERVICE THUẦN: user_id LUÔN lấy từ JWT — trước đây nhận `?id=` nên user
// bất kỳ ghi đè được avatar của NGƯỜI KHÁC (không scope, không permission).
func (h *Handler) CreateAvatarUser(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id := uintToStr(userID)

	// Request KHÔNG có part `image` → 422, KHÔNG trả 200 "thành công" (MOB-110).
	// `imaging.HandleFormImage` cố ý no-op khi thiếu file (nó dùng chung cho form
	// CUD nơi ảnh là optional), nên endpoint chuyên-đổi-avatar này phải tự chặn:
	// trước đây client nhận 200 + bản ghi cũ (`avatar` giữ nguyên/rỗng) ⇒ thất bại
	// upload trở nên VÔ HÌNH, user tưởng đã đổi ảnh mà server không lưu gì.
	// Chặn TRƯỚC khi query hồ sơ — request rác không phải tốn round-trip DB.
	if _, err := c.FormFile("image"); err != nil {
		if response.IsPayloadTooLarge(err) {
			response.PayloadTooLarge(c)
			return
		}
		response.Validate(c, i18n.T(c, "image_required"))
		return
	}
	// S3 chưa cấu hình là lỗi HẠ TẦNG, không phải lỗi của client: trả 5xx để
	// monitoring thấy, KHÔNG trả 422 "file không hợp lệ" (user sẽ đổi ảnh khác
	// rồi thử lại vô hạn). Mirror `emr/patient_document` CreateSignature.
	if h.Store == nil {
		response.ServerError(c, fmt.Errorf("storage chưa được cấu hình"))
		return
	}

	var item hrUserDao.UserProfile
	if err := h.DB.Where("user_id = ?", id).First(&item).Error; err != nil {
		response.Validate(c, i18n.T(c, "user_not_found"))
		return
	}
	oldAvatar := item.Avatar

	uploadedAvatar, err := imaging.HandleFormImageRaw(c, h.Store, "image", avatarPrefix(userID), imaging.OptAvatar())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if uploadedAvatar != "" {
		item.Avatar = uploadedAvatar
	}
	if item.Avatar == oldAvatar {
		// Không nhánh nào tới được đây sau 2 guard trên (file có + S3 có ⇒ URL mới
		// luôn khác, tên file gắn random suffix + timestamp). Giữ như lưới cuối:
		// im lặng báo thành công là đúng cái bug MOB-110 vừa vá.
		response.ServerError(c, fmt.Errorf("upload avatar không đổi được đường dẫn ảnh"))
		return
	}

	// Ghi avatar qua Updates (callback optlock bump updated_at + đọc lại vào item): trước đây `Save`
	// giữ nguyên khoá nên optimistic lock của UpdateUserProfile MÙ với thay đổi avatar.
	if err := h.DB.Model(&item).
		Where("user_id = ?", id).
		Updates(map[string]interface{}{
			"avatar": item.Avatar,
		}).Error; err != nil {
		imaging.CleanupNewOnError(h.Store, oldAvatar, uploadedAvatar, avatarPrefix(userID))
		response.ServerError(c, err)
		return
	}
	imaging.DeleteOldIfChanged(h.Store, oldAvatar, item.Avatar, avatarPrefix(userID))

	response.Data(c, withURL(item))
}

// GET /config/user-profile/get-overview-user
// SELF-SERVICE THUẦN: user_id LUÔN lấy từ JWT, KHÔNG nhận `?id=` từ client —
// cùng lớp lỗi với GetUserProfileSelf/CreateAvatarUser (DW-01). Ở đây phạm vi
// hẹp hơn (query đã qua tenant.ScopedDBNullBranch nên giới hạn trong tenant),
// nhưng vẫn cho xem chỉ số tương tác của ĐỒNG NGHIỆP mà không check perm.
// Không caller nào trong repo truyền `?id=` (FE `dashboard-telesale.service.ts`
// là dead code, mobile không dùng) ⇒ bịt luôn cho nhất quán.
// Cần xem overview người khác → mở route riêng có perm, đừng nới lại chỗ này.
func (h *Handler) GetOverviewUser(c *gin.Context) {
	id := uintToStr(middleware.GetUserID(c))

	from := int64(0)
	to := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour).Unix()

	if f := c.Query("_time[gte]"); f != "" {
		from = int64(parseUint(f))
	}
	if t := c.Query("_time[lte]"); t != "" {
		to = int64(parseUint(t))
	}

	var lead int64
	tenant.ScopedDBNullBranch(c, h.DB, "customer").Table("customer").Where("payment_user_id = ? AND created_day BETWEEN ? AND ?", id, from, to).Count(&lead)

	var lichMoi int64
	tenant.ScopedDBNullBranch(c, h.DB, "appointment").Table("appointment").
		Joins("LEFT JOIN appointment_type ON appointment.appointment_type_id = appointment_type.id").
		Where("appointment.revenue_id = ? AND appointment_type.isTypeRevenue = 1 AND appointment.created_at BETWEEN ? AND ?", id, from, to).
		Distinct("appointment.customer_id").Count(&lichMoi)

	var lichHen int64
	tenant.ScopedDBNullBranch(c, h.DB, "appointment").Table("appointment").
		Joins("LEFT JOIN appointment_type ON appointment.appointment_type_id = appointment_type.id").
		Where("appointment.revenue_id = ? AND appointment_type.isTypeRevenue = 1 AND appointment.appointment_date BETWEEN ? AND ?", id, from, to).
		Distinct("appointment.customer_id").Count(&lichHen)

	var khachDen int64
	tenant.ScopedDBNullBranch(c, h.DB, "appointment").Table("appointment").
		Joins("LEFT JOIN appointment_type ON appointment.appointment_type_id = appointment_type.id").
		Where("appointment.revenue_id = ? AND appointment_type.isTypeRevenue = 1 AND appointment.status IN (2,3) AND appointment.appointment_date BETWEEN ? AND ?", id, from, to).
		Distinct("appointment.customer_id").Count(&khachDen)

	var dongY int64
	tenant.ScopedDBNullBranch(c, h.DB, "appointment").Table("appointment").
		Joins("LEFT JOIN appointment_type ON appointment.appointment_type_id = appointment_type.id").
		Where("appointment.revenue_id = ? AND appointment_type.isTypeRevenue = 1 AND appointment.status IN (2,3) AND appointment.is_accepted = 1 AND appointment.appointment_date BETWEEN ? AND ?", id, from, to).
		Distinct("appointment.customer_id").Count(&dongY)

	pct := func(a, b int64) float64 {
		if b == 0 {
			return 0
		}
		return float64(a) / float64(b) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"lead":            lead,
		"lichMoi":         lichMoi,
		"lichHen":         lichHen,
		"khachDen":        khachDen,
		"dongY":           dongY,
		"lichMoiLead":     pct(lichMoi, lead),
		"khachDenLichHen": pct(khachDen, lichHen),
		"dongYKhachDen":   pct(dongY, khachDen),
	})
}

// avatarPrefix — prefix upload avatar của user; cũng là phạm vi DUY NHẤT được xoá ảnh cũ/dọn file
// (imaging.DeleteOldIfChanged/CleanupNewOnError) — key ngoài prefix này không bao giờ bị xoá.
func avatarPrefix(userID uint) string {
	return "user-profile/" + uintToStr(userID) + "/avatar/"
}

func uintToStr(id uint) string {
	return fmt.Sprintf("%d", id)
}

func uintSliceContains(ids []uint, id uint) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

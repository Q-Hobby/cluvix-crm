package handler

import (
	"encoding/json"
	"errors"
	"strings"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/modules/webapp/config/onboarding/dto"
	"erpCluvix/internal/modules/webapp/config/onboarding/service"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/imaging"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"
	"erpCluvix/pkg/seclog"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// onboardingImagePrefix — logo thương hiệu (cùng prefix với config/business_profile); cũng là
// phạm vi DUY NHẤT được dọn file khi tx lỗi.
const onboardingImagePrefix = "business_profile/"

// POST /config/onboarding/bootstrap (multipart/form-data)
//
//	company  — JSON string (dto.CompanyInput) = hồ sơ doanh nghiệp (`business_profile`), bắt buộc.
//	           Tên field form giữ nguyên cho FE cũ (story-63/43 đổi tên).
//	branch   — JSON string (dto.BranchInput), BẮT BUỘC (đổi 2026-09-19 — xem comment tại
//	           service.Validate: "bỏ qua chi nhánh" từng cho phép thiết lập xong mà không config_code)
//	logoFile — tuỳ chọn
//
// Thứ tự: super check → parse+validate → upload S3 (ngoài tx) → named lock → tx → sau tx.
// Mọi user đã đăng nhập đều tới được đây (không gate nào ở middleware chặn trước): toàn bộ
// phòng thủ nằm trong handler/service (super, "chưa có chi nhánh" kiểm lại dưới khoá, DTO allowlist).
func (h *Handler) Bootstrap(c *gin.Context) {
	uid := middleware.GetUserID(c)
	if uid == 0 {
		response.Forbidden(c)
		return
	}
	if response.RejectMultipartTooLarge(c) {
		return
	}
	isSuper, err := helpers.IsSuperUserE(h.DB, uid)
	if err != nil {
		// Lỗi DB phải là 500, không che thành 403.
		pagination.FailQuery(c, err)
		return
	}
	if !isSuper {
		response.Forbidden(c)
		return
	}

	var company dto.CompanyInput
	if err := json.Unmarshal([]byte(c.PostForm("company")), &company); err != nil {
		response.Validate(c, i18n.T(c, "onboarding_invalid_payload"))
		return
	}
	var branch *dto.BranchInput
	if raw := strings.TrimSpace(c.PostForm("branch")); raw != "" && raw != "null" {
		branch = &dto.BranchInput{}
		if err := json.Unmarshal([]byte(raw), branch); err != nil {
			response.Validate(c, i18n.T(c, "onboarding_invalid_payload"))
			return
		}
	}
	if err := service.Validate(&company, branch); err != nil {
		h.bootstrapError(c, err)
		return
	}

	logo, err := imaging.HandleFormImageRaw(c, h.Store, "logoFile", onboardingImagePrefix, nil)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	res, err := service.New(h.DB).Bootstrap(c.Request.Context(), service.BootstrapInput{
		UserID:    uid,
		Company:   &company,
		Branch:    branch,
		LogoImage: logo,
	})
	if err != nil {
		// Tx không commit ⇒ ảnh vừa upload thành mồ côi. Dọn best-effort (nil-safe, không trả lỗi).
		imaging.CleanupNewOnError(h.Store, "", logo, onboardingImagePrefix)
		h.bootstrapError(c, err)
		return
	}

	// Sau commit: cache hồ sơ doanh nghiệp (tz/tiền tệ/mã tiền tố) phải đọc lại dòng vừa ghi.
	brand.Invalidate()
	middleware.InvalidateUserScope(h.Cache, uid)
	fields := []zap.Field{}
	if res.BranchID != nil {
		fields = append(fields, zap.Uint("branch_id", *res.BranchID))
	}
	dexie.BumpDexieTable(h.DB, "branches")
	seclog.Event(c, "onboarding_bootstrap", fields...)

	response.CreatedDataWithMessage(c, gin.H{
		"branch_id": res.BranchID,
	}, i18n.T(c, "created"))
}

func (h *Handler) bootstrapError(c *gin.Context, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		response.Validate(c, i18n.T(c, ve.Key))
	case errors.Is(err, service.ErrAlreadyDone):
		response.Validate(c, i18n.T(c, "onboarding_already_done"))
	case errors.Is(err, service.ErrInProgress):
		response.Validate(c, i18n.T(c, "onboarding_in_progress"))
	default:
		// Đường GHI: ServerError rẽ 1062 (branch trùng name/code) ⇒ 422, còn lại 500.
		response.ServerError(c, err)
	}
}

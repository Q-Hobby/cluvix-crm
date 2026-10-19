package handler

import (
	"erpCluvix/internal/middleware"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/pagination"
	"erpCluvix/pkg/response"

	"github.com/gin-gonic/gin"
)

// GET /config/onboarding/status
//
// `can_bootstrap` = caller là super-user VÀ bảng `branch` rỗng (story-62: onboarding xong = đã có chi
// nhánh; hồ sơ `business_profile` luôn có sẵn dòng seed nên không dùng làm dấu). FE chỉ dùng để chọn nhánh UI
// (wizard / "liên hệ quản trị viên") — `bootstrap` tự kiểm lại dưới khoá. Non-super nhận `false`
// chứ KHÔNG 403, để FE phân nhánh được.
//
// Đếm "rỗng" trên TOÀN bảng, cố ý KHÔNG qua tenant scope: đang xét tenant CHƯA tồn tại (mô hình
// DB-per-customer ⇒ cả bảng thuộc đúng 1 khách). Đếm theo scope của user chưa có chi nhánh thì
// luôn ra 0 ⇒ cửa không bao giờ đóng.
func (h *Handler) Status(c *gin.Context) {
	// Kiểm "rỗng" TRƯỚC: lỗi SQL phải lộ ra 500 cho mọi caller, không bị che thành `false`.
	var ids []int64
	tx := h.DB.Table("branch").Select("id").Limit(1).Scan(&ids)
	if pagination.IsQueryError(tx.Error) {
		pagination.FailQuery(c, tx.Error)
		return
	}
	if len(ids) > 0 {
		response.Data(c, gin.H{"can_bootstrap": false})
		return
	}

	uid := middleware.GetUserID(c)
	if uid == 0 {
		response.Data(c, gin.H{"can_bootstrap": false})
		return
	}
	// IsSuperUserE (không phải IsSuperUser): lỗi SQL ở query quyền cũng phải lộ ra 500 như query
	// `branch` phía trên — khớp `bootstrap.go:38-43` và khớp chính comment đầu hàm. Bản nuốt lỗi
	// trả `false` ⇒ FE hiển thị nhầm nhánh "liên hệ quản trị viên" khi DB lỗi.
	isSuper, err := helpers.IsSuperUserE(h.DB, uid)
	if err != nil {
		pagination.FailQuery(c, err)
		return
	}
	response.Data(c, gin.H{"can_bootstrap": isSuper})
}

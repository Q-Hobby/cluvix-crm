// Package deps — shared app-level Deps struct.
//
// Tách ra package neutral để mỗi domain module có thể import (tránh cyclic
// import vào internal/router). Mọi domain entrypoint nhận deps.Deps qua
// signature đồng nhất: Register(api/authed *gin.RouterGroup, d deps.Deps).
package deps

import (
	"time"

	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/token"

	"gorm.io/gorm"
)

type Deps struct {
	DB *gorm.DB
	// Store — kho PUBLIC (avatar, logo, ảnh dịch vụ/chi nhánh, ảnh TinyMCE): URL công khai.
	// PrivateStore — kho PRIVATE (đính kèm khách hàng): KHÔNG có URL, chỉ đọc qua handler có auth.
	// Cả hai luôn non-nil sau bootstrap (storage.New lỗi ⇒ dừng khởi động).
	Store        storage.Store
	PrivateStore storage.Store
	// SignedURLTTL — hạn URL ký ngắn hạn xem/tải file kho private (STORAGE_SIGNED_URL_TTL, đã kẹp trần).
	SignedURLTTL time.Duration
	Cache        *cache.RedisCache
	TokenSvc     *token.Service
	RbacSvc      *authSvc.RbacService
	UserSvc      *hrSvc.UserService
	DownloadURL  string
	// Prod = cfg.IsProduction(). Cho các check chỉ áp dụng ở prod
	// (vd CSRF guard trên public logout — dev FE/BE có thể khác site).
	Prod bool
	// TotpIssuer — tên hiển thị trong app authenticator (Google Authenticator...).
	TotpIssuer string
}

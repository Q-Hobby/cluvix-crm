package define

// Onboarding — màn thiết lập ban đầu cho DB SaaS trắng (`config/onboarding`).
const (
	// OnboardingBootstrapLock — tên MySQL named lock tuần tự hoá `POST /config/onboarding/bootstrap`
	// (2 admin cùng bấm Hoàn tất trên DB rỗng ⇒ chỉ 1 bên tạo thương hiệu).
	OnboardingBootstrapLock = "onboarding_bootstrap"
	// OnboardingBootstrapLockTimeoutSec — số giây `GET_LOCK` chờ; hết hạn ⇒ 422 "đang được thiết lập".
	OnboardingBootstrapLockTimeoutSec = 5
	// MySQLNamedLockMaxLen — độ dài tối đa tên khoá của GET_LOCK (MySQL ≥5.7 báo lỗi nếu dài hơn).
	MySQLNamedLockMaxLen = 64
)

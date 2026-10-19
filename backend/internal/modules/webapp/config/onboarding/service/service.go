// Package service — nghiệp vụ thiết lập ban đầu cho DB SaaS trắng: hồ sơ doanh nghiệp
// (`business_profile`) + chi nhánh đầu tiên.
//
// Service không có `*gin.Context`: trả `error` (sentinel / *ValidationError), handler map ra response.
package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	branchDao "erpCluvix/internal/modules/webapp/config/branch/dao"
	branchHandler "erpCluvix/internal/modules/webapp/config/branch/handler"
	"erpCluvix/internal/modules/webapp/config/onboarding/dto"
	"erpCluvix/internal/modules/webapp/config/userscope"
	"erpCluvix/pkg/define"

	"gorm.io/gorm"
)

var (
	// ErrAlreadyDone — đã có chi nhánh (kiểm SAU khi giữ khoá, trong tx). Story-62: "DB trắng" =
	// bảng `branch` rỗng (hồ sơ `business_profile` luôn có sẵn 1 dòng seed nên không dùng làm dấu).
	ErrAlreadyDone = errors.New("onboarding: đã có chi nhánh")
	// ErrInProgress — không lấy được named lock trong thời gian chờ (phiên khác đang thiết lập).
	ErrInProgress = errors.New("onboarding: phiên thiết lập khác đang giữ khoá")
	// ErrProfileMissing — caller không có dòng `user_profile` ⇒ không đặt được chi nhánh active.
	ErrProfileMissing = errors.New("onboarding: user không có user_profile")
)

// ValidationError — payload sai; Key là key i18n để handler trả 422.
type ValidationError struct{ Key string }

func (e *ValidationError) Error() string { return "onboarding: validate " + e.Key }

// Validate kiểm payload TRƯỚC upload/khoá.
//
// ⚠ Đường tạo chi nhánh THỨ HAI ngoài `config/branch/handler.CreateBranch` — quy tắc bắt buộc giữ
// ĐÚNG MỨC handler đó (name+code+address). Hồ sơ doanh nghiệp: name+code + business_type_id (FE form
// bắt buộc; story-62: bảng `business_profile` không có cột loại hình ⇒ chỉ validate, không lưu —
// giữ nguyên hợp đồng form FE cũ tới story-63/43). Sửa bên nào thì soát bên kia.
// Validate — chi nhánh BẮT BUỘC (đổi 2026-09-19, đảo quyết định AC6 của
// `bmad/done/onboarding-setup-wizard`): "bỏ qua chi nhánh" cho phép company tạo ra mà KHÔNG một
// chi nhánh nào ⇒ `SeedConfigCodeForBranch` không có gì để chạy ⇒ company đó không hề có row
// `config_code`. `pkg/codegen.findConfig` sau đó không còn mượn config của company khác nữa (vá
// 2026-09-19, `pkg/codegen/codegen_tenant_scope_test.go`), nên hệ quả đổi từ "sai âm thầm, mượn
// nhầm mã/serial của company khác" thành "báo lỗi rõ ràng, không tạo được record" — đúng hướng
// nhưng company vẫn kẹt, không dùng được phần mềm cho tới khi admin tự vào Cấu hình tạo chi nhánh.
// Bắt buộc ngay tại wizard để không còn company nào rơi vào trạng thái nửa vời đó.
func Validate(company *dto.CompanyInput, branch *dto.BranchInput) error {
	switch {
	case strings.TrimSpace(company.Name) == "":
		return &ValidationError{Key: "onboarding_company_name_required"}
	case strings.TrimSpace(company.Code) == "":
		return &ValidationError{Key: "onboarding_company_code_required"}
	case !define.IsValidBusinessType(company.BusinessTypeId):
		return &ValidationError{Key: "onboarding_business_type_invalid"}
	case branch == nil:
		return &ValidationError{Key: "onboarding_branch_required"}
	case strings.TrimSpace(branch.Name) == "":
		return &ValidationError{Key: "onboarding_branch_name_required"}
	case strings.TrimSpace(branch.Code) == "":
		return &ValidationError{Key: "onboarding_branch_code_required"}
	case strings.TrimSpace(branch.Address) == "":
		return &ValidationError{Key: "onboarding_branch_address_required"}
	}
	return nil
}

type Service struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Service { return &Service{DB: db} }

// BootstrapInput — payload đã Validate + URL ảnh đã upload (rỗng nếu không có).
type BootstrapInput struct {
	UserID  uint
	Company *dto.CompanyInput
	// Branch — BẮT BUỘC (đổi 2026-09-19). Con trỏ giữ nguyên để không phải sửa lại toàn bộ chỗ
	// gọi/test hiện có, nhưng Validate() và Bootstrap() đều từ chối nil — "bỏ qua chi nhánh" không
	// còn là đường hợp lệ (xem comment tại Validate).
	Branch    *dto.BranchInput
	LogoImage string
}

type BootstrapResult struct {
	BranchID *uint
}

// Bootstrap ghi hồ sơ doanh nghiệp (upsert dòng `business_profile` duy nhất) + tạo chi nhánh đầu
// tiên và gán cho chính caller, nguyên tử.
//
// Tuần tự hoá bằng MySQL named lock (không phải `SELECT … FOR UPDATE`: trên bảng rỗng InnoDB chỉ
// khoá gap, hai tx cùng đọc "rỗng" rồi deadlock). Khoá được giữ tới SAU commit ⇒ bên chờ mở tx mới
// và thấy chi nhánh của bên thắng.
func (s *Service) Bootstrap(ctx context.Context, in BootstrapInput) (BootstrapResult, error) {
	var res BootstrapResult

	// Guard lại ở tầng service (không chỉ ở Validate): handler LUÔN gọi Validate trước, nhưng nếu
	// một caller tương lai (test, cron, tool khác) gọi thẳng Bootstrap bỏ qua Validate thì vẫn
	// KHÔNG được thiết lập xong mà 0 chi nhánh — đó chính là trạng thái đã gây sự cố ARA Plus
	// 2026-09-19 (không có config_code, codegen không sinh được mã dịch vụ).
	if in.Branch == nil {
		return res, errors.New("onboarding: branch bắt buộc — gọi Validate trước Bootstrap")
	}

	release, err := s.acquireLock(ctx)
	if err != nil {
		return res, err
	}
	defer release()

	now := time.Now().Unix()
	uid := int(in.UserID)

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Kiểm "rỗng" trên TOÀN bảng (DB-per-customer), cố ý không qua tenant scope.
		var exists bool
		if err := tx.Raw("SELECT EXISTS(SELECT 1 FROM branch)").Scan(&exists).Error; err != nil {
			return err
		}
		if exists {
			return ErrAlreadyDone
		}

		if err := upsertBusinessProfile(tx, in, now, uid); err != nil {
			return err
		}

		b := in.Branch
		branch := branchDao.Branch{
			Name:      b.Name,
			Code:      b.Code,
			Address:   b.Address,
			Hotline:   b.Hotline,
			Email:     b.Email,
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: &uid,
			UpdatedBy: &uid,
		}
		if err := tx.Create(&branch).Error; err != nil {
			return err
		}
		if err := tx.Model(&branch).Update("SortOrder", branch.ID).Error; err != nil {
			return err
		}
		// Hàm tự `db.Transaction` bên trong — truyền `tx` ⇒ GORM dùng savepoint, cùng tx ngoài.
		if err := branchHandler.SeedConfigCodeForBranch(tx, branch.ID, uid); err != nil {
			return err
		}
		bid := branch.ID
		res.BranchID = &bid

		if err := userscope.AppendID(tx, in.UserID, userscope.ColumnBranchIDs, bid); err != nil {
			return err
		}

		// Có dòng profile không: SELECT trước, KHÔNG suy từ RowsAffected (= số dòng ĐỔI giá trị).
		var profileIDs []int64
		if err := tx.Table("user_profile").Where("user_id = ?", in.UserID).Limit(1).
			Pluck("user_id", &profileIDs).Error; err != nil {
			return err
		}
		if len(profileIDs) == 0 {
			return ErrProfileMissing
		}
		if err := tx.Table("user_profile").Where("user_id = ?", in.UserID).Updates(map[string]any{
			"branchActive_id": res.BranchID,
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return BootstrapResult{}, err
	}
	return res, nil
}

// acquireLock lấy named lock trên MỘT *sql.Conn riêng — GET_LOCK/RELEASE_LOCK session-scoped, qua
// pool của gorm thì acquire và release có thể rơi vào 2 connection khác nhau (khuôn
// `emr/reception/handler.acquireWalkInLock`). Hàm release an toàn để `defer`.
func (s *Service) acquireLock(ctx context.Context) (func(), error) {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return nil, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	// Tên khoá tính MỘT lần trên chính conn giữ khoá, dùng lại nguyên chuỗi cho GET_LOCK và RELEASE_LOCK.
	var dbName sql.NullString
	if err := conn.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&dbName); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !dbName.Valid || dbName.String == "" {
		_ = conn.Close()
		return nil, errNoDatabase
	}
	lockName := BootstrapLockName(dbName.String)
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)",
		lockName, define.OnboardingBootstrapLockTimeoutSec).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !got.Valid || got.Int64 != 1 {
		_ = conn.Close()
		return nil, ErrInProgress
	}
	return func() {
		// Context riêng: request có thể đã bị huỷ, RELEASE_LOCK vẫn phải chạy.
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockName)
		_ = conn.Close()
	}, nil
}

var errNoDatabase = errors.New("onboarding: connection không chọn database — không dựng được tên named lock")

// BootstrapLockName — tên named lock cho DB `dbName`.
//
// Named lock MySQL có phạm vi SERVER, không theo database ⇒ gắn tên DB (DB-per-customer) để 2 khách
// chung một instance MySQL không tuần tự hoá / nhận nhầm `in_progress` của nhau. MySQL giới hạn tên
// khoá define.MySQLNamedLockMaxLen ký tự: tên DB dài thì thay bằng sha256 hex rút gọn — vẫn tất định
// (cùng DB ⇒ cùng tên), acquire/release/test đều gọi hàm này nên không lệch nhau.
func BootstrapLockName(dbName string) string {
	prefix := define.OnboardingBootstrapLock + ":"
	if name := prefix + dbName; len(name) <= define.MySQLNamedLockMaxLen {
		return name
	}
	sum := sha256.Sum256([]byte(dbName))
	return prefix + hex.EncodeToString(sum[:])[:define.MySQLNamedLockMaxLen-len(prefix)]
}

// upsertBusinessProfile ghi form bước "company" (tên field form giữ nguyên cho FE cũ — story-63/43
// đổi tên) vào dòng `business_profile` duy nhất: có dòng (seed) ⇒ UPDATE dòng id nhỏ nhất, chưa có ⇒
// INSERT. Chỉ ghi cột thuộc allowlist bootstrap; policy bảo mật (`allowed_ips`, `require_2fa*`,
// `totp_trust_days`) KHÔNG nhận từ form — giữ giá trị đang có / DB default. Field tuỳ chọn rỗng
// (timezone/tiền tệ, ảnh) ⇒ không ghi đè: giữ giá trị đang có (seed = mặc định) hoặc DB default.
//
// Caller gọi `brand.Invalidate()` SAU commit (cache hồ sơ).
func upsertBusinessProfile(tx *gorm.DB, in BootstrapInput, now int64, uid int) error {
	c := in.Company
	fields := map[string]any{
		"name":       c.Name,
		"code":       c.Code,
		"hotline":    c.Hotline,
		"address":    c.Address,
		"email":      c.Email,
		"website":    c.Website,
		"updated_at": now,
		"updated_by": uid,
	}
	optional := map[string]string{
		"timezone":        c.Timezone,
		"currency":        c.Currency,
		"currency_symbol": c.CurrencySymbol,
		"currency_locale": c.CurrencyLocale,
		"logo_image":      in.LogoImage,
	}
	for col, v := range optional {
		if strings.TrimSpace(v) != "" {
			fields[col] = v
		}
	}

	var ids []int64
	if err := tx.Table("business_profile").Order("id ASC").Limit(1).Pluck("id", &ids).Error; err != nil {
		return err
	}
	if len(ids) > 0 {
		return tx.Table("business_profile").Where("id = ?", ids[0]).Updates(fields).Error
	}
	fields["created_at"] = now
	fields["created_by"] = uid
	return tx.Table("business_profile").Create(fields).Error
}

// Package repository — generic CRUD base với tenant scope mặc định.
//
// Pattern lấy cảm hứng từ `Go/external/util/daoimpl/dao.go` (generic
// BaseInterface[T]). Adapted cho GORM + multi-tenant Cluvix:
//
//   - **Mọi** read/update/delete by id mặc định scope qua
//     `tenant.ScopedDBNullBranch` (theo CLAUDE.md B.1, bảng 4-case).
//   - **Create** KHÔNG auto-fill `company_id`/`branch_id` (theo B.1.1).
//     Caller phải set giá trị + validate qua `companyAllowed`.
//   - `Options.SkipTenant` cho phép bypass scope — chỉ dùng cho
//     cross-tenant admin op, **phải comment lý do tại call site**.
//
// Trạng thái (2026-05-16): Phase 1 của plan 09. Chưa có integration test
// chống IDOR — DO NOT use cho hot-spot module (payment, customer, treatment)
// cho tới khi integration suite cover Phase 2.
//
// Cách dùng (sẽ document đầy đủ trong README sau Phase 2):
//
//	type ReminderRepo struct { *repository.BaseRepo[crmModels.Reminder] }
//	func NewReminderRepo(db *gorm.DB) *ReminderRepo {
//	    return &ReminderRepo{repository.New[crmModels.Reminder](db, "reminder")}
//	}
//	// Trong handler:
//	item, err := h.ReminderRepo.FindByID(c, id, nil) // → ErrNotFound nếu out-of-scope hoặc missing
package repository

import (
	"errors"
	"time"

	"erpCluvix/pkg/errs"
	"erpCluvix/pkg/optlock"
	"erpCluvix/pkg/tenant"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Options điều chỉnh hành vi của một call. Truyền `nil` cho mặc định an toàn.
type Options struct {
	// SkipTenant bypass tenant scope. Default false (an toàn).
	// CHỈ set true cho cross-tenant admin op (vd super-admin list mọi company).
	// Tại call site phải có comment giải thích lý do.
	SkipTenant bool
}

// BaseInterface[T] — contract cho generic CRUD. Cho phép service depend
// abstraction thay vì *BaseRepo cụ thể → unit test pure với fake impl (B.9).
//
// Domain repo (vd LeadRepo) thường extend interface này thêm custom query:
//
//	type IRepo interface {
//	    repository.BaseInterface[entity.CustomerLead]
//	    FindByPhone(c *gin.Context, phone string) (*entity.CustomerLead, error)
//	}
type BaseInterface[T any] interface {
	FindByID(c *gin.Context, id any, opts *Options) (*T, error)
	List(c *gin.Context, dest *[]T, mods ...func(*gorm.DB) *gorm.DB) error
	Count(c *gin.Context, mods ...func(*gorm.DB) *gorm.DB) (int64, error)
	Create(c *gin.Context, m *T) error
	UpdateByID(c *gin.Context, id any, updates map[string]any, opts *Options) error
	DeleteByID(c *gin.Context, id any, opts *Options) error
	UpdateWithUpdatedAt(c *gin.Context, id any, expectedUpdatedAt int64, updates map[string]any, opts *Options) error
}

// BaseRepo[T] — generic CRUD wrapper cho 1 GORM model.
// Table name lưu để gọi tenant scope (alias-aware logic ở pkg/tenant).
type BaseRepo[T any] struct {
	DB    *gorm.DB
	Table string
}

// Compile-time assertion: *BaseRepo[T] thoả BaseInterface[T] với T cụ thể nào đó.
var _ BaseInterface[struct{}] = (*BaseRepo[struct{}])(nil)

// New construct repository cho model T trên bảng `table`.
// `table` phải match tên DB thực — dùng cho tenant.ScopedDBNullBranch lookup
// trong `branchTables` map ở pkg/tenant/scope.go.
func New[T any](db *gorm.DB, table string) *BaseRepo[T] {
	return &BaseRepo[T]{DB: db, Table: table}
}

// scope chọn root query: tenant-scoped (default) hoặc raw DB (SkipTenant).
func (r *BaseRepo[T]) scope(c *gin.Context, opts *Options) *gorm.DB {
	if opts != nil && opts.SkipTenant {
		return r.DB
	}
	return tenant.ScopedDBNullBranch(c, r.DB, r.Table)
}

// FindByID — load 1 record theo PK trong scope user. Out-of-scope = ErrNotFound
// (KHÔNG leak signal "record tồn tại nhưng bạn không có quyền").
func (r *BaseRepo[T]) FindByID(c *gin.Context, id any, opts *Options) (*T, error) {
	var m T
	err := r.scope(c, opts).Table(r.Table).Where(r.Table+".id = ?", id).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// List — load nhiều record trong scope user. `mods` áp thêm WHERE/ORDER/LIMIT.
//
//	var items []Item
//	err := repo.List(c, &items, func(q *gorm.DB) *gorm.DB {
//	    return q.Where("status = ?", 1).Order("id DESC").Limit(20)
//	})
func (r *BaseRepo[T]) List(c *gin.Context, dest *[]T, mods ...func(*gorm.DB) *gorm.DB) error {
	q := r.scope(c, nil).Table(r.Table)
	for _, mod := range mods {
		q = mod(q)
	}
	return q.Find(dest).Error
}

// Count — đếm record trong scope user, áp thêm filter qua `mods`.
func (r *BaseRepo[T]) Count(c *gin.Context, mods ...func(*gorm.DB) *gorm.DB) (int64, error) {
	q := r.scope(c, nil).Table(r.Table)
	for _, mod := range mods {
		q = mod(q)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// Create — pure INSERT. KHÔNG scope, KHÔNG validate company_id (B.1.1: caller
// trách nhiệm set company_id + gọi companyAllowed). Đây là chủ ý — auto-fill
// company_id từ JWT đã gây bug trong quá khứ.
func (r *BaseRepo[T]) Create(c *gin.Context, m *T) error {
	return r.DB.Create(m).Error
}

// UpdateByID — UPDATE qua scope: record out-of-tenant trả ErrNotFound (chống
// IDOR). `updates` map[column→value]: chỉ field listed được write.
//
// Không kiểm khoá — dùng UpdateWithUpdatedAt cho endpoint có optimistic lock.
func (r *BaseRepo[T]) UpdateByID(c *gin.Context, id any, updates map[string]any, opts *Options) error {
	if len(updates) == 0 {
		return nil
	}
	res := r.scope(c, opts).Table(r.Table).Where(r.Table+".id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// DeleteByID — DELETE qua scope. Out-of-tenant → ErrNotFound.
// GORM soft delete (nếu model có gorm.DeletedAt) tự xử lý.
func (r *BaseRepo[T]) DeleteByID(c *gin.Context, id any, opts *Options) error {
	var m T
	res := r.scope(c, opts).Table(r.Table).Where(r.Table+".id = ?", id).Delete(&m)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// UpdateWithUpdatedAt — optimistic lock theo `updated_at` (D6, pkg/optlock): WHERE updated_at = ?
// + SET updated_at = GREATEST(now, updated_at + 1) (tăng nghiêm ngặt ⇒ 2 lần ghi cùng giây vẫn
// phát hiện xung đột). Trả:
//   - ErrNotFound nếu record không tồn tại (hoặc out-of-scope).
//   - ErrVersionConflict nếu record có nhưng updated_at mismatch.
//   - nil + UPDATE thành công nếu match.
//
// Model phải có cột `updated_at` BIGINT NOT NULL. Handler dùng `response.HandleError`
// để auto map ErrVersionConflict → HTTP 409.
func (r *BaseRepo[T]) UpdateWithUpdatedAt(c *gin.Context, id any, expectedUpdatedAt int64, updates map[string]any, opts *Options) error {
	if updates == nil {
		updates = map[string]any{}
	}
	// Gán tường minh (bảng ngoài tập callback optlock cũng được bump đúng quy tắc).
	updates[optlock.Column] = optlock.BumpExpr(time.Now().Unix())

	res := r.scope(c, opts).Table(r.Table).
		Where(r.Table+".id = ? AND "+r.Table+".updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}

	// Distinguish "not found / out-of-scope" vs "updated_at mismatch":
	// nếu record vẫn exist trong scope → conflict; ngược lại → not found.
	var exists int64
	r.scope(c, opts).Table(r.Table).Where(r.Table+".id = ?", id).Count(&exists)
	if exists == 0 {
		return errs.ErrNotFound
	}
	return errs.ErrVersionConflict
}

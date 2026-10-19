package models

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"erpCluvix/pkg/codegen"

	"gorm.io/gorm"
)

// tablesRequiringCode: các bảng mà code phải được generate thành công trước khi INSERT.
// Nếu codegen trả "" (thiếu config_code entry), BeforeCreate trả error thay vì im lặng.
var tablesRequiringCode = map[string]bool{
	"order_service_used": true,
	"order_plan":         true,
	"appointment":        true,
}

// --- Timestamps: dùng chung cho cả BaseModel và SimpleModel ---

type Timestamps struct {
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at" form:"updated_at"` // khoá lạc quan 9 bảng CRM (pkg/optlock)
}

func (t *Timestamps) touchCreate() {
	now := time.Now().Unix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
}

func (t *Timestamps) touchUpdate() {
	t.UpdatedAt = time.Now().Unix()
}

// --- BaseModel: cho model nghiệp vụ (status + audit + timestamps) ---
// Tương đương MyActiveRecord + MyModel + TenantActiveRecord gộp lại.

type BaseModel struct {
	ID        uint  `gorm:"primaryKey" json:"id" form:"-"`
	Status    *int  `gorm:"default:1" json:"status,omitempty" form:"status"`
	CreatedBy *uint `json:"created_by,omitempty"`
	UpdatedBy *uint `json:"updated_by,omitempty"`
	Timestamps
}

// BaseModelWithCode: cho models có column code (service, customer, order...)
type BaseModelWithCode struct {
	BaseModel
	Code string `gorm:"size:50" json:"code,omitempty"`
}

// BaseModelNoStatus: giống BaseModel nhưng không có cột status
// Dùng cho các bảng không có cột status
type BaseModelNoStatus struct {
	ID        uint  `gorm:"primaryKey" json:"id" form:"-"`
	CreatedBy *uint `json:"created_by,omitempty"`
	UpdatedBy *uint `json:"updated_by,omitempty"`
	Timestamps
}

// BaseModelWithCodeNoStatus: giống BaseModelWithCode nhưng không có cột status
// Dùng cho các bảng không có cột status
type BaseModelWithCodeNoStatus struct {
	ID        uint  `gorm:"primaryKey" json:"id" form:"-"`
	CreatedBy *uint `json:"created_by,omitempty"`
	UpdatedBy *uint `json:"updated_by,omitempty"`
	Timestamps
	Code string `gorm:"size:50" json:"code,omitempty"`
}

// blameID đọc "userID" từ tx session — trả (nil, false) nếu chưa set HOẶC
// bằng 0. `created_by`/`updated_by` có FK constraint thật tới bảng `user`
// (xem migration baseline) — insert giá trị `0` (không phải NULL) vi phạm FK
// (lỗi 1452). Cột cho phép NULL nên "không biết ai tạo" phải để trống, KHÔNG
// gán `0` giả. Sự cố thật (2026-08-03): sync gán `userID=0` khi không resolve
// được user hệ thống, cần fallback về "để trống" thay vì chặn cứng bằng lỗi.
func blameID(tx *gorm.DB) (*uint, bool) {
	uid, ok := tx.Get("userID")
	if !ok {
		return nil, false
	}
	id := uid.(uint)
	if id == 0 {
		return nil, false
	}
	return &id, true
}

func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	b.touchCreate()
	b.setBlame(tx)
	return nil
}

func (b *BaseModelNoStatus) BeforeCreate(tx *gorm.DB) error {
	b.touchCreate()
	if id, ok := blameID(tx); ok {
		b.CreatedBy = id
		b.UpdatedBy = id
	}
	return nil
}

func (b *BaseModelNoStatus) BeforeUpdate(tx *gorm.DB) error {
	b.touchUpdate()
	if id, ok := blameID(tx); ok {
		b.UpdatedBy = id
	}
	return nil
}

func (b *BaseModelWithCode) BeforeCreate(tx *gorm.DB) error {
	b.touchCreate()
	b.setBlame(tx)
	table := tableName(tx)
	if b.Code == "" && table != "" {
		bid := resolveBranchID(tx)
		code, err := codegen.GenerateErr(tx, table, bid)
		if err != nil {
			return err // bộ đếm lỗi: KHÔNG insert với mã rỗng/bịa (xem codegen.GenerateErr)
		}
		if code != "" {
			b.Code = code
		}
	}
	if b.Code == "" && tablesRequiringCode[table] {
		return fmt.Errorf("code generation failed for %s: check config_code table", table)
	}
	return nil
}

func (b *BaseModelWithCodeNoStatus) BeforeCreate(tx *gorm.DB) error {
	b.touchCreate()
	if id, ok := blameID(tx); ok {
		b.CreatedBy = id
		b.UpdatedBy = id
	}
	table := tableName(tx)
	if b.Code == "" && table != "" {
		bid := resolveBranchID(tx)
		code, err := codegen.GenerateErr(tx, table, bid)
		if err != nil {
			return err // bộ đếm lỗi: KHÔNG insert với mã rỗng/bịa (xem codegen.GenerateErr)
		}
		if code != "" {
			b.Code = code
		}
	}
	if b.Code == "" && tablesRequiringCode[table] {
		return fmt.Errorf("code generation failed for %s: check config_code table", table)
	}
	return nil
}

func (b *BaseModelWithCodeNoStatus) BeforeUpdate(tx *gorm.DB) error {
	b.touchUpdate()
	if id, ok := blameID(tx); ok {
		b.UpdatedBy = id
	}
	return nil
}

func (b *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	b.touchUpdate()
	if id, ok := blameID(tx); ok {
		b.UpdatedBy = id
	}
	return nil
}

func (b *BaseModel) setBlame(tx *gorm.DB) {
	if id, ok := blameID(tx); ok {
		if b.CreatedBy == nil {
			b.CreatedBy = id
		}
		b.UpdatedBy = id
	}
}

// --- SimpleModel: cho model đơn giản (chỉ id + timestamps) ---
// Ví dụ: LocationProvince, Ethnicity, PaymentMethod...

type SimpleModel struct {
	ID     uint `gorm:"primaryKey" json:"id" form:"-"`
	Status *int `gorm:"default:1" json:"status,omitempty" form:"status"`
	Timestamps
}

func (b *SimpleModel) BeforeCreate(tx *gorm.DB) error {
	b.touchCreate()
	return nil
}

func (b *SimpleModel) BeforeUpdate(tx *gorm.DB) error {
	b.touchUpdate()
	return nil
}

// --- Audit user relations ---

type UserProfileRef struct {
	UserID   uint   `gorm:"primaryKey" json:"user_id"`
	Fullname string `json:"fullname"`
	Avatar   string `json:"avatar"`
}

func (UserProfileRef) TableName() string { return "user_profile" }

type WithAuditUser struct {
	UserCreated *UserProfileRef `gorm:"foreignKey:CreatedBy;references:UserID" json:"userCreated,omitempty"`
	UserUpdated *UserProfileRef `gorm:"foreignKey:UpdatedBy;references:UserID" json:"userUpdated,omitempty"`
}

// --- Dropdown ---

type DropdownItem struct {
	Label string `json:"label"`
	Value uint   `json:"value"`
}

// GetDropdownList — story-40 (D15): bỏ tham số lọc company (1 thương hiệu/DB).
func GetDropdownList(db *gorm.DB, table string) []DropdownItem {
	var items []DropdownItem
	q := db.Table(table).Select("id as value, name as label").Where("status = 1").Order("SortOrder DESC")
	q.Scan(&items)
	return items
}

type DropdownTreeItem struct {
	Label    string              `json:"label"`
	Value    uint                `json:"value"`
	ParentID *uint               `json:"parent_id,omitempty"`
	Children []*DropdownTreeItem `json:"children,omitempty"`
}

// --- Delete with FK check ---

func DeleteWithCheck(db *gorm.DB, model interface{}, id uint) error {
	// Quick FK check: try delete, if FK violation return friendly error
	stmt := &gorm.Statement{DB: db}
	_ = stmt.Parse(model)
	table := stmt.Table

	result := db.Table(table).Where("id = ?", id).Delete(nil)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "foreign key") || strings.Contains(result.Error.Error(), "1451") {
			return errors.New("fk_constraint")
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("not_found")
	}
	return nil
}

// --- Helpers ---

func tableName(tx *gorm.DB) string {
	if tx.Statement != nil && tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	stmt := &gorm.Statement{DB: tx}
	_ = stmt.Parse(tx.Statement.Model)
	return stmt.Table
}

func branchID(tx *gorm.DB) *uint {
	if v, ok := tx.Get("branchID"); ok {
		return new(v.(uint))
	}
	return nil
}

// resolveBranchID reads branch_id from gorm context or dest struct field.
func resolveBranchID(tx *gorm.DB) *uint {
	if bid := branchID(tx); bid != nil {
		return bid
	}
	if tx.Statement == nil || tx.Statement.Dest == nil {
		return nil
	}
	v := reflect.ValueOf(tx.Statement.Dest)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	f := v.FieldByName("BranchId")
	if !f.IsValid() {
		return nil
	}
	switch f.Kind() {
	case reflect.Pointer:
		if !f.IsNil() && f.Elem().CanInt() {
			if val := f.Elem().Int(); val > 0 {
				return new(uint(val))
			}
		}
	case reflect.Int, reflect.Int64:
		if val := f.Int(); val > 0 {
			return new(uint(val))
		}
	}
	return nil
}

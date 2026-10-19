# Backend Architecture Patterns

## Mục lục

- Response Format
- RESTful Routes
- Handler & Router DI Pattern (MANDATORY)
  - Handler struct (`handler.go`)
  - Router
  - Handler method
- Repository Reader Pattern (RECOMMENDED cho hot-spot services)
- DTO Pattern (RECOMMENDED cho endpoint mới)
- Layered Module Layout (RECOMMENDED cho module mới)
- SetCache Pattern
- Dao/ Pattern (Plan 08)

---


DI, Handler, Router, Repository Reader, DTO, Module Layout, SetCache, Dao pattern.

---

## Response Format
→ Xem `skill/skills/backend-hard-rules/SKILL.md` §Response Format. Wire format List: `{items: [], _meta: {totalCount, pageCount, currentPage, perPage}}`.

## RESTful Routes
```
POST   /resource              → Tạo mới
GET    /resource              → Danh sách
GET    /resource/:id          → Chi tiết
PUT    /resource/:id          → Cập nhật
DELETE /resource/:id          → Xoá
POST   /resource/:id/action   → Business action
```
Multi-type: `POST /resource/create-xxx`, `PUT /resource/:id/update-xxx`

## Handler & Router DI Pattern (MANDATORY)

### Handler struct (`handler.go`)
```go
type Deps struct {
    DB    *gorm.DB
    Store storage.Store     // kho PUBLIC (ảnh) — chỉ khi handler upload; PrivateStore cho file riêng tư
}

type Handler struct {
    Deps
    XxxSvc *services.XxxService
}

func NewHandler(d Deps) *Handler {
    return &Handler{Deps: d, XxxSvc: services.NewXxxService(d.DB)}
}
```

Reference: `internal/modules/webapp/crm/payment/handler/handler.go`, `internal/modules/webapp/auth/core/handler/handler.go`.

### Router
```go
// internal/router/router.go
type Deps struct {
    DB *gorm.DB
    Store, PrivateStore storage.Store
    Cache *cache.RedisCache
}

func Setup(cfg *config.Config, d Deps) *gin.Engine {
    ...
    registerCRMRoutes(authed, d)
}

// internal/router/crm.go
func registerCRMRoutes(authed *gin.RouterGroup, d Deps) {
    h := crmHandlers.NewHandler(crmHandlers.Deps{DB: d.DB, S3: d.S3})
    authed.GET("/customer", h.ListCustomer)
}
```

### Handler method
```go
// ❌ NEVER
db.DB.Where(...).First(&model)

// ✅ ALWAYS
h.DB.Where(...).First(&model)

// ✅ Với tenant scope
tenant.ScopedDBNullBranch(c, h.DB, "table").Where(...).First(&model)
```

`db.DB` global chỉ dùng ở `cmd/migrate`.

## Repository Reader Pattern (RECOMMENDED cho hot-spot services)

```go
// internal/modules/<domain>/<resource>/repo/repo.go
type Reader interface {
    CustomerInfo(tx *gorm.DB, customerID int) (CustomerInfo, error)
}

func NewGormReader() Reader { return gormReader{} }

type gormReader struct{}
func (gormReader) CustomerInfo(tx *gorm.DB, id int) (CustomerInfo, error) {
    var c CustomerInfo
    err := tx.Table("customer").Select("...").Where("id = ?", id).Take(&c).Error
    return c, err
}
```

Service inject:
```go
type XxxService struct {
    DB   *gorm.DB
    repo xxxRepo.Reader
}
func NewXxxService(db *gorm.DB) *XxxService {
    return &XxxService{DB: db, repo: xxxRepo.NewGormReader()}
}
func NewXxxServiceWithReader(db *gorm.DB, r xxxRepo.Reader) *XxxService {
    return &XxxService{DB: db, repo: r}
}
```

Migrated: payment (5), treatment_course (7), order_treatment_plan (6), appointment (4) = 22 Reader methods total.

## DTO Pattern (RECOMMENDED cho endpoint mới)

```go
// internal/modules/<domain>/<resource>/handler/dto.go
type CreateXxxInput struct {
    CompanyID *uint           `json:"company_id" binding:"required"`
    Amount    decimal.Decimal `json:"amount" binding:"required"`
    Note      string          `json:"note"`
}

func (in *CreateXxxInput) ToModel() *models.Xxx {
    return &models.Xxx{
        CompanyID: in.CompanyID,
        Amount:    in.Amount,
        Note:      in.Note,
    }
}

func (h *Handler) CreateXxx(c *gin.Context) {
    var input CreateXxxInput
    if err := c.ShouldBindJSON(&input); err != nil {
        response.ValidateErr(c, err); return
    }
    p, err := h.XxxSvc.CreateFromModel(input.ToModel(), userID)
    ...
}
```

Partial-update: pointer fields, nil = client omitted.

## Layered Module Layout (RECOMMENDED cho module mới)

```
internal/modules/webapp/crm/<resource>/
├── route.go
├── handler/
│   ├── handler.go, dto.go
│   └── find.go, upsert.go, delete.go
├── service/
│   ├── service.go
│   └── find.go, upsert.go, delete.go
└── repo/repo.go
```

- 3 package riêng: service không touch DB, handler không touch business rule
- Service depend `repo.IRepo` (interface) → unit test pure với fake repo
- Convention đầy đủ: `skill/skills/navigating-backend-architecture/reference/arch-modules.md`
- Pilot: `internal/modules/webapp/crm/order_plan/`

## SetCache Pattern

```go
// internal/modules/<domain>/<resource>/dao/cache.go
var cacheInstance *cache.RedisCache

func SetCache(c *cache.RedisCache) { cacheInstance = c }
func Cache() *cache.RedisCache    { return cacheInstance }
```

`bootstrap/app.go` gọi `xxxDao.SetCache(appCache)` một lần khi startup.
Reference: `pkg/dexie/`, `modules/zalo/news/dao/`.

## Dao/ Pattern (Plan 08)

```
internal/modules/{domain}/{resource}/dao/
├── entity.go   # GORM struct + TableName
└── dao.go      # IDao + Dao + BaseRepo
```

**entity.go:**
```go
package dao

import "erpCluvix/internal/models"

type {Resource} struct {
    models.BaseModel           // tenant table
    // hoặc models.SimpleModel // shared table
    Field string `gorm:"size:255" json:"field"`
}

func ({Resource}) TableName() string { return "{table_name}" }
```

**dao.go:**
```go
package dao

import (
    "erpCluvix/pkg/repository"
    "gorm.io/gorm"
)

type IDao interface {
    repository.BaseInterface[{Resource}]
}

type Dao struct {
    *repository.BaseRepo[{Resource}]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
    return &Dao{repository.New[{Resource}](db, "{table_name}")}
}
```

- KHÔNG import entity từ `internal/models/{module}/` — deprecated sau Plan 08
- Chỉ `models.BaseModel` / `models.SimpleModel` / `models.DeleteWithCheck` từ `internal/models/base.go`

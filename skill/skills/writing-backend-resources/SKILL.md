---
name: writing-backend-resources
description: >-
  Hướng dẫn quy trình 6 bước TẠO MỚI resource backend Go (entity, dao, repository, service,
  handler, router, DTO, module layout) cho ERP Cluvix. Dùng skill này TRƯỚC KHI tạo resource
  CRUD mới hoặc module mới trong internal/modules/, khi cần biết Handler DI/Router/Repository
  Reader/DTO pattern chuẩn, hoặc trước khi đụng module service (status filter 2 tầng, cascade,
  optimistic lock). KHÔNG dùng skill này để debug/sửa query đang sai — xem
  debugging-backend-queries cho việc đó.
---

# Tạo resource backend mới (Go)

> Kiến trúc DI/Handler/Router/DTO chi tiết: `reference/patterns-arch.md`
> Pattern module Service (status filter 2 tầng, cascade, optimistic lock, service_material): `reference/service.md`


> **Layout chuẩn (Plan 03 / Plan 08 + restructure 4 nhóm 2026-07-28):**
> - Resource web-app mới ở `internal/modules/webapp/<domain>/<resource>/` (client/mobile/public có nhóm riêng — xem Hard Rules)
> - Entity trong `dao/entity.go`
> - KHÔNG tạo file mới trong `internal/models/` (chỉ `base.go` ở đó)
> - `internal/handlers/` và `internal/services/` đã XOÁ (Plan 07)

---

## Bước 1 — Migration

```bash
# Tạo file BẰNG TAY — KHÔNG dùng `-cmd=create` (nó sinh kèm `.down.sql`, trái rule)
cd backend && ls migrations | sort | tail -3   # timestamp mới phải lớn hơn file mới nhất
touch migrations/$(date +%Y%m%d%H%M%S)_create_{table_name}.up.sql
```

Viết `.up.sql`:
- `CREATE TABLE` với đầy đủ columns, index, comment tiếng Việt
- `created_at`, `updated_at`: `BIGINT` (Unix), KHÔNG `DATETIME`
- `BaseModel` embed: cần `id`, `status`, `created_by`, `updated_by`, `created_at`, `updated_at` (KHÔNG còn `company_id` — D15/story-62; `branch_id` khai riêng trong entity nếu bảng có chi nhánh)
- `SimpleModel` embed: cần `id`, `status`, `created_at`, `updated_at`
- `status TINYINT(1) DEFAULT 1` nếu cần show/hide
- `sort_order INT DEFAULT 0` nếu cần sắp xếp
- **KHÔNG tạo `.down.sql`** (chi tiết: `skills/writing-mysql-migrations/SKILL.md`)

```bash
cd backend && go run cmd/migrate/main.go -cmd=up
```

---

## Bước 2 — Entity (dao/entity.go)

Template đầy đủ entity.go + dao.go → Xem `skill/skills/writing-backend-resources/reference/patterns-arch.md` §Dao/ Pattern.

- Chỉ GORM struct + TableName() — không có method, không business logic
- Embed `models.BaseModel` (tenant table) hoặc `models.SimpleModel` (shared table)
- camelCase DB column → `gorm:"column:xxx"`
- JSON column → `datatypes.JSON`, KHÔNG `string`
- Nullable fields → pointer `*Type`
- KHÔNG tạo entity ở `internal/models/{module}/`

---

## Bước 3 — Dao (dao/dao.go)

`IDao` + `Dao` + `BaseRepo` + `var _ IDao = (*Dao)(nil)` — code → Xem `skill/skills/writing-backend-resources/reference/patterns-arch.md` §Dao/ Pattern.

Bỏ qua khi: `_shared/dao` (types/constants chung) hoặc readonly view.

---

## Bước 4 — Service (service/service.go) — chỉ khi có business logic

Tạo khi có: validate business, side effects, optimistic locking.

```go
package service

import (
    dao "{domain}/{resource}/dao"
    "gorm.io/gorm"
)

type Service struct {
    db  *gorm.DB
    dao dao.IDao
}

func New(db *gorm.DB) *Service {
    return &Service{db: db, dao: dao.New(db)}
}
```

Bỏ qua nếu CRUD đơn giản không có side effect.

---

## Bước 5 — Handler (handler/handler.go)

```go
package handler

import (
    "gorm.io/gorm"
    dao "{resource_import_path}/dao"
    "erpCluvix/pkg/helpers"
    "erpCluvix/pkg/pagination"
    "erpCluvix/pkg/response"
    "erpCluvix/pkg/tenant"
    "github.com/gin-gonic/gin"
    "net/http"
    "strings"
)

type Deps struct {
    DB *gorm.DB
}

type Handler struct {
    Deps
}

func New(d Deps) *Handler { return &Handler{Deps: d} }

func (h *Handler) List{Resource}(c *gin.Context) {
    query := tenant.ScopedDBNullBranch(c, h.DB, "{table_name}").Model(&dao.{Resource}{})
    if keyword := c.Query("_keyword"); keyword != "" {
        like := "%" + strings.TrimSpace(keyword) + "%"
        query = query.Where("{searchable_column} LIKE ?", like)
        // {searchable_column} PHẢI khớp cột thật trên entity — 2 lần lỗi 1054 production
        // (warranty_code, supplier_service) vì copy nguyên "name" không có trên bảng
    }
    query = query.Order("sort_order DESC")
    var items []dao.{Resource}
    result := pagination.Paginate(c, query, &items)
    c.JSON(http.StatusOK, result)
}

func (h *Handler) Get{Resource}(c *gin.Context) { ... response.Data(c, item) }
func (h *Handler) Create{Resource}(c *gin.Context) { ... response.Created(c, item) }
func (h *Handler) Update{Resource}(c *gin.Context) { ... response.Updated(c, item) }
func (h *Handler) Delete{Resource}(c *gin.Context) { ... response.Deleted(c) }
```

- Tenant table → `tenant.ScopedDBNullBranch(c, h.DB, "{table_name}")`
- Bảng dùng chung không có `branch_id` (danh mục toàn thương hiệu) → `ScopedDBNullBranch` vẫn an toàn (bảng không đăng ký `branchTables` ⇒ không thêm điều kiện); GET/UPDATE/DELETE theo id vẫn đi qua nó theo Hard Rules
- Sau CUD config: `dexie.BumpDexieTable(h.DB, "<dexie_table_key>")`
- Update: typed DTO với pointer fields (nil = omit)
- KHÔNG import `db.DB` global

---

## Bước 6 — Route (route.go)

```go
package {resource}

import (
    "erpCluvix/internal/modules/webapp/{domain}/{resource}/handler"
    "github.com/gin-gonic/gin"
)

func RegisterRoutes(authed gin.IRouter, db *gorm.DB) {
    h := handler.New(handler.Deps{DB: db})
    g := authed.Group("/{resource-path}")
    g.GET("", h.List{Resource})
    g.GET("/:id", h.Get{Resource})
    g.POST("", h.Create{Resource})
    g.PUT("/:id", h.Update{Resource})
    g.DELETE("/:id", h.Delete{Resource})
}
```

Đăng ký trong `internal/router/` hoặc domain `router.go`.

**Màn chỉ-đọc cần hình dạng response khác → thêm `GET /:id/detail` RIÊNG:**
- KHÔNG đổi `GET /:id` đang phục vụ form sửa — đổi hình dạng ở đó là đụng luồng ghi và mọi màn khác đang tiêu thụ.
- Endpoint mới viết `Select` tường minh theo đúng field màn cần; đối chiếu schema **sau migration**, không chép cột từ baseline.
- Đăng ký sau `GET /:id` trong cùng group; thêm vào `route_perm_map.json` bằng edit additive.

---

## Layout tổng thể

```
internal/modules/webapp/{domain}/{resource}/
├── route.go
├── handler/
│   ├── handler.go        # Deps + Handler + New
│   ├── find.go           # List + Get
│   ├── upsert.go         # Create + Update
│   ├── delete.go         # Delete
│   └── dto.go            # DTO structs (optional)
├── service/
│   └── service.go        # Business logic (bỏ qua nếu CRUD thuần)
└── dao/
    ├── entity.go          # GORM struct + TableName
    └── dao.go             # IDao + Dao + BaseRepo
```

Pilot: `internal/modules/webapp/crm/order_plan/`

---

## Checklist sau khi tạo xong

- [ ] Migration đã chạy thành công
- [ ] Entity ở `dao/entity.go`, không phải `internal/models/`
- [ ] `dao/dao.go` có IDao + Dao + `var _ IDao = (*Dao)(nil)`
- [ ] Handler dùng Deps struct + `h.DB`
- [ ] List + single-record đều qua `tenant.ScopedDBNullBranch`
- [ ] Bảng có `branch_id` (là quyền sở hữu của dòng) → đã đăng ký `branchTables` trong `pkg/tenant/scope.go` (`tenantTables`/company đã xoá, story-41); verify `grep '"<table>"' pkg/tenant/scope.go` ra 1 hit (thiếu = `ScopedDBNullBranch` no-op câm lặng → IDOR; xem `skill/skills/backend-hard-rules/reference/security-patterns.md`)
- [ ] Sau CUD: `dexie.BumpDexieTable` nếu là config table
- [ ] Route đã đăng ký đủ endpoints
- [ ] `go build ./...` clean
- [ ] Không có magic string/number — constants trong `pkg/define/`

---

## FE form patterns (CRITICAL)

### Validators cho FK fields

`Validators.required` của Angular chấp nhận `0` là valid. FK = 0 → BE vỡ FK constraint.

```ts
// ❌ NEVER cho FK / select id
emr_department_id: [model.emr_department_id, Validators.required]

// ✅ ALWAYS dùng [required, min(1)]
emr_department_id: [model.emr_department_id, [Validators.required, Validators.min(1)]]
department_type:   [model.department_type,   [Validators.required, Validators.min(1)]]
```

### Visual invalid indicator

```html
<app-foo-autocomplete-form-control
  [class.ng-invalid]="isFieldInvalid('foo_id')"
  class="ng-dirty"
  formControlName="foo_id"
/>
<label for="foo_id">Tên field <span class="text-red-500">*</span></label>
```

`isFieldInvalid` ở `BaseForm`:
```ts
return !!c && c.invalid && (c.dirty || c.touched || this.submitted());
```

### BE defense-in-depth — KHÔNG phụ thuộc FE validate

```go
// Sentinel error = i18n key, KHÔNG hardcode string tiếng Việt
var errEmrDepartmentRequired = errors.New("emr_department_required") // key có trong pkg/i18n/*
func resolveDepartmentTypeForUpdates(tx *gorm.DB, updates map[string]interface{}) error {
    edid, ok := updates["emr_department_id"]
    if !ok || edid == nil { return errEmrDepartmentRequired }
    if helpers.ToInt64(edid) <= 0 { return errEmrDepartmentRequired }
    // ...auto-derive department_type
}

if err := resolveDepartmentTypeForUpdates(tx, updates); err != nil {
    if errors.Is(err, errEmrDepartmentRequired) {
        response.HandleError(c, err)  // sentinel → i18n.TryT → 422
    } else {
        response.ServerError(c, err)  // 500
    }
    return
}
```

Reference: `internal/modules/webapp/config/service/handler/handler.go` (`errEmrDepartmentRequired`).

### Data trong IndexedDB — KHÔNG attach từ BE

- BE trả raw FK id (`service_id`, `branch_id`)
- FE lookup qua helper: `servicesHelper.getServiceInfo(id)` / `branchHelper.getBranchInfo(id)`
- Nếu resource KHÔNG có IndexedDB cache → mới attach qua `attachRelation` ở handler

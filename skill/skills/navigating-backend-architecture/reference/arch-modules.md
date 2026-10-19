# Module Map

## Mục lục

- 1. Layered module layout (chuẩn hiện tại)
  - Resources với layout đặc biệt
  - Plan tham chiếu
- 2. Domain modules (`internal/modules/`)
- 3. Bootstrap & wiring (`internal/bootstrap/app.go`)
  - Flow trong `bootstrap.New(cfg)`
  - `internal/deps/deps.go` — shared Deps struct
- 4. Router structure
  - `internal/router/router.go`
  - `internal/modules/<domain>/router.go` (1 file/domain)
  - `internal/modules/<domain>/<resource>/route.go` (1 file/resource)
- 5. Handler DI pattern
- 6. Domain-level service packages (cross-resource business logic)
  - Service-to-service dependencies
- 7. Cross-module imports (`export` là sink chính)
- 8. Repository Reader pattern (Phase 4 — 5 resources)
- 9. Dao layer (Plan 08 — 140 resources)
- 10. Circular imports
- 11. Tóm tắt graph (1 cấp)
- 12. Reading order cho AI agents

---


Tài liệu cho AI agents: liệt kê modules/resources và mối phụ thuộc giữa chúng.
Tất cả số liệu đo trực tiếp trong code — không phỏng đoán.

> **Đã bỏ (2026-10-17, CRM standalone):** zalo, inventory, loyalty, med, ai, emr, accountant, marketing, care (customer-care), sale, cron, notification, workspace, omnichat, teamchat, client, public — cùng `_shared/`, `readonly/` của chúng. Chỉ còn 8 domain webapp: `auth, config, crm, export, history, hr, import, print`.

> **Cập nhật 2026-05-18** sau Plan 03 (layered module) + Plan 07 (services/ → modules/) + Plan 08 (models/ → dao/).
> `internal/handlers/` và `internal/services/` đã bị XOÁ. Mọi resource giờ ở `internal/modules/<domain>/<resource>/`.

---

## 1. Layered module layout (chuẩn hiện tại)

Mọi resource đẻ ở `internal/modules/<domain>/<resource>/`:

```
internal/modules/webapp/crm/payment/
├── route.go                  # RegisterRoutes(group, handler.Deps{...})
├── handler/                  # HTTP layer
│   ├── handler.go            # Deps + Handler struct + New(d Deps) *Handler
│   ├── find.go, upsert.go, delete.go, helpers.go
│   ├── dto.go                # DTO structs (typed input)
│   ├── main_integration_test.go
│   └── payment_http_test.go
├── service/                  # business logic (OPTIONAL — bỏ qua nếu CRUD thuần)
├── dao/                      # data access
│   ├── entity.go             # GORM struct + TableName() — KHÔNG có business logic
│   └── dao.go                # IDao interface + Dao + repository.BaseRepo[T]
├── dto/                      # DTO standalone package (HIẾM — chỉ `crm/payment` có)
└── repo/                     # Reader pattern (HIẾM — 3 hot-spot)
    └── repo.go               # Reader interface + gormReader + types
```

### Resources với layout đặc biệt

- **Domain-level shared service** (cross-resource business logic dùng chung):
  `internal/modules/<domain>/service/` — KHÔNG phải sub-resource, là package service tập trung của cả domain.
- **`_shared/`** — types/constants dùng chung giữa các resource cùng domain (hiện không domain nào còn `_shared/`).

### Plan tham chiếu

| Plan | Nội dung |
|---|---|
| Plan 03 (2026-05-16) | Layered module layout pilot (`crm/customer_lead`) — 3 layer package `handler/service/dao` + `route.go` |
| Plan 07 (2026-05-17) | `internal/services/<domain>/` → `internal/modules/<domain>/service/` (flat package) |
| Plan 08 (2026-05-18) | `internal/models/<domain>/<resource>.go` → `internal/modules/<domain>/<resource>/dao/entity.go`. `internal/models/` chỉ còn `base.go` |

---

## 2. Domain modules (`internal/modules/webapp/`)

Danh sách domain + số resource drift nhanh — nguồn chuẩn: `ls backend/internal/modules/` và `ls backend/internal/modules/<domain>/`. Bảng dưới chỉ mô tả vai trò từng domain, KHÔNG đảm bảo count/danh sách resource đầy đủ.

| Domain | Domain-level `service/`? | Mô tả |
|---|:---:|---|
| `auth` | — (rbac có sub-service) | `core` (login/logout/refresh/me), `rbac` (role/permission, `rbac/service/` + `rbac/seed/`), `audit`, `device`, `twofa` |
| `config` | ✅ (`config/service/`) | Bảng lookup/master-data (đếm: `ls internal/modules/webapp/config/`): service, service_type, unit, branch, business_profile, location_{country,province,ward}, appointment_*, customer_{category,ranks,rank_services}, payment_method, print_template, control_field_in_table, indexeddb, setting, options, … |
| `crm` | ✅ (`crm/service/`) | appointment, attachment, customer, customer_detail, dashboard, exchange_rate, note, order_plan, order_service_used, payment, reminder, report, tinymce |
| `export` | — | Flat package-level fns — `exportHandlers.ExportXxx(c, query)`. Bootstrap set `DownloadURL` + `SetDB` |
| `history` | — (log có sub-service) | `log` (audit list + service config, `log/service/`), `history_export` |
| `hr` | — (user có sub-service) | chỉ còn `user` (+ `user/service/`) |
| `import` | — | `customer` — import migration (xem `skill/skills/working-on-ops/reference/import.md`) |
| `print` | — | appointment, order_plan, payment, payment_in_day, reception_payment — package-fn handlers |

---

## 3. Bootstrap & wiring (`internal/bootstrap/app.go`)

`main.go` chỉ còn ~25 LOC — load env + gọi `bootstrap.New(cfg)` → `app.Serve(cfg)`.

### Flow trong `bootstrap.New(cfg)`

```
1. db.Connect(cfg.DSN(), cfg.IsProduction())                         → global db.DB
2. applog.Init(db.DB)                                                — log chẩn đoán → bảng app_event_log
3. Parallel goroutines:
     helpers.InitJSONColumnMap(db.DB)                                — INFORMATION_SCHEMA
     helpers.InitDecimalColumnMap(db.DB)
4. redis.NewClient(authRedis)                                       — JWT sessions DB
5. cache.New(...)                                                   — app cache DB
6. tokenSvc := token.New(...)                                        — JWT + Redis sessions
7. S3 client (optional, chỉ khi cfg.S3Key != "")
8. Wire shared globals:
     exportHandlers.DownloadURL / SetDB(db.DB)                       — export package
     dexie.SetCache(appCache)                                        — IndexedDB sync
     middleware.SetJWTSecret / SetIPAllowlistMode / SetPermEnforce
     seclog.InitSink(1000, auditDao.InsertBatch)                     — security audit sink
     validation.RegisterCustomTags()                                 — phone_vn, etc.
9. rbacSvc := &authSvc.RbacService{...}; seedPermissions(cfg, rbacSvc)   — seed catalog quyền (non-fatal trừ SEED_STRICT)
10. userSvc := &hrSvc.UserService{...}
11. router.Setup(cfg, deps.Deps{DB, S3, Cache, TokenSvc, RbacSvc, UserSvc, DownloadURL, Prod, TotpIssuer}) → *gin.Engine
12. Background: helpers.RunBackground — user_device_cleanup + security_audit_cleanup (24h/lần)
13. http.Server (ReadHeaderTimeout 10s) — graceful shutdown ở App.Serve
```

### `internal/deps/deps.go` — shared Deps struct

```go
type Deps struct {
    DB          *gorm.DB
    Store        storage.Store // kho PUBLIC (luôn non-nil sau bootstrap)
    PrivateStore storage.Store // kho PRIVATE (đính kèm khách hàng)
    Cache       *cache.RedisCache
    TokenSvc    *token.Service
    RbacSvc     *authSvc.RbacService
    UserSvc     *hrSvc.UserService
    DownloadURL string
    Prod        bool   // cfg.IsProduction()
    TotpIssuer  string // tên hiển thị trong app authenticator
}
```

**Pattern**: domain module entrypoint nhận signature thống nhất `Register(authed *gin.RouterGroup, d deps.Deps)`. Tách package `deps` neutral để tránh cyclic import.

---

## 4. Router structure

### `internal/router/router.go`

```go
func Setup(cfg *config.Config, d deps.Deps) *gin.Engine {
    r := gin.New()
    // Global middleware: Logger, Recovery, RequestID, RequestSizeLimit, CORS, SecurityHeaders, RequestPath, ResponseHeaders
    r.GET("/health", ...)
    r.Static("/uploads", ...) + "/download/export" + "/download/templates"

    api := r.Group("/api")
    authed := api.Group("", middleware.JWTAuth(), middleware.AuthResponseHeaders(db, d.Cache))

    // Module routes — 7 dòng Register, danh sách đầy đủ: xem router.go
    // + PermissionEnforcer(route→perm map) gắn vào authed sau JWTAuth
    auth.Register(api, authed, d)        // auth có cả public (login) + authed
    crm.Register(authed, d)
    configModule.Register(authed, d)
    hr.Register(authed, d)
    history.Register(authed, d)
    importmodule.Register(authed, d)
    printModule.Register(authed, d)
    return r
}
```

### `internal/modules/<domain>/router.go` (1 file/domain)

Pattern: domain entrypoint loop wire sub-resources, mỗi resource expose `RegisterRoutes(group, handler.Deps{...})`.

Ví dụ `internal/modules/webapp/crm/router.go`:
```go
func Register(authed *gin.RouterGroup, d deps.Deps) {
    db := d.DB

    g := authed.Group("/crm")
    customer.RegisterRoutes(g, customerHandler.Deps{DB: db, S3: d.S3})
    appointment.RegisterRoutes(g, appointmentHandler.Deps{DB: db})
    payment.RegisterRoutes(g, paymentHandler.Deps{DB: db})
    order_plan.RegisterRoutes(g, orderPlanHandler.Deps{DB: db})
    // ... các sub-resource còn lại (ls crm/)
}
```

### `internal/modules/<domain>/<resource>/route.go` (1 file/resource)

```go
func RegisterRoutes(crm gin.IRouter, d handler.Deps) {
    h := handler.New(d)
    g := crm.Group("/payment")
    g.GET("/:id", h.GetPayment)
    g.POST("", h.CreatePaymentClinic)
    g.PUT("/:id", h.UpdatePaymentClinic)
    // ...
}
```

---

## 5. Handler DI pattern

Handler module khai báo `Deps` struct, embed vào `Handler`, expose `New(d Deps) *Handler`.

```go
// internal/modules/webapp/crm/payment/handler/handler.go
package handler

import (
    crmServices "erpCluvix/internal/modules/webapp/crm/service"
    "gorm.io/gorm"
)

type Deps struct { DB *gorm.DB }

type Handler struct {
    DB         *gorm.DB
    PaymentSvc *crmServices.PaymentService    // domain-level service
}

func New(d Deps) *Handler {
    return &Handler{
        DB:         d.DB,
        PaymentSvc: crmServices.NewPaymentService(d.DB),
    }
}
```

3 patterns đang tồn tại trong codebase (CONVENTIONS):

1. **Deps struct + embed** — handler modules (default).
2. **Factory pattern** — middleware (`middleware.AuthResponseHeaders(db, cache)`).
3. **SetCache pattern** — model/dao layer + shared packages cần cache nhưng không thể embed Deps (vd `dexie.SetCache(c)`, `exportHandlers.SetDB(db)`, `export.DownloadURL`).

---

## 6. Domain-level service packages (cross-resource business logic)

Khi business logic dùng chung > 1 resource trong cùng domain → đẻ vào `<domain>/service/` (flat package, không phải sub-resource).

| Domain | Path | Files | Nội dung |
|---|---|---:|---|
| `crm` | `modules/webapp/crm/service/` | 24 (+ subpackage `dao/`) | `payment.go`, `payment_crud.go`, `payment_commission.go`, `payment_validate.go`, `order_plan.go` (+ validate + test), `order_service_used_change.go`, `order_service_used_validate.go`, `customer_delete.go`, + integration test files |
| `auth` | `modules/webapp/auth/rbac/service/` | 5 | RBAC service (resource-level): `service.go`, `seeder.go` + test |
| `hr` | `modules/webapp/hr/user/service/` | 1 | UserService (resource-level) |
| `history` | `modules/webapp/history/log/service/` | 2 | `service.go`, `config.go` — `LogChanges` + `SnapshotRow` package fns + field constants |
| `config` | `modules/webapp/config/service/` | — | resource `service` (danh mục dịch vụ, handler) — không phải domain-level service package |

### Service-to-service dependencies

Đo bằng `grep -rn 'modules/.*/service' internal/modules/webapp/*/service/`:

- `crm/service` → import `history/log/service` (dùng `historySvc.LogChanges/SnapshotRow`).
- Không có cross-domain service ↔ service import khác; cross-domain logic đi qua direct DB queries hoặc package-level fns.

---

## 7. Cross-module imports (`export` là sink chính)

Đo bằng `grep -rln 'modules/webapp/export"' internal/modules`:

`internal/modules/webapp/export/` là **flat package** (23 file gồm test, không có sub-resource), expose `exportHandlers.ExportXxx(c, query)`. Caller import:

| Caller module | Import target |
|---|---|
| `crm` (appointment, customer, note, order_service_used, report), `config` (service, service_type, unit), `hr/user`, `print` | `internal/modules/webapp/export` |

**Pattern caller**: handler tách `BuildXxxQuery(c) *gorm.DB` (public), khi `?_export=1` → gọi `exportHandlers.ExportXxx(c, query)`. `export.DownloadURL` + `SetDB` set 1 lần trong bootstrap.

Không có cross-handler dependency khác — handler không gọi handler khác trừ qua export.

---

## 8. Repository Reader pattern (3 resources)

3 hot-spot service có `repo/` package extract reads để unit test pure (no DB):

| Resource | Path |
|---|---|
| `crm/payment` | `modules/webapp/crm/payment/repo/repo.go` |
| `crm/order_plan` | `modules/webapp/crm/order_plan/repo/repo.go` |
| `crm/appointment` | `modules/webapp/crm/appointment/repo/repo.go` |

Pattern: `Reader interface` + `gormReader` (prod) + test fakes. Service inject qua `NewXxxServiceWithReader(db, reader)`.

---

## 9. Dao layer (Plan 08)

45 package `dao/` hiện có (đo `ls -d internal/modules/webapp/*/*/dao | wc -l`) (resource có CRUD bằng entity typed).

Mẫu (`modules/webapp/crm/payment/dao/dao.go`):
```go
package dao

import (
    "erpCluvix/pkg/repository"
    "gorm.io/gorm"
)

type IDao interface {
    repository.BaseInterface[Payment]
}

type Dao struct {
    *repository.BaseRepo[Payment]
}

var _ IDao = (*Dao)(nil)

func New(db *gorm.DB) *Dao {
    return &Dao{repository.New[Payment](db, "payment")}
}
```

**Generic `pkg/repository.BaseRepo[T]`** cung cấp CRUD chuẩn (Find/List/Create/Update/Delete) với tenant scope mặc định + optimistic lock helper. KHÔNG retrofit module CRM hot-spot (return `[]map` với JOIN) — `BaseRepo` cho new resources + simple CRUD.

**Resources KHÔNG có `dao/`** (handler-only):
- `print/*` — package-fn render PDF, không query entity riêng
- `crm/dashboard`, `crm/report`, `crm/exchange_rate`, `crm/tinymce` — read-only / aggregate / external API
- `import/customer`, `auth/core`, `auth/twofa` — handler-only (đối chiếu bằng `ls <resource>`)

---

## 10. Circular imports

- `internal/router/router.go` → `internal/modules/<domain>` (one-way).
- `internal/modules/<domain>` ↔ `internal/modules/<domain>` đôi khi cross (vd `crm` import một số package của `config`) — chấp nhận miễn `config` không import lại `crm`.
- `internal/modules/<domain>/handler` → `internal/modules/<domain>/{service,dao,repo,dto}` (one-way).
- `internal/modules/<domain>/service` → `internal/modules/<other>/dao` (cross-domain entity import OK).
- `internal/modules/<domain>/dao` không import service/handler.
- `internal/deps` không import `internal/modules/*` trừ `auth/rbac/service` + `hr/user/service` (2 service được set vào Deps).

**Kết luận: không có circular import**. Build clean: `go build ./...`.

---

## 11. Tóm tắt graph (1 cấp)

```
main.go ──► bootstrap.New(cfg) ──► app.Serve(cfg, startedAt)
              internal/bootstrap/app.go
                DB → applog → Redis → cache → tokenSvc
                SetCache(dexie), export globals, seclog sink
                rbacSvc (+ seedPermissions) → userSvc
                Build deps.Deps → router.Setup(cfg, deps)
                                │
              internal/router/router.go
                global middleware, /api + authed group (JWT + PermissionEnforcer)
                auth / crm / config / hr / history / import / print .Register
                                │
   crm/router.go · config/router.go · auth/router.go · ...  (loop sub-resource)
                                │
   crm/payment/route.go → handler/ + dto/ + dao/ + repo/ → crm/service (shared)

Shared domain services:
  crm/service/         — 24 file (payment + order_plan + customer_delete business logic)

Cross-handler sink:
  crm, config, hr/user, print handlers ──► internal/modules/webapp/export (package fn ExportXxx)
```

---

## 12. Reading order cho AI agents

→ Xem `skill/skills/navigating-backend-architecture/SKILL.md` §6 (bảng reading order theo task type). Domain-level service: §6 file này; domain routing: §4; cross-handler call: §7.

---

**Tham chiếu kiến trúc khác:**
- `skills/navigating-backend-architecture/SKILL.md` — tech stack, layer model, cross-cutting concerns
- `arch-routing.md` — endpoint index (số endpoint: đếm trực tiếp trong code, không hardcode)
- `arch-glossary.md` — VN→EN domain glossary
- 
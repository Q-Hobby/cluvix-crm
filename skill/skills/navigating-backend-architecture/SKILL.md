---
name: navigating-backend-architecture
description: >-
  Tech stack, folder layout, naming convention, module map (handler + service modules), và
  bảng điều hướng routing theo domain (CRM, Config, HR/user, Ops), glossary thuật ngữ tiếng Việt↔code, S3 storage/imaging, TLS
  certificate + nginx hosting. Dùng skill này TRƯỚC KHI khám phá codebase Go BE lần đầu, khi cần
  tìm route/handler của một domain cụ thể, khi gia hạn/nạp SSL hoặc sửa vhost nginx, hoặc trước
  khi hỏi "module này nằm ở đâu" — tra bảng điều hướng dưới đây thay vì grep mù toàn repo.
---

# ERP Cluvix Backend — Architecture Overview

> Bảng điều hướng chi tiết theo domain nằm ở cuối file (mục "Điều hướng tiếp"). Mỗi dòng trỏ tới
> 1 file trong `reference/`.
> Project overview + dev tooling (lock `run_dev.sh`, bẫy air `include_ext`, ép rebuild, chẩn đoán `status=137`): `reference/project-overview-and-dev-tooling.md`.

> TLS certificate + nginx hosting (gia hạn cert wildcard ~6 tháng/lần, **3 nơi phải nạp**, fullchain vs chain, verify): `reference/tls-certificates-and-nginx.md`.


Tài liệu cho AI agents (Claude, Amazon Q, Kiro) hiểu codebase Go BE nhanh.
Chỉ ghi cái có trong code, không phỏng đoán.

> ⚠ CRM standalone: `internal/modules/` chỉ còn nhóm `webapp/` (đã bỏ `client/`, `mobile/`, `public/`).
> Path `internal/modules/<domain>` viết trước restructure 2026-07-28 đọc là `internal/modules/webapp/<domain>`.

---

## 1. Tech stack

| Thành phần | Phiên bản / chi tiết | Vị trí |
|---|---|---|
| Go | `1.25.0` | `backend/go.mod:3` |
| HTTP framework | `github.com/gin-gonic/gin v1.10.1` | `backend/go.mod:11` |
| ORM | `gorm.io/gorm v1.31.1` + `gorm.io/driver/mysql v1.6.0` | `backend/go.mod:29-30` |
| MySQL | qua GORM, DSN từ `config.DSN()` | `backend/db/db.go:14-32` |
| Redis | `github.com/redis/go-redis/v9 v9.18.0` — 2 DB: auth (sessions) + cache (app data) | `backend/main.go:71-84` |
| Decimal | `github.com/shopspring/decimal v1.4.0`, `MarshalJSONWithoutQuotes = true` | `backend/main.go` |
| JWT | `github.com/golang-jwt/jwt/v5 v5.3.1` | `backend/internal/middleware/auth.go:9` |
| Migrations | `github.com/golang-migrate/migrate/v4 v4.19.1` | `backend/cmd/migrate/main.go` |
| Validator | `github.com/go-playground/validator/v10` | binding tag `binding:"required"` |
| Excel export | `github.com/xuri/excelize/v2` | `backend/pkg/export/export.go` |
| Logger | `go.uber.org/zap` + Telegram sink cho ERROR | `backend/pkg/logger/logger.go` |
| Storage (local/S3/R2) | `pkg/storage.Store` (interface) + `github.com/aws/aws-sdk-go-v2/service/s3` | `backend/pkg/storage/` (`factory.go`, `local.go`, `s3.go`, `r2.go`) |
| Test infra | `TEST_MYSQL_DSN` env → MySQL disposable thật (KHÔNG dockertest) | `backend/internal/testhelpers/suite.go` |

---

## 2. Folder layout (`backend/`)

> **Cập nhật 2026-05-18** sau Plan 03 (layered module) + Plan 07 (services/→modules/) + Plan 08 (models/→dao/).
> `internal/handlers/` và `internal/services/` đã XOÁ. Mọi resource đẻ ở `internal/modules/<domain>/<resource>/`.

```
backend/
├── main.go                  # ~25 LOC — load env, gọi bootstrap.New(cfg) → app.Serve()
├── go.mod / go.sum
├── cmd/migrate/             # Standalone migrate binary (golang-migrate runner)
├── config/
│   └── config.go            # Env loader (TLS, JWT, Redis, S3 settings)
├── db/
│   └── db.go                # Global *gorm.DB (db.DB) — chỉ dùng ở bootstrap + cmd/migrate
├── migrations/              # SQL files (chỉ .up.sql) cho golang-migrate
├── docs/                    # arch-{overview,modules,routing,glossary}.md + refactor-* + audit
├── certs/                   # TLS certs (gitignored runtime)
├── storage/                 # uploads/ (kho public local), private/ (kho private local — KHÔNG static), export/, font/, fonts/, templates/
├── tmp/                     # tạm
├── internal/                # Application code (Go internal — chỉ module này import được)
│   ├── bootstrap/app.go     # Wiring tập trung: DB → Redis → tokenSvc → SetCache / gán field DownloadURL → deps → router
│   ├── deps/deps.go         # Shared Deps struct (DB, Store, PrivateStore, Cache, TokenSvc, RbacSvc, UserSvc, DownloadURL)
│   ├── router/router.go     # global middleware + loop module.Register(d) (danh sách module: xem file)
│   ├── modules/             # domain modules (đếm: ls internal/modules/) — xem arch-modules.md
│   │   └── <domain>/
│   │       ├── router.go    # Wire sub-resources qua RegisterRoutes(group, handler.Deps{...})
│   │       ├── service/     # (optional) cross-resource business logic flat package
│   │       └── <resource>/
│   │           ├── route.go
│   │           ├── handler/ # Deps + Handler + New(d Deps), dto.go, http_test
│   │           ├── service/ # (optional resource-level service)
│   │           ├── dao/     # entity.go (GORM struct) + dao.go (IDao + BaseRepo[T])
│   │           ├── dto/     # (hiếm — chỉ payment có)
│   │           └── repo/    # (Phase 4 — 5 hot-spot có Reader interface)
│   ├── models/base.go       # CHỈ còn BaseModel, SimpleModel, DeleteWithCheck (Plan 08)
│   ├── middleware/          # auth.go (JWTAuth), partner_auth.go, response_headers.go, cors.go, request_id.go, ...
│   ├── repository/          # (rỗng — pkg/repository.BaseRepo[T] đã thay thế)
│   └── testhelpers/         # MySQL test suite qua TEST_MYSQL_DSN (suite.go, fixtures.go, http.go, context.go) + tenant_leak canary
└── pkg/                     # Reusable utilities (cross-cutting, no business logic)
    ├── cache/redis.go       # *RedisCache (Get/Set/GetOrSet)
    ├── cleanup/file.go      # Dọn file expired storage
    ├── codegen/codegen.go   # Sinh mã code (KH-xxxx, …)
    ├── define/              # Hằng số nghiệp vụ (PaymentForOrder, …) per domain
    ├── dexie/               # IndexedDB sync helpers — BumpDexieTable, ConfigSystems{Get,Set}Option + SetCache
    ├── errs/                # Sentinel errors per-module (payment.go, order.go, treatment.go, inventory.go, loyalty.go, appointment.go, ai.go, common.go) + ErrVersionConflict
    ├── export/export.go     # Excel exporter chung (batch processing, formula sanitize)
    ├── helpers/             # decimal.go, gorm.go, helpers.go, json_schema.go, InitJSONColumnMap, InitDecimalColumnMap
    ├── i18n/                # T(c, key), TF(c, key, args), FieldLabel + vi/en/jp/fil dict
    ├── imaging/             # Resize, watermark (disintegration/imaging + webp)
    ├── logger/              # Zap + custom GORM logger + Telegram alert + Logger interface + MemoryLogger
    ├── pagination/          # Paginate(c, query, dest) + PaginateTyped[T] generic
    ├── query/filter.go      # Filter param parser cho list endpoints (ApplyFilter, ApplyKeywordFilter, ...)
    ├── repository/          # Generic BaseRepo[T] với tenant scope + optimistic lock helper
    ├── response/            # response.Data/Created/Updated/HandleError/NotFound/Conflict + client.go + validator.go
    ├── storage/             # Store interface + driver local/s3/r2 (STORAGE_DRIVER) + ValidateFileType (magic bytes)
    ├── telegram/            # Bot send-message
    ├── tenant/scope.go      # Multi-tenant: ScopedDBNullBranch (default) + canary test
    ├── token/               # JWT access/refresh + session/refresh revoke (Redis)
    ├── tz/                  # Timezone helpers
    └── validation/          # Custom validator rules (phone_vn) + Bind helper
```

---

## 3. Layer model

```
┌─────────────────────────────────────────────────────────────────────┐
│ HTTP request                                                        │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Bootstrap (internal/bootstrap/app.go)                               │
│   bootstrap.New(cfg) → wire DB+Redis+tokenSvc+rbacSvc+userSvc       │
│                       → SetCache (dexie/news/zalo/...)              │
│                       → DownloadURL field (export)                     │
│                       → Build deps.Deps                             │
│                       → router.Setup(cfg, deps)                     │
│   app.Serve(cfg) → http.Server + graceful shutdown (ctx)           │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Router  (internal/router/router.go)                                 │
│   global middleware: Logger, Recovery, RequestID, RequestSizeLimit, │
│                      CORS, SecurityHeaders, RequestPath,            │
│                      ResponseHeaders(db, cache)                     │
│   /api group → /authed group (JWTAuth + AuthResponseHeaders)        │
│   Loop modules: <module>.Register(authed, deps.Deps)               │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Domain router (internal/modules/<domain>/router.go)                 │
│   Wire sub-resources: <res>.RegisterRoutes(group, handler.Deps{...})│
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Resource route (internal/modules/<domain>/<resource>/route.go)      │
│   h := handler.New(d)                                               │
│   g := crm.Group("/payment"); g.GET("/:id", h.GetPayment); ...      │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Handler (internal/modules/<domain>/<resource>/handler/)             │
│   type Deps struct { DB *gorm.DB; ... }                             │
│   type Handler struct { DB; XxxSvc *crmServices.XxxService }        │
│   func New(d Deps) *Handler                                         │
│   func (h *Handler) GetPayment(c *gin.Context)                      │
│   • Bind DTO / query → call h.XxxSvc.Method(...)                    │
│   • response.Data / Updated / HandleError                           │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Service (DOMAIN-LEVEL: internal/modules/<domain>/service/*.go)      │
│   — shared business logic giữa các resource trong cùng domain       │
│   type PaymentService struct { DB *gorm.DB; repo paymentRepo.Reader }│
│   func NewPaymentService(db *gorm.DB) *PaymentService               │
│   • s.DB.Transaction(func(tx) {...})                                │
│   • Optimistic lock check → errs.ErrVersionConflict                 │
│   • historySvc.LogChanges(tx, ...) cho audit trail                  │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Dao (internal/modules/<domain>/<resource>/dao/)                     │
│   entity.go: type Payment struct { models.BaseModel; ... decimal }  │
│              func (Payment) TableName() string { return "payment" } │
│   dao.go:    IDao interface { repository.BaseInterface[Payment] }   │
│              Dao struct { *repository.BaseRepo[Payment] }           │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
                          MySQL
```

Routing entrypoint: `main.go` (~25 LOC) → `bootstrap.New(cfg)` → `router.Setup(cfg, deps)` → loop `<module>.Register(authed, d)` (danh sách module: xem `router.go`).

### Service location quy ước

| Vị trí | Khi nào |
|---|---|
| `<domain>/service/*.go` (flat package) | Business logic chia sẻ giữa > 1 resource cùng domain (vd `crm/service/payment.go` dùng cho `crm/payment` + cross-call từ `crm/order_treatment_plan`) |
| `<domain>/<resource>/service/` | Business logic chỉ riêng cho resource đó (vd `crm/customer/service/service.go`) |
| KHÔNG có service | CRUD đơn giản — handler gọi trực tiếp `dao.New(h.DB)` |

---

## 4. Cross-cutting concerns

| Concern | File / Pattern |
|---|---|
| **Multi-tenant scoping** | `pkg/tenant/scope.go`. Rule đầy đủ (default, deprecated, ngoại lệ, bẫy quên đăng ký bảng) → `skill/skills/backend-hard-rules/SKILL.md` §Tenant scope |
| **Money / Decimal** | `decimal.Decimal` từ `shopspring/decimal`. Marshal JSON ra number không quote: `decimal.MarshalJSONWithoutQuotes = true` ở `main.go`. So sánh: `a.Cmp(b) == 0` hoặc `a.Equal(b)`. Helpers: `pkg/helpers/decimal.go`, auto-parse decimal columns trong `pkg/pagination` qua `InitDecimalColumnMap` (INFORMATION_SCHEMA). |
| **Optimistic locking** | Sentinel `errs.ErrVersionConflict` ở `pkg/errs/common.go`. Service so `input.Version != current.Version` → return error. Handler dùng `response.HandleError(c, err)` — auto map sang HTTP 409. Direct: `response.Conflict(c, msg)`. Mẫu: `internal/modules/webapp/crm/service/payment.go`. |
| **i18n** | `pkg/i18n/i18n.go` — `i18n.T(c, "key")`, `i18n.TF(c, "key", args…)`, `i18n.FieldLabel(c, fieldName)`. Dictionaries: `vi.go`, `en.go`, `jp.go`, `fil.go`. Sentinel errors ở `pkg/errs/<module>.go` (payment/order/treatment/inventory/loyalty/appointment/ai/common). Verify keys: `go run scripts/check_i18n_keys.go`. |
| **JWT auth** | `internal/middleware/auth.go` — `JWTAuth(tokenSvc)` gin middleware. Claims chỉ có `UID` (+ `ipHash` khi IP binding). Set ctx `userID`. `GetCompanyID` đã xóa — dùng `GetBranchActiveID` (company đã xoá hoàn toàn D15, không chỉ đổi tên helper). Token service: `pkg/token/token.go` (issue/refresh/revoke + Redis sessions). (đã bỏ, D15/CRM standalone: `internal/middleware/partner_auth.go`/module `client/` không còn tồn tại trong repo này.) |
| **Response helpers** | `pkg/response/response.go` + `client.go`, `validator.go`. APIs: `Data`, `DataWithMessage`, `Created`, `Updated`, `Deleted`, `Validate` (422), `ValidateErr`, `HandleError`, `Conflict`, `NotFound`, `Forbidden`, `ServerError`. `HandleError` flow: LocalizedError → 422 + i18n.TF; sentinel `errors.New("key")` → translate qua TryT → 422 (hoặc 409 nếu ErrVersionConflict); MySQL 1451/1452 (FK) → message i18n; còn lại trả raw. |
| **File upload validation** | `pkg/storage/mime.go` — `storage.ValidateFileType(file, allowed)` đọc 512 byte đầu (magic bytes), chống Content-Type spoof. White-lists: `ImageMIMETypes`, `AttachmentMIMETypes`. |
| **Redis cache** | `pkg/cache/redis.go` — `cache.New(addr, password, prefix, db)` → `*RedisCache`. APIs: `Get`, `Set`, `GetOrSet`. Inject qua `deps.Deps.Cache` (DI) hoặc `SetCache` pattern cho package free-fn: `dexie.SetCache(c)`, `newsDao.SetCache(c)`, `zaloModels.SetCache(c)`. Tất cả trỏ chung 1 `appCache` instance. |
| **Pagination** | `pkg/pagination/pagination.go` — `Paginate(c, query, dest)`. Đọc `?page=&per-page=`. Header `X-Pagination-Total-Count/Page-Count/Current-Page/Per-Page`. `PaginateWithTotal(c, query, dest, total)` khi đã có count. Generic typed: `PaginateTyped[T any](c, q, &dest)`. Auto-parse JSON + decimal columns cho `*[]map[string]interface{}` qua schema map (`InitJSONColumnMap`, `InitDecimalColumnMap`). |
| **Logger / panic recovery** | `pkg/logger/logger.go` (zap) + `gin.CustomRecoveryWithWriter` ở `router.go`. `Logger` interface ở `pkg/logger/interface.go` + `MemoryLogger` test impl. ERROR tự gửi Telegram (`logger.Init` từ bootstrap). |
| **History/audit log** | `internal/modules/webapp/history/log/service/service.go` — `historySvc.LogChanges(db, "log_xxx", "fk_col", entityID, oldData, newData, includeFields, skipNullFields, userID)`. Snapshot: `historySvc.SnapshotRow(db, table, id, fields)`. Field-list constants (CustomerIncludeFields, AppointmentIncludeFields, …) ở `config.go`. |
| **DI** | `internal/deps/deps.go` shared Deps struct. Domain modules nhận `Register(authed, d deps.Deps)`. Handler nhận `New(d Deps) *Handler`. 3 patterns: Deps embed (handler default), Factory (middleware), SetCache (model/dao layer free-fn). 19/19 globals đã migrate (xem REFACTOR-SUMMARY.md (file cũ, đã gỡ)). |

---

## Đảo chiều phụ thuộc bằng BIẾN HÀM — và cái giá của nó

Package A đã import B thì B **không** gọi thẳng A được (Go từ chối vòng import). Cách đang dùng: B
phơi một biến hàm, A gắn vào lúc wire startup (A vốn đã biết B).

```go
// package B (bị A import)
var AfterX func(id uint64)          // nil = không ai gắn ⇒ no-op
// package A hoặc lớp wire (bootstrap/route)
b.AfterX = svc.DoSomething
```

⛔ Đánh đổi phải trả: biến hàm mua sự linh hoạt bằng cách **bỏ kiểm tra của compiler**. Không gắn =
no-op **hoàn toàn im lặng** — build xanh, test xanh, tính năng đơn giản là không chạy và không có lỗi
nào để đi tìm. Phải trả lại phần kiểm tra đó bằng lưới quét tĩnh neo **cả hai đầu dây** (nơi GẮN và
nơi GỌI), và lưới phải chặn cả ca gắn `nil`. Chỉ neo một đầu là còn nguyên một nửa lỗ.

## 5. Naming conventions

| Hạng mục | Quy ước | Ví dụ |
|---|---|---|
| File names | `snake_case.go` | `payment_crud.go`, `customer_warranty_card.go` |
| Package names | lowercase 1 từ (trừ `customer-care` → `package customercare` do dấu gạch) | `package handler`, `package dao`, `package service`, `package crm` |
| Module entry | `func Register(authed *gin.RouterGroup, d deps.Deps)` | `internal/modules/webapp/crm/router.go` |
| Resource route | `func RegisterRoutes(group gin.IRouter, d handler.Deps)` | `internal/modules/webapp/crm/payment/route.go` |
| Handler Deps | `type Deps struct { DB *gorm.DB; ... }` | `internal/modules/webapp/crm/payment/handler/handler.go` |
| Handler struct | `type Handler struct { DB *gorm.DB; XxxSvc *crmServices.X… }` | embed Deps hoặc list fields |
| Handler constructor | `func New(d Deps) *Handler` (KHÔNG còn `NewHandler`) | tất cả modules |
| Handler method | `func (h *Handler) ActionName(c *gin.Context)` | `h.GetPayment`, `h.CreateCustomer` |
| Service struct | `type XxxService struct { DB *gorm.DB; repo paymentRepo.Reader }` | `internal/modules/webapp/crm/service/payment.go` |
| Service constructor | `func NewXxxService(db *gorm.DB) *XxxService` + optionally `NewXxxServiceWithReader(db, r)` cho test | `internal/modules/webapp/crm/service/payment.go` |
| Dao | `type IDao interface { repository.BaseInterface[T] }`; `type Dao struct { *repository.BaseRepo[T] }`; `func New(db *gorm.DB) *Dao` | `internal/modules/webapp/crm/payment/dao/dao.go` |
| Entity | GORM struct + `func (T) TableName() string`. Đặt trong `dao/entity.go`, KHÔNG ở `internal/models/` | `internal/modules/webapp/crm/payment/dao/entity.go` |
| Sentinel errors | `errs.ErrXxx` (PascalCase, prefix `Err`) per-module file | `errs.ErrVersionConflict` (`pkg/errs/common.go`) |
| LocalizedError | `errs.NewLocalizedError(key, args...)` cho lỗi cần i18n format | `errs.NewLocalizedError("stock_insufficient", avail, deduct)` |
| Mixed VN/EN | Identifier/type EN, comment + i18n key mix VN/EN, FE-facing message VN | route comment `Tạo lịch hẹn`, error key `service_has_payment` |
| Field tags | `gorm:"column:..."` + `json:"..."` | dao/entity.go |
| Tenant scope | `tenant.ScopedDBNullBranch(c, h.DB, "table")` (default duy nhất) + 4th arg cho alias | resource handlers |
| Constants | `pkg/define/<domain>.go` — `define.PaymentForOrder`, `define.SpecialtyDental` | `pkg/define/payment.go`, `pkg/define/specialty.go` |

---

## 6. Reading order cho AI agent (theo task type)

| Task type | Đọc theo thứ tự |
|---|---|
| **Add new resource** | Hard Rules (`CLAUDE.md`) → `skill/skills/writing-backend-resources/SKILL.md` → `skill/skills/debugging-backend-queries/SKILL.md` |
| **Sửa endpoint** | `internal/modules/<domain>/router.go` → `<resource>/route.go` → `<resource>/handler/` → `<domain>/service/` hoặc `<resource>/service/` |
| **Fix tenant leak** | `pkg/tenant/scope.go` → `pkg/tenant/canary_integration_test.go` → grep `h.DB.Where\|h.DB.Table\|h.DB.First` trong `internal/modules/<suspect>/` (chưa dùng ScopedDBNullBranch) |
| **Add money field** | `skill/skills/writing-mysql-migrations/SKILL.md` → `pkg/helpers/decimal.go` → tham chiếu `internal/modules/webapp/crm/service/payment.go` |
| **Write test** | `internal/testhelpers/suite.go` (build tag `integration`) → `skill/skills/testing-backend/SKILL.md` → tham chiếu `internal/modules/webapp/crm/service/payment_integration_test.go` (DB thật) hoặc `payment_test.go` (pure fn) |
| **Add CRM resource** | `skill/skills/working-on-crm/SKILL.md` + module-specific cùng folder (`skills/working-on-crm/SKILL.md`, `skill/skills/working-on-crm/reference/customer.md`, `skills/working-on-crm/SKILL.md`, `skill/skills/working-on-crm/reference/treatment.md`, `skill/skills/working-on-crm/reference/order.md`) → `internal/modules/webapp/crm/router.go` xem cách wire |
| **Add config (lookup) table** | `skill/skills/writing-backend-resources/SKILL.md` → tham chiếu file đơn giản nhất `internal/modules/webapp/config/unit/` |
| **Auth / sessions / RBAC** | `skill/skills/handling-auth-and-workspace/SKILL.md` → `internal/middleware/auth.go` → `pkg/token/token.go` → `internal/modules/webapp/auth/rbac/service/` |
| **Dashboard / report query** | `skill/skills/working-on-ops/SKILL.md` → `internal/modules/webapp/crm/dashboard/handler/` → xem cache pattern qua `Deps.Cache` |
| **Export Excel** | `skill/skills/working-on-ops/reference/export.md` → `pkg/export/export.go` → `internal/modules/webapp/export/<entity>.go` (flat package fn) |
| **Storage / image** | `skill/skills/navigating-backend-architecture/reference/s3-storage.md` + `s3-imaging.md` → `pkg/storage/` → `pkg/imaging/` |
| **DB migration** | `skill/skills/writing-mysql-migrations/SKILL.md` → `backend/migrations/` (timestamp) → `cmd/migrate/main.go` |
| **Filter mapping (?filter=...)** | `skill/skills/debugging-backend-queries/reference/filter-mapping.md` → `pkg/query/filter.go` |
| **i18n (thêm message)** | `pkg/i18n/{vi,en,jp,fil}.go` → `pkg/errs/<module>.go` cho sentinel → `pkg/i18n/i18n.go` xem `T/TF/FieldLabel` |
| **History / audit log** | `skill/skills/debugging-backend-queries/reference/log-history.md` → `internal/modules/webapp/history/log/service/` |
| **Wiring / bootstrap** | `main.go` → `internal/bootstrap/app.go` → `internal/deps/deps.go` → `internal/router/router.go` |

---

## Phụ lục — request flow tóm tắt

```
client ──► PUT /api/crm/payment/123 (body: {version: 5, amount: ...})
              │
              ▼
   global middleware: Logger → Recovery → RequestID → RequestSizeLimit
                    → CORS → SecurityHeaders → RequestPath
                    → ResponseHeaders(db, cache)
              │
              ▼
   /api group → /authed group: JWTAuth() → AuthResponseHeaders(db, cache)
   ctx: {userID, branchActiveId, branchIds, locale}
   (đã bỏ, D15: companyID/username/companyActiveId/companyIds — company xoá hoàn toàn)
              │
              ▼
   crm.Register(authed, deps)
     → crm.Group("/crm")
     → payment.RegisterRoutes(g, handler.Deps{DB: db})
     → g.PUT("/payment/:id", h.UpdatePaymentClinic)
              │
              ▼
   Handler — internal/modules/webapp/crm/payment/handler/upsert.go
     h := handler.New(d)  // wires h.PaymentSvc = crmServices.NewPaymentService(db)
     h.UpdatePaymentClinic(c)
     ├── Bind DTO (PaymentInput) qua validation.Bind(c, &input)
     ├── tenant.ScopedDBNullBranch(c, h.DB, "payment").First(&current, id)
     ├── h.PaymentSvc.UpdatePaymentClinic(current, input, userID)
              │
              ▼
   Service — internal/modules/webapp/crm/service/payment_crud.go
     s.DB.Transaction(func(tx) {
       ├── beforeSave: validate (qua paymentRepo.Reader) → return errs.ErrVersionConflict nếu version cũ
       ├── tx.Save(&p) → update payment row
       ├── afterSave: updateCustomer (totalPayment, totalDebt, ...), calculatorPaymentServiceUsed
       └── historySvc.LogChanges(tx, "log_payment", "payment_id", p.ID, old, new, ..., userID)
     })
              │
              ▼
   response.HandleError(c, err)   → 409 nếu ErrVersionConflict, 422 còn lại
   response.Updated(c, p)         → 200 + i18n("updated")
```

---

## Điều hướng tiếp

| File | Nội dung |
|------|----------|
| `reference/arch-modules.md` | Module map — danh sách handler + service modules |
| `reference/arch-routing.md` | Routing index — auth groups, middleware, bảng trỏ tới file routing theo module |
| `reference/arch-routing-crm.md` | Routes CRM |
| `reference/arch-routing-config.md` | Routes Config (compact — số resource: `ls internal/modules/webapp/config/`) |
| `reference/arch-routing-hr.md` | Routes Auth + HR (user) |
| `reference/arch-routing-ops.md` | Routes Print/History/Import/Export |
| `reference/arch-glossary.md` | Thuật ngữ dự án (tiếng Việt ↔ code) |
| `reference/s3-storage.md` | Storage: `STORAGE_DRIVER=local\|s3\|r2`, kho public/private, DB lưu KEY + ghép URL (`PublicURL`/`ToKey`), đính kèm private + download |
| `reference/s3-imaging.md` | S3 image upload: `pkg/imaging`, magic bytes validation, resize; ⚠ xoá ảnh cũ khi update/xoá, dọn temp file sau upload |
| `reference/tls-certificates-and-nginx.md` | Cert wildcard `*.example.com` (DV ~6 tháng): 3 nơi phải nạp (2 VPS + CDN bên thứ 3), fullchain vs chain, quy trình gia hạn + rollback, 4 phép verify; ⚠ file Sectigo thiếu newline cuối, thiếu intermediate không lộ qua browser |

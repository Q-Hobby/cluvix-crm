---
name: backend-hard-rules
description: >-
  Rule bắt buộc khi viết/sửa code Go backend Cluvix: tenant scope (branch, 1 thương
  hiệu/DB), thông tin thương hiệu qua pkg/brand, tiền decimal, response/i18n, timezone,
  GORM pitfall, select fields, sensitive field, lỗi SQL bị nuốt. Dùng skill này TRƯỚC KHI
  chạm bất kỳ file .go nào trong backend/, kể cả khi chỉ sửa 1 dòng — phần lớn rule ở
  đây sinh ra từ sự cố thật (mất data, rò rỉ tenant, sai tiền) và đọc code không suy ra
  được.
---

# Backend hard rules (Go)

> Bản digest **vô điều kiện** nằm ở `CLAUDE.md` §Hard Rules — luôn áp dụng kể cả khi
> skill này không được nạp. File này là bản **chi tiết + lý do**, không chép lại digest.
>
> - Ví dụ code ❌/✅ đầy đủ: `reference/examples.md`
> - Security pattern (tenant leak, mass assignment, transaction): `reference/security-patterns.md`


## Flow
`internal/modules/` chia 4 nhóm (restructure 2026-07-28) — chọn folder theo loại việc:

| Làm việc gì | Vào đâu | Auth |
|---|---|---|
| Feature/API web-app (mọi domain: crm, emr, hr, accountant, config, omnichat, zalo, fhir...) | `webapp/<domain>/<resource>/` | JWTAuth (group `authed`) |
| API cho bên thứ 3 / Zalo Mini App (broker, customer, employee, quiz) | `client/` | PartnerAuth |
| API cho app mobile staff (BFF `/api/mobile/v1`) | `mobile/v1/` | JWTAuth dùng chung + login riêng |
| Webhook provider / OAuth callback / livechat visitor runtime | `public/` — CHỈ route + handler mỏng, logic nặng ở webapp; import 1 chiều public→webapp; đăng ký qua `public.Register` (PHẢI sau `omnichat.Register` — global wire) | Không JWT (signature/VisitorAuth riêng) |
| Ngoại lệ KHÔNG vào public/ dù no-JWT: login staff (`webapp/auth`), login mobile (`mobile/v1/auth`), FHIR (`webapp/fhir`, FHIRAuth) | tại module của chúng | — |

Domain module web-app: `internal/modules/webapp/<domain>/<resource>/` gồm `handler/` (parse request → gọi service → response) → `service/` (business logic) → `dao/` (entity + BaseRepo, KHÔNG viết logic). `internal/models/` chỉ còn `base.go`. `pkg/` shared packages, KHÔNG import internal.
Docs/story cũ viết trước 2026-07-28 ghi `internal/modules/<domain>` → hiểu là `internal/modules/webapp/<domain>` (trừ client/mobile giữ nguyên chỗ).

## Naming
- File: `snake_case.go` | Struct: `PascalCase` | JSON tag: match Yii2
- Handler: `List/Get/Create/Update/Delete{Resource}`

## HTTP Method
- Update JSON body → `PUT /:id`
- Update FormData (file upload) → `POST /:id`

## pkg/helpers — LUÔN check trước khi viết hàm mới
Trước khi viết bất kỳ utility function nào (pointer compare, deref, parse, convert...) → `grep` trong `pkg/helpers/` trước.
- Có rồi → dùng lại, KHÔNG viết lại inline trong service/handler.
- Chưa có → thêm vào `pkg/helpers/helpers.go` (hoặc file chuyên biệt nếu đã tồn tại, ví dụ `decimal.go`), KHÔNG viết local function trong module.
- Các helpers đã có: `IntPtr`, `IntPtrEqual`, `DerefDecimal`, `DecimalPtr`, `ToDecimal`, `ToInt64`, `ParseUint`, `ParseDecimalFields`...

## pkg/attendance — shared validators cho chấm công
Validator logic dùng chung giữa `hr/hr_attendance` và `client/appdev/employee` → `pkg/attendance/validate.go`.
- Trả `error` có message = i18n key — caller tự gọi `i18n.T(c, err.Error())`.
- `ValidateQR` KHÔNG ở đây (cần DB) — ở từng handler.

## Quy tắc cơ bản
- **KHÔNG dùng magic number/string** — constants trong `pkg/define/`
- Update: bind `map[string]interface{}` rồi merge (tránh zero value overwrite)
- JSON columns: `datatypes.JSON`, KHÔNG `string`
- Empty slice: `make([]T, 0)` (tránh `null`) — `var items []T` khi Find() 0 rows → nil → JSON `null` → FE `.map()` crash. **LUÔN** dùng `items := make([]T, 0)` trước Find(). Quan trọng với các endpoint trả raw array (load-by-keyword, autocomplete).
- camelCase DB columns: phải có `gorm:"column:xxx"`
- Sau CUD: `bumpDexieVersion()` + xóa cache
- Filter: `pkg/query/filter.go`

## Phân định trách nhiệm

| Layer | Trách nhiệm |
|---|---|
| **Entity** (`<resource>/dao/entity.go`) | GORM struct + TableName() — không có method, không có business hook |
| **Service** (`<resource>/service/`) | Toàn bộ business: validate, CUD, side effects (cache, codegen, related records) |
| **Handler** (`<resource>/handler/`) | Parse request → gọi service → trả response. Không query DB ngoài List/Get đơn giản |
| **`pkg/`** | Logic cross-module (helpers, query, define, pagination, response). Không chứa business logic |

- Chỉ dùng 1 chỗ → để ngay tại chỗ, không extract sớm. Lặp ≥3 lần mới tách.
- Validation: `binding tags` (required, min, max) → model struct; business validation → service
- GORM hooks: chỉ cho auto-set field kỹ thuật (version=1, timestamps). Side effects sau save → service/handler tự gọi.
- **AfterCreate/AfterSave tx KHÔNG có tenant scope (CRITICAL)**: `tx` từ hook là raw transaction — `tx.Table("customer").Where("id = ?", id).Updates(...)` là unscoped, có thể ghi vào record ngoài tenant nếu FK bị giả mạo. Side effects cần scoped DB (update customer, related record) → BẮT BUỘC vào handler dùng `tenant.ScopedDBNullBranch`.

## GORM Pitfalls (CRITICAL)
- **KHÔNG dùng `gorm:"-"`** cho computed/join fields cần scan — dùng `gorm:"column:xxx;->"` (read-only)
- **NGƯỢC lại — field GHÉP TRONG GO (slice/struct lồng) BẮT BUỘC `gorm:"-"`**: không có tag, GORM coi là quan hệ và **bỏ luôn cả câu Scan** (`invalid field found ...: define a valid foreign key for relations`). Chỉ nổ khi kết quả CÓ ≥1 dòng ⇒ test danh sách rỗng vẫn xanh.
- **Shadow field**: KHÔNG define `CompanyId int` trong child struct khi đã embed `BaseModel` — GORM chỉ thấy field child
- **Set NULL**: `Updates(map[string]any{"field": nilPointer})` — map-Updates KHÔNG skip nil (chỉ struct-Updates skip zero value) → set thẳng `*uint`/`*int` nil vào map là ghi NULL, không cần `gorm.Expr("NULL")`. Precedent: `config/omni_channel` UpdateBranch, `zalo_oa_config` RevokeTokens (`exp_token: nil`).
- **FK nullable = 0 (CRITICAL)**: FK column nullable → field Go PHẢI dùng `*uint`/`*int` (pointer), KHÔNG `uint`/`int`. `uint` zero value = 0 ≠ NULL → GORM ghi 0 vào DB, scope query `IN (x) OR IS NULL` không bao giờ match → orphaned data. `GetBranchActiveID` trả 0 = "không có branch active", KHÔNG được dùng làm FK value; guard: `if branchID == 0 { return }` hoặc set NULL qua raw Exec.
- **ScopedDBNullBranch**: KHÔNG dùng cho `user`, `user_profile`, `config_systems`, và RBAC catalog global `rbac_auth_item`/`rbac_auth_item_child`/`rbac_auth_assignment` (không có `branch_id` → raw `s.DB`, quyền/role global toàn hệ thống)
- **Bảng được `LEFT JOIN` vào KHÔNG được `ScopedDB*` che — phải tự gắn vế branch vào điều kiện `ON`**: `ScopedDBNullBranch(c, db, "order_service_used")` chỉ lọc bảng chính; `LEFT JOIN appointment ON appointment.id = ...` đứng đó không bị lọc branch lần nào. Gắn vào `ON` chứ KHÔNG phải `WHERE` (ở `WHERE` sẽ biến LEFT thành INNER, mất dòng), giữ nhánh `IS NULL`, lấy phạm vi từ `tenant.BoundsFromContext(c)`, phạm vi rỗng ⇒ để nguyên chứ đừng chế fallback. Kèm theo là nút thắt hiệu năng lớn (thiếu vế này thì index dẫn đầu `branch_id` của bảng join vô dụng ⇒ optimizer chọn sai bảng dẫn, đo được 0.62–1.40s → 0.04–0.07s, đo trước D15 khi trục scope còn là `company_id` — cùng cơ chế, giờ đổi trục sang `branch_id`): `skill/skills/debugging-backend-queries/reference/perf-list-endpoints.md` §Bảng được JOIN không bị scope.
- **Chạy NGOÀI request (tác vụ nền/worker/listener; scheduler cron đã bỏ 2026-10-17, bài học vẫn áp cho `helpers.RunBackground`) — phạm vi tenant phải TƯỜNG MINH, không bịa `*gin.Context`**: `ScopedDBNullBranch` đọc scope từ SESSION; tác vụ nền không có session. Bịa context giả để "gọi lại được hàm cũ" là bịa luôn phạm vi tenant — thiếu `branchActiveId` thì hàm KHÔNG thêm vế lọc nào (fail-OPEN) và job đọc xuyên chi nhánh mà không có lỗi nào báo. Dùng `tenant.Bounds` + `tenant.ScopedDBNullBranchBounds(db, b, table)`; `BoundsFromContext(c)` là cầu nối DUY NHẤT giữa hai đường, thân hàm scope dùng chung (`applyNullBranchScope`) nên hai đường không trôi lệch. Kiểm `b.Empty()` trước khi chạy — Bounds rỗng KHÔNG phải "lấy tất".
- **Ngoại lệ HỢP LỆ duy nhất của `ScopedDB*`: đếm/kiểm chính bảng gốc tenant khi nó đang được TẠO** (`branch` lúc chưa có chi nhánh nào). Ở thời điểm đó scope của user còn rỗng ⇒ mọi query qua `ScopedDB*` luôn trả 0 bất kể DB có gì ⇒ điều kiện "bảng đã có dữ liệu thì đóng cửa" không bao giờ đúng, cửa thiết lập ban đầu mở vĩnh viễn. Đếm **toàn bảng** (`db.Model(&Branch{}).Count`) là đúng chủ đích, KHÔNG phải quên scope — bắt buộc kèm comment tại chỗ nói rõ lý do, nếu không người review sau sẽ "sửa" nó thành `ScopedDB*` và mở lại lỗ. Ngoại lệ chỉ áp cho bảng gốc tenant đang được tạo, KHÔNG nới sang bảng nghiệp vụ. (Trước D15 ngoại lệ này còn áp cho `company` — bảng đã xoá.)
- **Mở rộng tham số thành interface mà `*gin.Context` ĐÃ thoả mãn sẵn — cách thêm lối vào không-HTTP mà không sửa call site**: `pkg/query` đổi `c *gin.Context` → `c Params` (interface chỉ có `Query`/`QueryArray`) ⇒ 848 call site compile nguyên, tác vụ nền (trước đây là cron) dựng bộ lọc bằng `query.ParseValues("filter[doctor_id]=3")`. Đồng thời tách nguồn-tham-số khỏi phạm-vi-tenant làm việc "quên truyền scope" thành **không diễn đạt được**, thay vì trông cậy vào người review nhớ. Precedent: `BuildAppointmentQueryIn(p query.Params, b tenant.Bounds)` (2026-09-06).
- **Cache invalidation**: update data ảnh hưởng scope → gọi `middleware.InvalidateUserScope(userID)`
- **`RowsAffected == 0` KHÔNG có nghĩa "không khớp row nào" (bài học 2026-08-14)**: MySQL mặc định đếm row *thực sự ĐỔI GIÁ TRỊ*, không phải row khớp `WHERE` (`CLIENT_FOUND_ROWS` tắt). Dùng nó làm guard "row not found" thì mọi luồng **idempotent** — ghi lại đúng giá trị cũ — bị biến thành lỗi. Thật: `linkZaloConfigAccount` gán `account_id` y như cũ ở lần gọi thứ 2 → `RowsAffected = 0` → trả lỗi giả, 4 test cũ đỏ ngay. Cần biết "có row không" → `SELECT` id trước rồi `UPDATE ... WHERE id = ?` (qua đúng scope tenant), đừng suy từ `RowsAffected`. Cùng lớp: import theo dòng có `id` → dòng hợp lệ bị báo lỗi giả (2026-09-03). Cờ này nằm ở DSN trong `config/config.go` (`clientFoundRows` KHÔNG bật). **Ngoại lệ đúng**: optimistic lock `WHERE version = ?` + `RowsAffected == 0 → 409` vẫn hợp lệ — ở đó row thật sự không khớp `WHERE` vì version đã đổi.
- **MySQL named lock (`GET_LOCK`/`RELEASE_LOCK`) session-scoped, KHÔNG transaction-scoped** — bắt buộc dùng chung 1 `*sql.Conn` (`db.DB(); sqlDB.Conn(ctx)`) cho cả acquire lẫn release, KHÔNG query qua `*gorm.DB` trực tiếp (mỗi query có thể checkout connection khác nhau trong pool → GET_LOCK/RELEASE_LOCK lệch session, lock không bao giờ release đúng).

## Codegen
- 13 tables: appointment(LH), customer(KH), order_treatment_plan(DH), treatment_course(TC), order_service_used(SV), reminder(RE), service(DV), payment(TT), broker(CTV), broker_payment_commission(BC), warehouse(NK), stock_entry(SE), stock_usages(SU)
- `BaseModelWithCode` models: hook BeforeCreate tự generate qua `resolveBranchID(tx)` — KHÔNG cần field company trong struct (D15: đã bỏ trục company khỏi codegen)
- Custom struct models (Payment, Broker, StockEntry...): gọi `codegen.Generate(tx, table, &branchID)` trước `tx.Create` — chữ ký chỉ còn `branchID` (trước D15 có thêm `&companyID`, đã bỏ)
- Raw map handlers: đọc `branch_id` từ body bằng `helpers.ToInt64`, gọi `codegen.Generate` manual
- `branch_id` lấy từ **payload/model**, KHÔNG từ JWT/session
- **CRITICAL**: payload branch_id PHẢI validate nằm trong `middleware.GetBranchIDs(c)` trước khi Create/Update — KHÔNG trust thẳng. customer_id (hoặc FK khác) từ body cũng phải verify thuộc tenant (`tenant.ScopedDBNullBranch(c, db, "<table>").Where("id = ?", id).Count()`). Thiếu check = IDOR ghi (tenant A tạo/sửa record trỏ customer tenant B). Pattern: `crm/appointment/handler/scope.go`. Helper có sẵn cho check branch_id: `middleware.IsBranchAllowed(c, branchID)` — branchID nằm trong `GetBranchIDs(c)` (rỗng = không giới hạn branch → cho qua). Check trước khi viết lại logic tương tự.
- ⚠ **(đã bỏ, D15)** Trước 1 thương hiệu/DB, còn thêm rule "company validate / company ghi / company TÌM phải là MỘT giá trị" (bài học import lead 2026-09-03, `pkg/customerimport.customerScope`, `middleware.IsBranchAllowedInCompany`, `middleware.IsCompanyAllowed`) — company_id đã xoá khỏi schema (story-62) nên các helper này không còn tồn tại; import lead giờ chỉ còn validate `branch_id` qua `IsBranchAllowed`.
- **Update handler validate FK/scope field (branch_id/department_id/staff_id...) CHỈ khi giá trị body khác giá trị đang lưu** — record cũ có thể có FK ngoài phạm vi quyền hiện tại của user đang sửa (gán bởi người khác, đổi cơ cấu sau này); validate lại giá trị KHÔNG đổi sẽ chặn nhầm save hợp lệ không phải hành vi client cố ý sửa field đó. Pattern mirror `company_id` check có sẵn (so `current.X` trước khi validate). Sự cố thật: `customer-care/care` `UpdateCustomerCare` 2026-07-27. Gate bằng "key CÓ MẶT trong payload" là SAI: form FE gửi lại đủ field, file export/import luôn đủ cột → phải so với `current.X`, chỉ validate khi khác (hoặc khi scope cha như `company_id` đổi). (Khác rule xoá field ở `merge*` — `skills/debugging-backend-queries/reference/patterns-gotchas-gorm-write.md`: ở ĐÓ key-presence là đúng, vì đang nhận diện Ý ĐỊNH xoá; ở ĐÂY đang quyết định có RE-VALIDATE quyền hay không.) Cùng lý do: cột không đổi cũng KHÔNG được đưa vào `Updates` — bump `updated_at`/`updated_by` của record không ai sửa là sai dấu vết audit, và với bảng nhiều FK thì mỗi UPDATE thừa là chi phí thật (đo trên `user`: 3–19s/dòng).
- Tuỳ biến format/company qua `config_code.include_branch_code`/`monthly_reset`/`serial_length` (đọc trong `findConfig`/`generateMonthly`) — `include_branch_code=0` bỏ mã chi nhánh khỏi prefix, `monthly_reset=0` bỏ segment YYMM + serial tăng vĩnh viễn (dùng row `config_code_monthly` cố định year=0/month=0), `serial_length` set độ rộng tối thiểu (tự nở khi vượt).
- **CRITICAL — cột mã UNIQUE optional (không `binding:required`) PHẢI 2-step insert**: insert temp code unique (`_tmp<UnixNano>`) → `Create` → update lại bằng `PREFIX+YYMM+LPAD(id,5,'0')`. KHÔNG insert thẳng `''` hay trust `codegen.Generate` (trả `""` nếu `config_code` chưa seed parent_table đó) → 2 record cùng để trống mã fail `Duplicate entry ''`. Đã lặp ở inventory (`stock_adjustment`/`stock_take`) và `config/material` — áp dụng cho MỌI resource mới có cột mã kiểu này.

## Optimistic Locking
- Áp dụng: Appointment, CustomerCare, Broker, Coupon, OrderPlan, TreatmentCourse
- Service: check `input.Version != db.Version` → return `errs.ErrVersionConflict` → `version += 1`
- Handler: KHÔNG check version trực tiếp — chỉ gọi `response.HandleError(c, err)`
- KHÔNG thực hiện trong model hook

## Response Format
- `Data(c, data)` → `200 { data }`
- `DataWithMessage(c, data, msg)` → `200 { data, message }`
- `CreatedDataWithMessage(c, data, msg)` → `201 { data, message }`
- `Deleted(c)` → `200 { message }`
- `Validate(c, msg)` / `ValidateErr(c, err)` → `422 { message }`
- `NotFound(c)` → `404` | `Forbidden(c)` → `403` | `ServerError(c, err)` → `500`
- **`NotFound(c)` CHỈ khi id có giá trị nhưng thực sự không tồn tại/không thuộc tenant.** Id rỗng hoặc không parse được số (`"undefined"`, `"NaN"`...) → `response.Validate` (422), KHÔNG `NotFound` — FE interceptor coi MỌI 404 là lỗi toàn cục, xem `frontend/skill/skills/working-on-fe-modules/reference/fe-patterns-gotchas.md` §Interceptor 404 redirect toàn app.
- List: `{ items, _meta }` qua `pagination.Paginate`
- Message luôn qua `i18n.T(c, "key")`, KHÔNG hardcode string
- KHÔNG dùng `c.JSON()` trực tiếp cho CRUD
- Update handler: `response.Updated(c, item)` PHẢI truyền entity đã merge field mới — KHÔNG `response.Updated(c, nil)`. FE `BaseForm.onSubmit` (`responseData.data || responseData`) + `BaseList.updateRow` match theo `id` → nil data khiến list không refresh sau update thành công (silent, không lỗi rõ).

## Logging sink — chọn đúng theo mục đích
- Global zap pin `ErrorLevel` (`pkg/logger/logger.go`) → `logger.Warn`/`Info` bị NUỐT câm. Đừng dùng cho telemetry cần thấy.
- Audit/dry-run telemetry (would-deny, mismatch, đo trước khi bật enforce) cần AGGREGATE → `seclog.Event` (async, non-blocking, ghi `security_audit_log`, query bằng SQL). KHÔNG `logger.Error` (trigger Telegram alert = spam).
- Debug tạm: `fmt.Fprintf(os.Stderr, ...)`. Permanent + đáng alert: `logger.Error`.

## Timezone
- **KHÔNG dùng `time.Local` / `time.Now().Location()`**
- Lấy loc: `loc := tz.LocFromContext(c, db.DB)`
- Time range: `tz.StartOfDay(loc)` / `tz.MonthRange(loc, 0)` / `tz.DayRange(loc, "2006-01-02")` / `tz.StartOfMonth(loc, offset)`
- Năm/tháng cụ thể: `tz.MonthStart(db, companyID, year, month)` / `tz.MonthEnd(...)`

## Select fields
- **KHÔNG `SELECT *`** — chỉ select field FE cần dùng
  - Hệ quả: thêm cột mới vào entity **phải rà TỪNG chuỗi `Select` tường minh** đang đọc bảng đó, không
    chỉ chuỗi của endpoint mình làm. Thiếu ở một chuỗi → field về zero value, **không lỗi không cảnh
    báo**. Thật (2026-08-20): `loadConvForAutomation` thiếu 3 cột ⇒ `reply_time` KHÔNG BAO GIỜ được
    ghi và `first_response` ghi LẶP mỗi lần bấm macro — build xanh, test cũ xanh.
- List: explicit SELECT; Detail: explicit SELECT (có thể nhiều hơn List)
- Join relations: chỉ select field cần thiết, KHÔNG `table.*`
- Exception: GORM `First/Save` với struct model → chấp nhận select all

## Nested data
- **Struct + Preload**: tạo `{Model}WithRel` + `gorm:"foreignKey:XxxId"` → `Preload("Xxx")` — ưu tiên cho List/Get cấu trúc rõ
- **`map` + attach thủ công**: query relation riêng → gán `item["xxx"] = data` — khi query phức tạp
- **KHÔNG** flatten join thành `customer_name` — phải trả nested object `customer: {...}`
- Ref struct đơn giản: tạo `XxxRef` riêng (`UserProfileRef`, `CustomerRef`)
- **Preload 1:many (array) trong List** → KHÔNG. `Preload("StockReturns")`, `Preload("Logs")` v.v. chỉ dùng trong Get/Detail. List chỉ Preload quan hệ 1:1 (foreign ref).
- **Nested customer có field `phone`/`other_phone`** (qua `AttachRelationCompanyOnly`/`AttachRelation`/`Preload`) → BẮT BUỘC mask ngay sau, không exception theo module. Chi tiết + snippet: `skill/skills/working-on-crm/reference/customer.md` §Phone visibility.

## Money fields — BẮT BUỘC `decimal.Decimal` (CRITICAL)
- Model: `decimal.Decimal` (NOT NULL) hoặc `*decimal.Decimal` (nullable)
  ```go
  Amount decimal.Decimal `gorm:"type:decimal(20,2);not null" json:"amount"`
  DepositAfter *decimal.Decimal `gorm:"type:decimal(20,2);default:0.00" json:"deposit_after"`
  ```
- `main.go`: `decimal.MarshalJSONWithoutQuotes = true` → FE nhận `number`
- **Arithmetic** — dùng method decimal, KHÔNG `+ - * /`:
  | float64 | decimal |
  |---|---|
  | `a + b` | `a.Add(b)` |
  | `a - b` | `a.Sub(b)` |
  | `a * b` | `a.Mul(b)` |
  | `a / b` | `a.Div(b)` |
  | `math.Round(a*100)/100` | `a.Round(2)` |
  | `a > 0` | `a.IsPositive()` |
  | `a != 0` | `!a.IsZero()` |
  | `a == b` | `a.Equal(b)` |
  | `a > b` | `a.GreaterThan(b)` |
  | `-a` | `a.Neg()` |
  | `math.Abs(a)` | `a.Abs()` |
  | `0.0`, `100` | `decimal.Zero`, `decimal.NewFromInt(100)` |
- **Nil-safe helpers**: `helpers.DerefDecimal(p)` / `helpers.DecimalPtr(d)` / `helpers.ToDecimal(v interface{})`
- **Guards**: Before divide → check `divisor.IsZero()`; Before insert → check `amount.IsPositive()`
- **KHÔNG phải tiền** (giữ float64): `percent_success`, `StandardWorkdays`, `OtHours`, `Percentage`, `Point`, `QuantityMin/Max`

## Decimal scan
- `pagination.Paginate`: tự parse decimal qua `ParseDecimalFieldsForTables` — **chỉ rows TOP-LEVEL**. Khối attach **nested SAU khi Paginate chạy** phải tự gọi helper. Khuôn hay gặp: detail đúng / list sót (detail dùng loader riêng đã parse).
- Query thủ công vào map — chọn ĐÚNG nhánh:
  - Cột THẬT của bảng → `helpers.ParseDecimalFieldsForTables(rows, "table_a", "table_b")` (nhận slice; truyền đủ cả bảng JOIN)
  - **Alias `SUM(...)`/biểu thức → `helpers.ParseDecimalFields(row, "alias1", ...)`** (nhận 1 row → tự loop). Alias KHÔNG thuộc bảng nào nên helper theo-bảng bỏ qua; `SUM(decimal)` và cả `SUM(int)`/`SUM(COUNT())` đều ra DECIMAL. Phép toán số học (`/`, `FLOOR`, `DATEDIFF(...)/86400`...) trên alias — kể cả khi input toàn INT/timestamp — cũng ép kiểu DECIMAL y hệt `SUM()`, cùng phải qua `ParseDecimalFields`.
  - Có cả cột thật lẫn alias → gọi CẢ HAI.
- Struct field `decimal.Decimal`: GORM tự convert
- **`NormalizeMapDecimals`/`NormalizeSliceDecimals` KHÔNG thay thế được 2 helper trên**: chúng chỉ xử lý nhánh `[]uint8`, mà GORM `scanIntoMap` đã đổi `sql.RawBytes` → `string` TRƯỚC đó → với `Raw(...).Scan(&map)` chúng là **no-op im lặng**. Handler có gọi 2 hàm này ≠ đã xử lý decimal.
- **`Paginate` khớp theo TÊN CỘT của bảng** ⇒ alias tổng hợp (`SUM(...) AS x`) trong query phân trang KHÔNG được phủ, phải tự gọi `ParseDecimalFields` sau `Paginate`.
- **`Paginate` chỉ phủ BẢNG CHÍNH, KHÔNG phủ bảng JOIN**: `pagination.extractTables` đọc bảng JOIN từ `Statement.SQL`, nhưng GORM reset `stmt.SQL` sau execute (`gorm@v1.31.1/callbacks.go:150`) ⇒ nhánh đó là **dead code**. Cột DECIMAL lấy từ bảng JOIN trong list phân trang vẫn ra string → phải tự gọi `ParseDecimalFieldsForTables(rows, "bang_join")`.
- `Table("tbl alias")` → `Statement.Table` = **`alias`**, KHÔNG phải tên bảng (GORM `tableRegexp`) ⇒ logic bám `Statement.Table` sẽ bỏ luôn cột decimal của **chính bảng chính**. Tên thật: `Statement.TableExpr.SQL` rồi lấy token đầu.
- `Statement.SQL` **bị reset sau execute** (`gorm@v1.31.1/callbacks.go:150`) ⇒ mọi logic đọc SQL đã build **sau** `Find` là dead code. `Statement.Joins` KHÔNG bị reset → đó mới là nguồn tin cậy để lấy bảng JOIN.
- Alias trùng tên cột trong `baseline.up.sql` KHÔNG chứng minh `Paginate` phủ được — phải đối chiếu schema **SAU migration**. Ca thật: `order_treatment_plan.total` đã bị DROP (`20260601221056`), alias `COALESCE(SUM(...)) AS total` vẫn ra string.
- Dấu hiệu nhận biết field decimal CHƯA parse: giá trị giữ `"9000000.00"`; nếu đã qua `decimal.Decimal` thì `String()` cắt còn `"9000000"`.
- Đọc giá trị decimal ra từ map: `helpers.ToDecimal(v)` — KHÔNG type-assert `v.(decimal.Decimal)` (assertion fail âm thầm → tiền tính thiếu, không cảnh báo).
- **Đo blast-radius site dùng `Paginate` bằng AST-parse (`go/parser`), KHÔNG grep chuỗi literal**: grep `Table("tbl alias")` bỏ lọt `Table()` nhận biến/nối chuỗi (vd `client/handler.ScopedClient`) và query dựng ở hàm khác rồi mới truyền vào `Paginate` — cả 2 dạng grep đếm thiếu.

## Đổi kiểu field handler `webapp/*` có delegate mobile

## Lỗi SQL bị nuốt → `200 + list rỗng` (CRITICAL)

- `pagination.Paginate*` **đã tự trả 500** khi lỗi SQL. Nhưng `.Find`/`.Scan`/`.Update` **viết tay PHẢI tự kiểm `.Error`** — GORM không panic; `.Find()` lỗi trả slice rỗng và handler vẫn `c.JSON(200, ...)`.
- **Chân dung bệnh, nhận ra ngay**: `items: []` nhưng field tổng ≠ 0 (subquery chạy, query chính chết). Hoặc POST trả 201 mà DB không đổi gì.
- 4 ca thật cùng gốc, mỗi ca sống nhiều tháng không ai biết: cột bị migration đổi tên/DROP · `only_full_group_by` (1055) · query thiếu `.Table()` · `Update` lên cột không tồn tại.
- **Pattern chuẩn cho map-scan viết tay ngoài package `pagination`**: `tx := q.Find(&x)` (hoặc `Scan`/`Take`) rồi `if pagination.IsQueryError(tx.Error) { pagination.FailQuery(c, tx.Error); return }`. `IsQueryError` loại trừ `gorm.ErrRecordNotFound` (0 dòng hợp lệ ≠ lỗi); `FailQuery` tự niêm phong writer (chống double-write nếu handler còn code chạy tiếp).
- Service layer không có `*gin.Context` (vd `RbacService`, `UserService`, `CustomerService`) → đổi chữ ký hàm trả thêm `error`, caller (handler, có `c`) tự gọi `FailQuery`. KHÔNG gọi `FailQuery` từ service.
- Endpoint đang gộp "lỗi SQL" và "0 dòng thật" vào chung 1 nhánh `response.NotFound` (404) → tách riêng: `IsQueryError` trước, `len(x)==0` sau — 404 chỉ dành cho 0-dòng thật.
- Aggregator gộp nhiều nguồn dữ liệu (vd 1 timeline từ N bảng): 1 nguồn lỗi SQL → mặc định làm CẢ kết quả lỗi (không trả kết quả thiếu 1 phần coi như bình thường) — đúng tinh thần "lỗi và không-có-dữ-liệu phải phân biệt được".
- Check tay trước khi merge PR đụng map-scan mới: `python3 backend/scripts/check_swallowed_sql_errors.py` (chưa gắn CI tự động — hạ tầng CI thật chưa chốt).
- **Cùng bệnh áp dụng cho `.Raw().Scan(&x)` đích TYPED** (`decimal.Decimal`/`int64`/struct aggregate, không chỉ map) — phổ biến nhất ở các handler `*/dashboard` (KPI card SUM/COUNT). Cùng pattern chuẩn `tx := .../IsQueryError/FailQuery`. Check tay: `python3 backend/scripts/check_swallowed_sql_errors_typed.py <path...>` (script riêng, KHÔNG gộp với script map ở trên — tiêu chí phát hiện khác nhau). Gotcha đã gặp thật (epic `raw-scan-aggregate-error-sweep`, 2026-08-01): hằng số khoảng cách tối đa từ dòng `.Raw(` tới dòng `.Scan(` trong script PHẢI ≥25 dòng (không phải 10) — câu SQL nhiều dòng (CASE WHEN, nhiều JOIN) thực tế trong repo có khoảng cách tới 17+ dòng, ngưỡng thấp làm bỏ sót finding thật mà không báo lỗi gì (false-negative âm thầm).
- **Script quét tĩnh phải phân giải KIỂU của biến, không grep chuỗi** — và kết quả sai theo **CẢ HAI chiều**: false-positive (chỗ đã an toàn bằng cú pháp script không nhận, vd **gán tuple** `a, b := x.A, x.B`) lẫn false-negative (biến thể cú pháp bị bỏ sót, vd `ShouldBindJSON(item)` **không có `&`** khi entity là con trỏ từ `repository.FindByID`). Ca thật (epic `idor-body-id-write`, 2026-08-12): grep thô ra 86 điểm, số đúng **81**, và bỏ sót nguyên một lane multipart. ⇒ Bảng từ script chỉ là **điểm khởi đầu**, story vá phải đọc tay từng điểm; "có dòng bảo vệ" ≠ PASS.
- **`check_swallowed_sql_errors_typed.py` CHỈ bắt `.Raw(...).Scan()` — KHÔNG bắt GORM chain `.Table(...).Count()/.Scan()` không qua `.Raw()`**. Ca thật: `sale/dashboard/handler.go` trước fix 2026-08-07 báo "0 điểm nghi ngờ — sạch" dù ~15 query GORM chain (`tenant.ScopedDBNullBranch(...).Table(...).Count()/.Scan()`) chưa check `.Error`. Script sạch KHÔNG chứng minh file sạch — handler dùng GORM chain thay vì `.Raw()` vẫn phải tự grep `\.Scan(&\|\.Count(&` + soát tay từng điểm.

## Sensitive fields (api_key/access_token/secret_key...) — CRITICAL
- Trường lưu secret bên thứ 3 PHẢI mã hoá qua `pkg/crypto` enc-aware: `crypto.EncryptToken`/`crypto.DecryptTokenAware`/`crypto.IsEncrypted` (marker `enc:v1:`, key `EMR_CONFIG_ENCRYPTION_KEY`) — KHÔNG tự viết AES riêng, KHÔNG lưu plaintext dù comment field ghi "encrypted".
- Đã cắn: `ai_config.api_key` (comment nói "Encrypted" nhưng chưa từng mã hoá) — fix + backfill theo mẫu `cmd/encrypt_tokens`.
- **Key env vắng mặt ⇒ lưu PLAINTEXT, không lỗi** (`pkg/crypto/aes.go`): `EncryptAESGCMKey`/`EncryptAESGCM` chỉ log WARN "dev-mode" rồi ghi nguyên văn, decrypt trả lại nguyên văn ⇒ hệ thống chạy như bình thường, không ai biết secret đang trần.
- **Bật key muộn trên hệ đã có dữ liệu — 2 đường KHÁC nhau**: đường marker-aware (`DecryptTokenAware`, `enc:v1:`) an toàn, plaintext cũ vẫn đọc được; đường THÔ (`DecryptAESGCMKey`/`DecryptAESGCM` — `user.totp_secret`, `digital_signature.credentials_enc`, einvoice `*_enc`) **KHÔNG**: hex-decode giá trị plaintext cũ sẽ lỗi ⇒ 2FA/chữ ký số chết.
- Trước khi set key ở prod: đếm dòng non-empty của đúng các cột đó; có dữ liệu ⇒ re-encrypt hoặc reset trước (2FA: tắt rồi cho user đăng ký lại).

## Path param `id` PHẢI parse ra số trước khi vào query (CRITICAL)

`c.Param("id")` trả về CHUỖI. Đưa thẳng vào `Where("id = ?", ...)` thì MySQL **ép kiểu chuỗi → số**
để so với cột INT: `WHERE id = '79738abc'` khớp đúng dòng `id = 79738`. MySQL chỉ sinh **warning
1292**, GORM nuốt warning ⇒ **build xanh, test xanh, request trả 200 "thành công"**.

Đã kiểm qua đúng đường GORM + driver (2026-09-07), không chỉ bằng `mysql` client — đây là điểm dễ
kết luận nhầm, vì nếu chỉ thử `mysql -e "... id = '1abc'"` thì có người sẽ cãi là do interpolate:

```go
testDB.Table("customer").Where("id = ?", "1abc").Take(&m)   // → trả về record id=1
testDB.Table("customer").Where("id = ?", "abc").Take(&m)    // → record not found (chuỗi thuần thì vô hại)
```

Ở đường ĐỌC thì hậu quả nhẹ (URL rác vẫn nằm trong tenant scope, chỉ làm cache/log/analytics đếm
nhiều URL cho cùng một record). Ở đường **GHI/XOÁ thì URL rác sửa/xoá trúng record thật** —
`POST /crm/customer/5abc/add-tag`, `DELETE /customer-care/complaints/5abc`.

```go
// ❌ chuỗi thô vào query
Where("customer.id = ?", c.Param("id"))
id := c.Param("id")
tenant.ScopedDBNullBranch(c, db, "x").Table("x").Where("id = ?", id).Delete(nil)

// ✅ parse trước, không parse được ⇒ 404 (helper đã có, đừng đẻ bản sao)
id, ok := helpers.ParseID(c)        // (uint, bool) — pkg/helpers/helpers.go, tự loại 0/âm/rỗng
if !ok { response.NotFound(c); return }
```

**Chặn tái diễn bằng KIỂU, không bằng trí nhớ**: hàm service nhận id thì khai `id int`/`uint`, đừng
nhận `string`. Ca thật — đổi `CustomerService.Update(id string → int)` làm compiler chỉ thẳng ra
`emr/patient/handler.UpdatePatient` cũng đang truyền `c.Param("id")` thô, chỗ mà đọc tay đã bỏ sót.
Kéo theo phải đổi cả các hàm nội bộ nhận id (`genCode`, `syncToothBioScope`, `customerInScope`) —
chi phí một lần, đổi lại là không sót được nữa.

**Phạm vi tồn kho — ĐÃ VÁ HẾT 2026-09-07 (119 + 38 điểm)**: `verify-id-params.sh` quét theo TỪNG
HÀM; đợt quét đầu ra **119 điểm / 52 file (25 GHI/XOÁ)**, nay còn **1** (`med/dqg_config`, để lại vì
phiên khác đang sửa chính file đó). `BASELINE` đã hạ 119 → 1 — vá tiếp thì hạ tiếp, một người ghi.

⚠ **Con số tồn kho đó từng SAI, và sai theo hướng yên tâm giả**: cùng ngày, `crm/note` vẫn còn **2
điểm** (`ListAdvise` qua `c.Query("id")`, `GetNote` qua `c.Param("id")`) — tức repo thật đang ở 3 >
`BASELINE`=1, gate lẽ ra ĐỎ. Lộ ra tình cờ khi `/check-scale` đo `list-advise`, không phải do ai chạy
lại gate. Đã kiểm bằng đối chứng: chạy `scan` của chính gate trên bản `git show HEAD:` của 2 file đó
in ra đúng 2 dòng, chạy trên bản đã vá in ra 0 ⇒ **scanner không hề mù, chỉ là con số trong tài liệu
được chép lại từ report của đợt vá thay vì đọc lại từ gate**. Luật rút ra: mọi câu "đã vá hết / còn
N điểm" phải kèm lần chạy gate NGAY TRƯỚC KHI viết câu đó — report của đợt trước không phải bằng chứng
cho hiện tại.

⚠ **Lớp thứ hai, cùng bug nhưng gate cũ KHÔNG phủ: `c.Query("idCustomer")`.** `c.Param` và `c.Query`
đều trả chuỗi và đi vào cùng một phép ép kiểu của MySQL — chỉ khác cửa vào. Quét riêng ra thêm
**38 điểm** (tất cả đường ĐỌC), gồm `reminder-of-customer`, `check-up-of-customer`,
`service-used-of-customer`… Gate nay phủ cả hai (mở rộng 2026-09-07); với query param thì chỉ soi
param có TÊN là id (`IDISH`), vì `c.Query` còn phục vụ filter/sort/paging mà chuỗi là đúng.

Khác biệt khi vá query param: **giữ NGUYÊN response của guard `== ""` sẵn có**, chỉ thay phép gán
bằng `strconv.ParseUint` rồi đổi điều kiện thành `err != nil || v == 0`. Đổi sang 404 như path param
là đổi hợp đồng API cho một param vốn tuỳ chọn. Chỗ KHÔNG có guard (danh sách con: `GetExportBatches`,
`GetStockCheckBatches`…) thì `_` là đúng: id rác → 0 → trả rỗng, thay vì trước đây trả trúng dữ liệu
của bản ghi khác.

⚠ **Lớp thứ ba, gate KHÔNG phủ: `query.GetFilterValue(c, "customer_id")`.** Scanner chỉ nhận diện
`c.Param`/`c.Query`/`c.DefaultQuery`, nên id đi qua helper filter Yii2-style (`filter[customer_id]`,
`customer_id[eq]`) lọt hoàn toàn — gate xanh KHÔNG chứng minh được gì cho cửa này. Ca thật:
`buildTreatmentCourseOfCustomerQuery` (`crm/customer_detail`), `?filter[customer_id]=12abc` trả liệu
trình của khách 12. Chặn bằng KIỂU: hàm build query trả `customerID uint64`, không `string` — cùng
thủ thuật compiler-gác đã dùng cho service layer.

Cửa này nay CÓ lưới tập trung: `pkg/query/numeric_id_filter.go` — `ApplyFilter` tự chặn giá trị
không parse được cho cột ID thuần số, phủ cả 636 call site mà không sửa handler nào. **Lưới đó
dùng ALLOWLIST, KHÔNG phải quy tắc "tên kết thúc `_id`"** — schema có 36 cột `*_id` kiểu `varchar`
(`template_id`, `page_id`, `client_id`, `icd_id`…) mà giá trị chữ là hợp lệ. Nó trả **danh sách
rỗng**, không trả lỗi; endpoint coi id là điều kiện BẮT BUỘC vẫn phải tự parse để báo lỗi rõ ràng
(khuôn: `WarrantyOfCustomer`) — hai tầng phục vụ hai mục đích, đừng bỏ tầng nào. Số đo + cách thêm
cột vào allowlist: `reference/security-patterns.md` §Ép kiểu im lặng ở `filter[<cột>_id]`.

Khi tự parse: dùng `query.GetFilterValues` (**số nhiều**) + đòi đúng 1 phần tử. Bản số ít trả
**PHẦN TỬ ĐẦU**, nên `filter[customer_id][0]=1&filter[customer_id][1]=2` âm thầm chỉ đọc khách 1 —
sai im lặng, tệ hơn báo lỗi. 3/3 điểm đã vá 2026-09-08 (`care-of-customer`, `WarrantyOfCustomer`,
`buildTreatmentCourseOfCustomerQuery`); gate giữ mức:
`grep -rn 'ParseUint(query.GetFilterValue(' --include='*.go' internal/` phải = **0**.

Param id **không tên `id`** (`assignmentId`, `customerId`, `rid`, `version_number`) dùng
`helpers.ParseParamID(c, "<tên>")` — `ParseID` nay chỉ là lớp mỏng gọi nó.

### Parse fail thì REJECT — không được hạ cấp xuống nguồn id khác (biến thể thứ 5 của lớp IDOR body-id)

Bốn biến thể "pin id" đã ghi trước đây đều nói chuyện *reset `id` client gửi kèm trước khi ghi*.
Biến thể này khác hẳn và lọt qua hết mọi lần rà: id **lấy đúng chỗ, đúng thứ tự ưu tiên** — chỉ hỏng
ở **nhánh lỗi**.

```go
// ❌ path param thua IM LẶNG khi không parse được ⇒ body/query thắng
var body struct{ ID uint `json:"id" form:"id"` }
_ = c.ShouldBindQuery(&body)
if body.ID == 0 { _ = c.ShouldBindJSON(&body) }
if parsed, err := strconv.ParseUint(c.Param("id"), 10, 64); err == nil {
    body.ID = uint(parsed)          // ← err != nil thì GIỮ NGUYÊN id của client
}
// POST /crm/note/abc/pin-note?id=<note của người khác> → ghim đúng note trong query

// ✅ path param là nguồn DUY NHẤT; body chỉ còn dữ liệu nghiệp vụ
id, ok := helpers.ParseID(c)
if !ok { response.Validate(c, i18n.T(c, "missing_id_or_pinned")); return }
var body struct{ Pinned string `json:"pinned" form:"pinned"` }
```

Luật: **route đã có `:id` thì id CHỈ đến từ path**; xoá luôn field `ID` khỏi struct body để compiler
chặn đường cũ, đừng chỉ đảo thứ tự gán. Dấu hiệu nhận diện khi đọc code: một `if err == nil` bọc
quanh phép gán id — nghĩa là tồn tại một nhánh mà id đến từ nơi khác.

Ca thật: `crm/note` `PinNote` (vá 2026-09-07). Neo bằng
`note_id_integration_test.go::TestPinNote_IgnoresBodyID` — gửi path rác + `?id=<note nạn nhân>`, assert
422 **và** assert note nạn nhân vẫn chưa bị ghim (chỉ assert status code là chưa đủ: bản lỗi trả 200
nhưng thứ cần chứng minh là DB không đổi).

⚠ Hai bẫy đã dính khi quét (cùng lớp "lệnh không khớp thì im lặng thành công"):
- Regex chọn file viết `c\.Query\("[^"]*[Ii][Dd]"\)` đòi tên param **kết thúc** bằng id ⇒ bỏ lọt
  sạch nhóm `idCustomer` (bắt đầu bằng id). Phải khớp CẢ hai đầu: `(^|_)(id)$|^id`.
- Self-test của gate chỉ có ca `c.Param`, nên phần mở rộng sang `c.Query` **PASS mà chưa chứng minh
  được gì**. Đã bổ sung 2 ca sai (`BadQueryVar`, `BadQueryInline`) + 2 ca đúng (`status` filter, đã
  ParseUint) vào chính self-test — mở rộng scanner mà không mở rộng self-test là tự lừa mình.

⚠ Đừng suy ra "mọi `c.Param` đều phải parse": param mà kiểu chuỗi là ĐÚNG nghiệp vụ (`token`,
`code`, `slug`, `phone`…) thì giữ nguyên — danh sách này nằm ở `ALLOW_PARAM` trong script.

## Security patterns (CRITICAL)

Xem `security-patterns.md` — tenant scope, field lock/permission, transaction+defer, mass assignment, optimistic lock → 409.

## Performance — Loop trong 1 vòng đời API

### ORDER BY alias phải khớp với JOIN alias trong query
Khi đổi tên alias trong JOIN (vd. `emr_medical_record` → `pivot_mr` / `leg_mr`), phải cập nhật ORDER BY tương ứng — MySQL báo `Unknown column` nếu alias không tồn tại:
```go
// ❌ Sai — alias cũ sau khi đã refactor JOIN
query.Order("COALESCE(emr_medical_record.admission_date, appointment.appointment_date) DESC")

// ✅ Đúng — dùng alias đã khai báo trong JOIN
query.Order("COALESCE(pivot_mr.admission_date, leg_mr.admission_date, appointment.appointment_date) DESC")
```

### In-memory cache cho lookup lặp lại trong loop
Khi xử lý batch (import, bulk update), nếu mỗi row cần lookup DB theo 1 FK (e.g. `emr_department_id → department_type`), dùng cache map trước loop thay vì query N lần:

```go
// ✅ Cache lookup — N rows, tối đa K query (K = số giá trị FK unique)
cache := map[int64]int{}
for _, row := range rows {
    id := helpers.ToInt64(row["foreign_key_id"])
    if id <= 0 { continue }
    if val, ok := cache[id]; ok {
        row["resolved_field"] = val
    } else {
        var val int
        db.Table("lookup_table").Select("field").Where("id = ?", id).Scan(&val)
        if val > 0 {
            cache[id] = val
            row["resolved_field"] = val
        }
    }
}

// ❌ Không làm — query DB mỗi row
for _, row := range rows {
    resolveSomethingFromDB(db, row) // query bên trong → N query
}
```

**Khi nào áp dụng:** Import Excel, bulk CUD, bất kỳ loop nào gọi DB với FK có thể trùng lặp.
**Cache chỉ sống trong 1 request** — không dùng global/Redis cho trường hợp này.

## (đã bỏ, D15) `GetCompanyActiveIDWithQuery` / fallback `GetCompanyIDs` — mô hình multi-company cũ

Trước D15 (1 thương hiệu/DB), endpoint cần FE truyền `company_id` tường minh qua query dùng
`middleware.GetCompanyActiveIDWithQuery(c)` (validate qua `IsCompanyAllowed`), và READ/LIST scope có
chuỗi fallback `companyActive` → `GetCompanyIDs` để không reject user chưa switch company. Toàn bộ
2 helper này (`GetCompanyActiveIDWithQuery`, `GetCompanyIDs`) đã xoá cùng schema `company_id` (story-62)
— hệ thống giờ 1 thương hiệu/DB, không còn khái niệm "company user được gán". Trục còn lại chỉ là
`branch_id`, dùng thẳng `middleware.GetBranchActiveID(c)`/`GetBranchIDs(c)`/`IsBranchAllowed(c, id)` —
xem §Tenant scope. Endpoint CUD thông thường chỉ cần `GetBranchActiveID`; READ/LIST vẫn theo
`ScopedDBNullBranch` (đã tự fallback `GetBranchIDs` khi không có branch active), KHÔNG cần pattern
`company_required`/fallback riêng nào nữa.

---

## Lưu mảng con trong payload update — CẤM delete-all-recreate

- **KHÔNG xoá sạch dòng chi tiết rồi tạo lại từ payload.** Dùng **upsert theo `id` + merge theo key
  presence**: dòng cũ load từ DB → chỉ gán field CÓ MẶT trong payload (`if v, ok := row["x"]; ok`,
  không `else`) → `Updates` với `Select(<cột tường minh>)`; dòng cũ không còn trong payload thì
  `DELETE ... WHERE <parent>_id = ? AND id IN (...)`; payload rỗng mới xoá sạch.
- **Lý do**: delete-all-recreate làm **field client không gửi bị mất vĩnh viễn, im lặng** — không lỗi,
  không cảnh báo. Đã gây **6 ca mất dữ liệu thật trên production** ở module med (mất file đính kèm phiếu
  nhập; mất `batch_id` khiến duyệt phiếu xuất không trừ kho → tồn sai thật; mất toàn bộ liều dùng + thang
  YHCT khi sửa đơn thuốc). Client nào cũng dính, không riêng form web.
- **Điều kiện tiên quyết**: client PHẢI gửi `id` từng dòng. Không gửi → mọi dòng thành "mới" → thoái hoá
  về đúng delete-all mà build xanh + test qua. Sửa BE kiểu này BẮT BUỘC kiểm payload thật của mọi client.
- **Danh sách cột `Select(...)` phải đủ và tường minh**, khai thành biến + có test khoá. Thiếu cột =
  cột đó không bao giờ lưu được, im lặng (đã xảy ra: `bulkUpdateDetails` cũ chỉ ghi 3/10 cột). Cột do
  luồng khác quản (`received_quantity`, `dispensed_quantity`, `approved_*`, giá/tiền do `OnDispense`)
  KHÔNG được nằm trong danh sách.
  (`PartitionDetailRows`, `ParseDetailRowID`).

- **[CRITICAL] Handler kiểu "seed struct từ DB → `ShouldBindJSON` → `Save()`": PHẢI pin lại field hệ
  thống từ record DB TRƯỚC `Save()`.** `Save()` ghi TOÀN BỘ field của struct, mà `ShouldBindJSON` cho
  client ghi đè bất kỳ field nào có tag json — kể cả field chỉ hệ thống được set. Hệ quả không chỉ là
  dữ liệu sai: client gửi `null` cho chính cột dùng làm ĐIỀU KIỆN GUARD sẽ **vô hiệu hoá guard đó ở
  request kế tiếp**. Ca thật (story-04 `med-debt-cleanup`): guard "chỉ sửa đơn đặt mua khi chưa nhận
  hàng" dựa trên `stock_import_id IS NULL`; client gửi `"stock_import_id": null` là xoá liên kết và mở
  lại cửa.
  → Khi thêm guard dựa trên một cột, **luôn kiểm cột đó có ghi được từ payload không**.

- **[CRITICAL] Pin tay KHÔNG hiệu lực với field CON TRỎ — dùng `helpers.BindUpdate`.**
  `encoding/json` ghi **XUYÊN QUA** con trỏ non-nil (chỉ cấp phát mới khi field đang `nil`). Nên mọi cách
  "chụp lại rồi gán ngược" đều vô nghĩa với `*uint`/`*int`/`*string`/`*decimal.Decimal`:
  - `orig := item.X` … `item.X = orig` → `orig` giữ chính địa chỉ vừa bị payload sửa.
  - `a, b := item.X, item.Y` … (gán tuple) → như trên.
  - `body := struct{ dao.X; … }{X: item}` rồi `body.F = item.F` → **2 biến DÙNG CHUNG con trỏ**
    (scanner gọi là `PIN-ALIAS`).
  - Truyền `orig` con trỏ vào hàm khác làm "giá trị hiện tại" (hàm audit nhận giá trị hiện tại) →
    nhánh fallback trả về chính giá trị đã hỏng.
  Field kiểu **giá trị** (`uint`, `int64`, `string`) thì pin tay vẫn đúng.
  → **Cách duy nhất đúng cho mọi kiểu**: `helpers.BindUpdate(c, &item, "CreatedBy", …)`
  (`pkg/helpers/bind_update.go`) — **deep-copy** trước bind, tự nhận JSON/multipart, LUÔN giữ `ID`
  (`preserve` là cộng thêm). Handler có binder multipart THỦ CÔNG (allowlist `c.PostForm`, ngữ nghĩa
  partial-update) → `helpers.BindUpdateWith`, **KHÔNG** thay bằng `c.ShouldBind` (zero-out field không gửi).
  ⚠ Từng có nhiều chỗ trong repo "trông như đã vá" (có code pin + comment giải thích) mà thật ra chưa bao
  giờ bảo vệ được gì — gồm cả `UpdateCustomerPrescription`, chỗ tài liệu này từng nêu làm mẫu.

- **Lane thứ 2 — multipart**: gin form-binding **không đọc tag `json`**; thiếu tag `form` thì nó dùng
  **TÊN FIELD** làm key → form field tên `ID` ghi đè được `item.ID`. Mọi entity nay có `form:"-"` ở field
  `ID` (phòng thủ chiều sâu). Lưu ý gin so khớp **chuỗi tuyệt đối** `== "-"`, nên `form:"-,omitempty"`
  KHÔNG được hiểu là bỏ qua.

- **Handler CREATE**: bind payload vào entity rồi `Create` mà không `item.ID = 0` → client tự chọn khoá
  chính. Luôn reset PK trước khi tạo.

- **[QUAN TRỌNG] Helper chỉ làm sạch `dst` — biến NGUỒN vẫn bị payload sửa.** Với khuôn
  `body := struct{ dao.X; Details … }{X: item}`, sau `BindUpdateWith(&body.X, …)` thì `body` đúng nhưng
  **`item` vẫn bẩn** (ô nhớ con trỏ dùng chung không được chữa). Hệ quả 2 chiều, đã gặp thật:
  - Đọc `item.<field con trỏ>` SAU bind = đọc giá trị của client. Ca thật: guard
    guard đọc `item.<field>` phải **chuyển lên TRƯỚC bind** mới đúng.
  - So sánh `*body.F != *item.F` **luôn bằng nhau** (2 alias) → nhánh điều kiện không bao giờ chạy. Ca
    thật: `UpdateCustomerPrescription` re-verify tenant + resolve lại `company_id` chưa bao giờ chạy.
    Fix: chụp giá trị vào biến riêng TRƯỚC bind rồi so với nó.
  → Sau bind, chỉ đọc `dst`; cần giá trị cũ thì chụp trước bind. Scanner KHÔNG bắt được dạng này.

- **Lưới**: `python3 scripts/body-id-scan.py` (có `SELF_TEST`, exit 2 nếu script hỏng). Chạy tay trước khi
  merge PR đụng `internal/modules/**/handler/**`. Giới hạn đã biết ghi trong docstring của script.

---

## Thêm resource mới
Xem `skill/skills/writing-backend-resources/SKILL.md`

---

## Tham chiếu thêm
- **Code examples thực tế**: `reference/examples.md`
- **Security patterns đầy đủ**: `reference/security-patterns.md`

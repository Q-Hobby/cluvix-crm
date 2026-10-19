---
name: working-on-config
description: >-
  Quy tắc và pattern nghiệp vụ Config Cluvix còn sống trong CRM standalone: mẫu in
  (print_template), control field trong bảng (control_field_in_table), IndexedDB sync
  của config. Dùng skill này KHI làm feature/sửa bug đụng các module đó — trước khi đọc
  code. Tổng đài/chữ ký số/AI config đã bỏ (chỉ còn mô tả lịch sử trong skill, KHÔNG
  áp dụng).
---

# Working on Config

> ⚠ **(đã bỏ, D15/CRM standalone)** Verify 2026-09-28: `internal/modules/webapp/config/` trong repo
> CRM standalone này KHÔNG có `config_call_center`, `config_digital_signature`,
> hay module `ai` — chỉ còn `branch`, `business_profile`, `appointment_*`, `control_field_in_table`,
> `print_template` (CÒN SỐNG — section mẫu in bên dưới đã cập nhật theo code 2026-10-17),
> `options`, `service`/`service_type`, danh mục (`customer_category`, `source`...), `hr_department`,
> `indexeddb`, `user_profile`, `userscope`... Các section bên dưới nói về tổng đài/Omicall, chữ ký
> số, AI config mô tả kiến trúc ERP cũ — giữ làm lịch sử/tham khảo, KHÔNG áp dụng cho repo
> này. Mọi mention `company_id`/`GetCompanyActiveID`/`companyActiveId` trong các section đó cũng đã
> bỏ cùng company (D15) — xem `CLAUDE.md` §Hard Rules cho model hiện tại (branch-only).
>
> reference/call-center-fe.md: FE tổng đài · reference/omicall-api.md: Omicall REST API ·
> reference/omicall-sdk.md: Omicall SDK.



Module `config/config_call_center` — cấu hình tổng đài per company. Mỗi company có thể cấu hình nhiều provider, chỉ 1 active tại 1 thời điểm.

## Bảng `config_call_center`

Nhiều record per company. `company_id IS NULL` = shared template (Core ERP, áp dụng cho tất cả công ty chưa config riêng).

| Column | Type | Ghi chú |
|---|---|---|
| `company_id` | `INT UNSIGNED NULL` | NULL = shared template; có giá trị = riêng company đó |
| `name` | `VARCHAR(255) NOT NULL` | Tên nhận dạng |
| `provider_type` | `TINYINT UNSIGNED` | Xem constants bên dưới |
| `config_system` | `JSON` | Credentials tổng đài — **SENSITIVE** |
| `config_ext` | `JSON` | Danh sách máy nhánh `[{userId, ext, phonenumber, password}]` |
| `config_other` | `JSON` | Cấu hình phụ — **SENSITIVE** |
| `status` | `TINYINT` | `1` = active, `0` = inactive |

## Entity — `dao/entity.go`

```go
type ConfigCallCenter struct {
    models.SimpleModel
    CompanyID    *uint          `gorm:"column:company_id;uniqueIndex:uq_company_name;index" json:"company_id"`
    // ...
}
```

**QUAN TRỌNG**: `CompanyID` phải là `*uint` (pointer) vì column nullable. `uint` zero=0≠NULL → GORM ghi 0 vào DB → orphaned data. Mọi check phải dùng `item.CompanyID != nil && *item.CompanyID > 0`.

## Provider type constants — `pkg/define/call_center.go`

```go
CallCenterProviderVoip24h  = 1   // config_other.secret
CallCenterProviderStringee = 2   // config_system.API_SID, config_system.Secret_Key
CallCenterProviderOmicall  = 3   // config_system.api_key, config_system.sipRealm
```

**KHÔNG hardcode số** — dùng `define.CallCenterProvider(item.ProviderType)`.

## Scope rule

- Dùng `tenant.ScopedDBNullBranch(c, h.DB, "config_call_center")` — row `company_id IS NULL` luôn được trả về
- GET/UPDATE/DELETE by id: `ScopedDBNullBranch(...).First(&item, id)` trước, rồi mới dùng `item.CompanyID`
- **KHÔNG** `db.DB.First(&item, id)` trực tiếp

## Active/Inactive logic — QUAN TRỌNG

Mỗi company chỉ có 1 provider active. Pattern đúng (sau khi CompanyID đổi sang `*uint`):

```go
var item dao.ConfigCallCenter
if err := tenant.ScopedDBNullBranch(c, h.DB, "config_call_center").First(&item, body.ID).Error; err != nil {
    response.NotFound(c); return
}
// Deactivate others in same company scope
if item.CompanyID != nil && *item.CompanyID > 0 {
    h.DB.Table("config_call_center").Where("company_id = ? AND id != ?", *item.CompanyID, item.ID).Update("status", 0)
} else {
    h.DB.Table("config_call_center").Where("company_id IS NULL AND id != ?", item.ID).Update("status", 0)
}
h.DB.Table("config_call_center").Where("id = ?", item.ID).Update("status", 1)
```

**Lý do KHÔNG dùng `GetCompanyActiveID(c)`**: trả về company user đang active, nhưng row có thể thuộc company khác hoặc IS NULL → 0 rows affected mà GORM không báo lỗi.

## Create — dùng LAST_INSERT_ID để reload

Raw INSERT (để `company_id` ghi được NULL) rồi reload theo `LAST_INSERT_ID` thay vì query theo
name+provider_type (tránh nhầm khi trùng name). Mọi đường tạo đi qua `insertConfigCallCenter`
(`handler/helpers.go`) — INSERT + `LAST_INSERT_ID` trong CÙNG 1 `db.Transaction`. Lý do + test đồng thời:
`skill/skills/debugging-backend-queries/reference/patterns-gotchas-data-integrity.md` §`LAST_INSERT_ID()`.

## Credential masking — `handler/helpers.go`

Pattern: show 6 ký tự đầu + `****`. Áp dụng cho tất cả GET responses và IndexedDB sync.

```go
maskItemCredentials(&item)   // mutate in-place trước khi trả response

// Khi update: restore nếu FE gửi lại masked value
mergedCS, mergedCO := mergeCredentials(item.ProviderType, item.ConfigSystem, body.ConfigSystem, item.ConfigOther, body.ConfigOther)
```

**Fields bị mask theo provider:**
- Stringee: `config_system.API_SID`, `config_system.Secret_Key`
- Omicall: `config_system.api_key` (KHÔNG mask `sipRealm` hay `hotlines`)
- Voip24h: `config_other.secret`

`mergeCredentials` tự động giữ nguyên giá trị gốc nếu FE gửi lại masked value.

**updateConfigCallCenterByProvider phải gọi cả 2:**
```go
// UPDATE path: merge trước
mergedCS, mergedCO := mergeCredentials(item.ProviderType, item.ConfigSystem, body.ConfigSystem, item.ConfigOther, body.ConfigOther)
// ...sau khi save:
maskItemCredentials(&item)
response.Updated(c, item)
```

## List endpoint — response format

```go
// ĐÚNG: c.JSON trực tiếp (BaseApiService.list() không unwrap .data)
c.JSON(http.StatusOK, result)

// SAI: response.Data(c, result) → wrap thêm {"data": ...} → FE nhận items=undefined
```

## Routes (17 endpoints)

```
GET  /config-call-center                              list + paginate
GET  /config-call-center/get-call-center-active       lấy provider đang active
GET  /config-call-center/get-config-call-center-by-id
GET  /config-call-center/get-config-voip24h           provider_type=1
GET  /config-call-center/get-config-stringe           provider_type=2
GET  /config-call-center/get-config-omicall           provider_type=3
GET  /config-call-center/omicall/extensions           proxy lấy danh sách máy nhánh từ Omicall API
GET  /config-call-center/get-token-stringee           tạo JWT token Stringee cho FE
GET  /config-call-center/:id
POST /config-call-center                              create
POST /config-call-center/update-config-call-center-stringee
POST /config-call-center/update-config-call-center-voip24h
POST /config-call-center/update-config-call-center-omicall
POST /config-call-center/active
POST /config-call-center/in-active
PUT  /config-call-center/:id
DELETE /config-call-center/:id
```

## Omicall extensions proxy — `handler/omicall_client.go`

BE làm proxy để FE không cần biết `api_key`. Pattern chung:

```go
// Package-level client — tái sử dụng TCP connections
var omicallHTTPClient = &http.Client{
    Timeout:   15 * time.Second,
    Transport: http.DefaultTransport,
}

// baseURL đọc từ config_system.base_url; fallback về omicallDefaultBaseURL
// Truyền baseURL vào tất cả helper functions, KHÔNG dùng global const trực tiếp
func omicallGetToken(baseURL, apiKey string) (string, error)
func omicallListExtensions(baseURL, token string) ([]OmicallExtItem, error)
func omicallExtPassword(baseURL, token, sipUser string) (string, error)
func resolveOmicallExtPasswords(baseURL, apiKey string, extList []map[string]any)
```

### base_url per tenant

Omicall có thể chạy trên nhiều server. Mỗi tenant cấu hình `base_url` riêng trong `config_system`:

```go
var sys struct {
    APIKey  string `json:"api_key"`
    BaseURL string `json:"base_url"`  // optional — để trống dùng default
}
baseURL := sys.BaseURL
if baseURL == "" {
    baseURL = omicallDefaultBaseURL  // "https://public-v1-stg.omicall.com"
}
```

`base_url` được lưu vào `config_system` chỉ khi FE nhập — `buildPayload` dùng spread:
```typescript
config_system = {
  api_key: v.Omicall_ApiKey,
  sipRealm: v.Omicall_SipRealm,
  hotlines,
  ...(v.Omicall_BaseURL?.trim() ? { base_url: v.Omicall_BaseURL.trim() } : {}),
};
```

### resolveOmicallExtPasswords — parallel, best-effort

List API trả `password: "XXXXXX"` (masked). BE tự resolve bằng `extensions/detail`. Mỗi extension có `sipRealm` riêng từ `pbx_account.sip_realm`.

```go
type omicallExtDetail struct {
    Password string
    SipRealm string
}

// Condition: resolve khi thiếu password HOẶC thiếu sipRealm
if ext != "" && (pw == "" || sipRealm == "") {
    jobs = append(jobs, job{idx: i, ext: ext})
}

// Goroutine: chỉ overwrite password nếu trước đó rỗng; luôn set sipRealm
if detail.Password != "" {
    existing, _ := extList[idx]["password"].(string)
    if existing == "" {
        extList[idx]["password"] = detail.Password
    }
}
if detail.SipRealm != "" {
    extList[idx]["sipRealm"] = detail.SipRealm
}
```

**QUAN TRỌNG**: Condition phải check cả `sipRealm == ""`. Nếu chỉ check `pw == ""`, extension đã có password sẽ bị skip → `sipRealm` không bao giờ được set.

Gọi trong `UpdateConfigCallCenterOmicall` — sau `mergeCredentials`, trước khi save:
```go
resolveOmicallExtPasswords(burl, sys.APIKey, extList)
// Void return — KHÔNG block save nếu Omicall auth fail
```

`config_ext` schema sau khi resolve:
```json
[{ "ext": "116", "userId": 2, "password": "SwtxyVRQOA", "sipRealm": "dend", "phonenumber": null }]
```

### GetOmicallExtensions — auth fail trả 422, không 500

```go
token, err := omicallGetToken(baseURL, sys.APIKey)
if err != nil {
    // Config issue (sai key) → Validate, không ServerError
    response.Validate(c, i18n.T(c, "call_center_not_found")+" ("+err.Error()+")")
    return
}
```

## IndexedDB sync — `internal/modules/webapp/config/indexeddb/handler/sync.go`

`listCallCenters` nằm trong `customTables` (không phải `configTables`) để mask credentials:

```go
"listCallCenters": func(c *gin.Context, d *gorm.DB) interface{} {
    rows := findRows(tenant.ScopedDBNullBranch(c, d, "config_call_center").Table("config_call_center").Order("id DESC"))
    maskCallCenterCredentials(rows)
    return rows
},
```

`maskCallCenterCredentials` unmarshal `json.RawMessage` → mask key → re-marshal:
- Stringee: mask `config_system.API_SID`, `config_system.Secret_Key`
- Omicall: mask `config_system.api_key`
- Voip24h: mask `config_other.secret`

**`provider_type` type assertion** — MySQL TINYINT scan ra `map[string]interface{}` có thể là bất kỳ numeric type nào (`int64`, `int8`, `uint8`, v.v.), **không phải lúc nào cũng `int64`**. Dùng `toInt64()` helper thay vì direct assertion:
```go
// SAI — silent zero khi không phải int64
pt, _ := row["provider_type"].(int64)

// ĐÚNG
pt := toInt64(row["provider_type"])
```

Sau CUD: `dexie.BumpDexieTable(h.DB, "listCallCenters")`

---

## FE patterns

→ Xem `skill/skills/working-on-config/reference/call-center-fe.md` — provider form, extensions dropdown, popup-call SDK, hotline selector, browser permissions.


Module `config/digital_signature` — cấu hình ký số CA per company.

## Bảng `config_digital_signature`

1 record per company (`company_id UNIQUE`). Upsert qua `POST /config/digital-signature/config`.

| Column | Type | Ghi chú |
|---|---|---|
| `sign_method` | `ENUM('manual','digital')` | `manual` = ký tay scan; `digital` = CA |
| `provider` | `VARCHAR(50)` | lowercase: `matbao`, `vnpt`, `viettel`, `fpt`, `bkav`, `newtel`, `ca2`, `misa` |
| `signing_method` | `VARCHAR(20)` | `USB_TOKEN` / `HSM` / `CLOUD` |
| `is_active` | `TINYINT(1)` | 1 = đang dùng |
| `credentials_enc` | `TEXT` | AES-GCM encrypt toàn bộ JSON credentials |
| `sign_config` | `JSON` | `{sign_page, sign_x, sign_y, sign_width, sign_height, sign_image_base64}` |

## Credentials JSON blob pattern

Credentials là JSON map `map[string]string` encrypt bằng `crypto.EncryptAESGCM`:

```go
// Matbao
{"taxcode": "...", "username": "...", "password": "..."}

// Generic API provider
{"api_endpoint": "...", "api_key": "...", "serial_number": "..."}
```

**KHÔNG dùng cột cố định per provider** — tránh migrate mỗi khi thêm provider mới.

## Masking pattern — secret fields

GET trả `credentials_masked` (không trả `credentials_enc`). Secret fields (`password`, `api_key`) bị mask 8 ký tự đầu + `***`. Non-secret (`taxcode`, `username`, `api_endpoint`, `serial_number`) trả plain.

```go
var secretFields = map[string]bool{"password": true, "api_key": true}

func maskSecret(field, value string) string { ... }          // mask nếu secret
func isMaskedVersion(field, input, original string) bool { } // true → FE gửi lại masked → giữ nguyên
```

Upsert merge logic:
```go
if isMaskedVersion(k, incomingVal, existingVal) {
    merged[k] = existingVal  // giữ nguyên
} else {
    merged[k] = incomingVal  // cập nhật
}
```

**CRITICAL:** FE gửi masked value → BE giữ giá trị cũ, không cần FE xử lý gì thêm.

## Response shape

```json
{
  "sign_method": "digital",
  "provider": "matbao",
  "signing_method": "CLOUD",
  "is_active": true,
  "has_credentials": true,
  "credentials_masked": {
    "taxcode": "0987654321",
    "username": "admin",
    "password": "mypass12****"
  },
  "sign_config": { "sign_page": 1, "sign_x": 50, "sign_y": 700, "sign_width": 150, "sign_height": 50 }
}
```

`has_credentials: bool` — shorthand để FE check nhanh mà không cần đọc `credentials_masked`.

## Dùng ở đâu

`sign.go` trong `emr/medical_record/handler` đọc config này khi digital signing:

```go
h.DB.Table("config_digital_signature").
    Select("provider, credentials_enc, sign_config").
    Where("company_id = ? AND sign_method = 'digital' AND is_active = 1", middleware.GetCompanyActiveID(c)).
    First(&dsCfg)
```

→ Decrypt credentials → `creds["taxcode"]`, `creds["username"]`, `creds["password"]`

## Validation

- `sign_method=digital` → `provider` bắt buộc, phải thuộc set hợp lệ
- Tạo mới `digital` mà không có credentials → 400
- `signing_method` nếu có → phải là `USB_TOKEN` / `HSM` / `CLOUD`


## Bảng DB

`print_templates` — `branch_id` (0 = mẫu chung toàn thương hiệu), `template_type`, `name`, `description`, `template_html` (LONGTEXT), `is_default`, `is_active`, `created_at`, `updated_at` (optimistic lock, D21), `created_by`, `updated_by`. KHÔNG còn `company_id`/`version` (D15/D21). Entity: `internal/modules/webapp/config/print_template/dao/entity.go`.

## Endpoints

| Method | Path | Mô tả |
|--------|------|-------|
| GET | `/config/print-template` | List (explicit SELECT, không `SELECT *` vì LONGTEXT) |
| GET | `/config/print-template/:id` | Get một record |
| GET | `/config/print-template/default?type=&branch_id=` | Lấy template mặc định |
| POST | `/config/print-template` | Tạo mới |
| PUT | `/config/print-template/:id` | Cập nhật |
| DELETE | `/config/print-template/:id` | Xóa (chặn nếu is_default=1) |

## GetDefault logic

Ưu tiên: branch cụ thể (`branch_id` query param, parse ra số trước) → fallback mẫu chung `branch_id=0`, cùng điều kiện `template_type = ? AND is_default = 1 AND is_active = 1`. Không lọc theo company (đã bỏ, story-39/D15). List/Get/Update/Delete theo id vẫn qua `tenant.ScopedDBNullBranch(c, h.DB, "print_templates")`.

## Optimistic locking

Theo `updated_at` (D21): Update gọi `optlock.Save(tx, &current, input.UpdatedAt)` — `WHERE updated_at = expected`, 0 dòng ⇒ `errs.ErrVersionConflict` ⇒ `response.Conflict` (409, i18n `version_conflict`). FE PHẢI gửi lại `updated_at` đã đọc. (Cột `version` cũ đã bỏ.)

## Template types

Định nghĩa tại `pkg/define/print_template.go` → `ValidPrintTemplateTypes` map. Thêm type mới → update map này.

## Render engine (FE)

Handlebars-lite tại `print-template-render.service.ts`:
- `{{key.path}}` — interpolation
- `{{#each array}}...{{this.field}}...{{/each}}` — loop
- `{{#if condition}}...{{/if}}` — conditional

Standalone function `renderTemplateHtml(html, data)` dùng cho preview.

## FE — PrintTemplatePage

- Page riêng (không dùng dialog) tại `/config/print-template/create` và `/config/print-template/:id`
- Monaco editor (`ngx-monaco-editor-v2`) NGOÀI `<form>` — tránh CVA `setDisabledState` gotcha
- Sync Monaco → FormControl thủ công qua `editor.onDidChangeModelContent`
- Placeholder sidebar: click → `editor.executeEdits()` chèn tại cursor position
- Live preview: `<iframe #previewFrame sandbox="allow-same-origin">` set `srcdoc` qua `effect()`

## FE — PrintPayment (ví dụ tích hợp)

`PrintTemplateRenderService.getDefault('payment', branchId)` → nếu có tpl → render + `renderedHtml.set(html)` → `[innerHTML]="renderedHtml() | safeHtml"`. Nếu null → fallback hardcode Angular template.

## Template data shape (payment)

```typescript
{
  company: { name, hotline, website, logo },   // key giữ tên `company` cho template cũ; giá trị lấy từ authBusinessProfile() (business_profile, D22)
  branch: { name, address, hotline, email },
  customer: { fullname, phone, email, address, code },
  payment: { code, date, note, total, outstanding },
  services: [{ name, quantity, unit_price, subtotal, discount, total }],
  print: { date, day, month, year },
}
```


Table `control_field_in_table`: 2 static row, KHÔNG có `company_id`/`branch_id` (global, không tenant-scoped by design — seed 1 lần trong migration, không CRUD thêm row).

- `id=1` "Bảng khách hàng" → form Contact/CustomerForm
- `id=2` "Bảng khách hàng (Lead)" → form Lead/FormLead

Column `control` (`datatypes.JSON`): mảng `{field_name, field_label, create, update, edit}`.
- `edit=true` = **"Không được cập nhật"** (field bị khoá khỏi update), KHÔNG phải "cho phép sửa".

## GOTCHA — 2 format lưu trữ khác nhau cho cùng cột `control`

DB có thể lưu 1 trong 2 dạng cho cùng cột:
- Plain array: `[{...}, {...}]`
- Wrapped: `{"control": [{...}, {...}]}`

Mọi nơi đọc field này PHẢI thử unwrap cả 2 dạng, không được assume 1 format cố định:
```go
var fields []map[string]interface{}
if json.Unmarshal(raw, &fields) != nil {
	var wrapper map[string]json.RawMessage
	if json.Unmarshal(raw, &wrapper) == nil {
		if inner, ok := wrapper["control"]; ok {
			_ = json.Unmarshal(inner, &fields)
		}
	}
}
```
Áp dụng ở: BE `handler.GetFieldCustomer`, BE `CustomerService.applyFieldLocks` (xem `skill/skills/working-on-crm/reference/customer.md`), FE `customer-form.ts loadValidationRules()`.

## Routes (`internal/modules/webapp/config/control_field_in_table/`)

- `GET :id/get-field-customer` — trả `control` đã parse, hoặc `defaultCustomerFields` hardcode nếu `control` rỗng
- `POST :id/update-field-customer` — nhận `{control: [...]}`, lưu nguyên array vào cột `control`
- `POST :id/re-build` — set `control = NULL` (reset về default)

Không có service layer — handler gọi thẳng `h.DB`. Sau CUD: `dexie.BumpDexieTable(h.DB, "controlFieldInTables")`.

FE consumer chính không gọi trực tiếp GET endpoint — `customer-form.ts loadValidationRules()` đọc qua IndexedDB cache (`ControlFieldInTableCacheService`).

## KNOWN GAP — chưa fix

`route_perm_map.json` ĐÃ map đủ 3 route — cả 3 dùng `manageConfig` (quyền duy nhất của module Cấu hình từ 2026-09-02), `enforce_phase1:true` ở `update-field-customer` + `re-build` — nhưng enforcer toàn cục đang audit-mode (`PERM_ENFORCE=false`) → chưa chặn thật. Bất kỳ user đã login vẫn gọi thẳng `update-field-customer`/`re-build` được, ảnh hưởng TOÀN hệ thống vì bảng global (không `company_id` để scope theo tenant) — `re-build` reset `control` về NULL cho mọi company.

Không thêm `requirePerm` riêng cho 3 route này để "vá sớm" — gap chung của cả module config, xem `skills/backend-hard-rules/reference/security-patterns.md` §Field lock / permission.




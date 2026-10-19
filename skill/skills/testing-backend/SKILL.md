---
name: testing-backend
description: >-
  Hạ tầng test Go: unit test, integration test với TEST_MYSQL_DSN (MySQL disposable thật,
  KHÔNG dockertest), FreshTenant helper, HTTP handler test qua httptest, và mutation test
  (`go test -overlay`) để chứng minh test thật sự neo hành vi. Dùng skill này TRƯỚC
  KHI viết test mới cho backend Go, khi cần biết cách dựng DB test sạch mỗi lần chạy,
  trước khi báo một AC dạng phủ định/mặc định là Done, hoặc
  trước khi verify build/test theo Development Protocol bước 4 (Validate).
---

# Testing (Go) — Unit + Integration Test Infrastructure


## Tool
- Test framework: stdlib `testing` + `github.com/stretchr/testify` (assert/require)
- DB integration: `TEST_MYSQL_DSN` env trỏ MySQL disposable thật (KHÔNG dockertest/container) — Suite tự DROP toàn bộ table rồi apply lại schema mỗi lần chạy
- Shared infrastructure: `internal/testhelpers/` — Suite, fixtures, gin context builder

⚠ **Thiếu `TEST_MYSQL_DSN` → suite SKIP nhưng `go test` VẪN in `ok <pkg> 0.5s` — XANH GIẢ, không chứng minh gì.**
Cùng hậu quả khi DSN CÓ nhưng tên DB không kết thúc `_test`/`_tmp` (guard từ chối, vd `cluvix_qa2_svc` ⇒ `ok` + 44 SKIP). Luôn đối chiếu số `=== RUN` với số test kỳ vọng, đừng đọc mỗi chữ `ok`.

⛔ **Tên DB trong DSN BẮT BUỘC kết thúc `_test`/`_tmp`** (hậu tố, không phải tiền tố) —
`testhelpers.New` từ chối chạy nếu không: nó `dropAllTables` = DROP MỌI BẢNG, DB-per-customer nên
trỏ nhầm = mất sạch data 1 khách. Vì sao fail-closed bằng quy ước tên (guard cũ no-op nhiều
tháng) + lớp hook Bash: `reference/disposable-db-guard.md`.

Kết luận "test pass" chỉ hợp lệ khi có đủ: DSN đã set · `-count=1` (kết quả `(cached)` cũng không tính) ·
đếm được số `--- PASS` thật. Đã có lần dùng `ok 0.799s` của một suite skip sạch làm bằng chứng hoàn thành.

## Build tag — tách unit vs integration

```bash
# Pure tests (không cần DB, ~0.7s)
go test ./...

# Full suite (cần TEST_MYSQL_DSN trỏ MySQL disposable thật)
TEST_MYSQL_DSN='root:pass@(localhost:3306)/cluvix_test?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true' go test -tags=integration ./...
```

Nhiều package chung 1 DB test → BẮT BUỘC `-count=1 -p 1` khi chạy nhiều package cùng lúc (chạy song song sẽ đụng schema DROP/apply lẫn nhau).

- ⚠ **Nhiều agent/phiên cùng chạy `-tags=integration` (mỗi phiên 1 DB riêng nhưng cùng MySQL instance)**: `dropAllTables`/apply schema là DDL — nhiều suite song song có thể làm DDL MySQL nghẽn **>10 phút**, vượt timeout mặc định `go test` (10m) khiến `TestMain` chết ngay giữa chừng dù code không sai. Gặp timeout ngay ở `TestMain` mà không rõ lý do khi biết có agent khác đang chạy integration test song song → chạy lại với `-timeout 40m` khi MySQL rảnh hơn, không phải lỗi thật.

- `testhelpers.New` tự thêm `multiStatements=true` nếu DSN thiếu (`applySchema` chạy cả file `.up.sql` như MỘT `Exec`). Trả `ErrNoDSN` khi chưa set DSN — `TestMain` PHẢI phân biệt: `errors.Is(err, testhelpers.ErrNoDSN)` → skip; lỗi khác → `log.Fatalf`. Chỉ `log` rồi `m.Run()` cho cả hai là cách suite integration hỏng đi qua CI với dấu `ok`.

- File pure tests: KHÔNG có build tag → luôn compile và chạy
- File integration tests: `//go:build integration` ở dòng đầu → chỉ compile khi `-tags=integration`
- TestMain (Docker setup) đặt trong `main_integration_test.go` — không có tag thì package không có TestMain, chạy thẳng

## ⛔ `-race` đắt gấp ~4.7× — BẮT BUỘC `-p 1`, KHÔNG quét cả `./<module>/...`

Sự cố lặp lại nhiều lần (đo lại 2026-08-31, máy 16GB/8 core): agent tuân thủ đúng
`GOFLAGS=-p=4` của INDEX.md mà máy user vẫn đơ cứng. Rule `-p=4` được hiệu chỉnh cho
build THƯỜNG; nó không hề bảo vệ khi có `-race`.

Đo trực tiếp, cùng package `./internal/modules/webapp/<module>/service/`, chỉ khác 1 cờ:

| Lệnh | Peak RSS cả cây build | Test binary |
|---|---|---|
| `go test -c ...` | **0.98 GB** | 32 MB |
| `go test -c -race ...` | **4.60 GB** | 40 MB |

Phép nhân dẫn tới sập: `go test -race ./internal/modules/webapp/<module>/...` = 13 package
có test × `-p=4` song song ⇒ **4 × 4.6 = ~18 GB từ MỘT lệnh của MỘT agent**. Nền máy dev lúc
rảnh đã 11 GB (GoLand 4.0 + node AngularService 1.7 + mysqld 1.1 + Chrome 2.5 + các phiên
claude 1.8) ⇒ chỉ còn ~5 GB headroom. 2 agent QA song song ⇒ ~47 GB nhu cầu trên 16 GB RAM →
memory compressor nén ~10 GB, swap cạn, máy đơ. Trong Activity Monitor ở chế độ **hiển thị
phân cấp**, toàn bộ `compile`/`link`/test binary bị cộng dồn vào dòng `Terminal` — nên hiện
tượng user thấy là "terminal chiếm 40GB", KHÔNG phải terminal rò rỉ bộ nhớ.

**Luật:**

1. Có `-race` ⇒ **luôn `-p 1`**. Headroom ~5 GB chỉ đủ đúng 1 đơn vị 4.6 GB.
   ```bash
   go test -race -p 1 -count=1 ./internal/modules/webapp/<module>/service/
   ```
2. `-race` **chỉ chạy trên package thật sự có concurrency** (goroutine/channel/mutex/map dùng
   chung) — vd package có manager + goroutine. KHÔNG
   quét `./<module>/...`: các package CRUD thuần trả 4.6 GB mà không mua được gì.
3. **KHÔNG 2 agent cùng chạy `-race`.** Race phải xếp TUẦN TỰ, khác với build thường vốn chạy
   song song được.
4. Trước wave có `-race`: hạ nền trước. Đóng GoLand hoặc tắt Angular language service khi đang
   làm BE — riêng 2 thứ đó trả lại 5.7 GB, gấp đôi headroom.

Ngân sách RAM để tự tính `-p` (máy 16 GB): `(RAM_trống_GB) / (4.6 nếu có -race, 1.0 nếu không)`,
làm tròn XUỐNG, tối thiểu 1. Đo RAM trống thật bằng `vm_stat` / `sysctl vm.swapusage` —
`swap used` sát trần và `Pages stored in compressor` cao là dấu máy đang ngạt, dừng fan-out ngay.

## File layout chuẩn cho 1 package có DB tests

Service-level tests đặt cạnh service file:
```
internal/modules/webapp/crm/service/                  # domain-level shared services
  payment_test.go                  # pure tests, no tag
  main_integration_test.go         # //go:build integration — TestMain
  payment_integration_test.go      # //go:build integration — DB tests
  treatment_course_test.go         # //go:build integration — DB tests
```

Hoặc cho resource-level service:
```
internal/modules/webapp/crm/customer/service/
  service.go
  service_integration_test.go      # //go:build integration
```

## TestMain template (file `main_integration_test.go`)

```go
//go:build integration

package mypackage

import (
    "log"
    "os"
    "testing"

    "gorm.io/gorm"
    "erpCluvix/internal/testhelpers"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
    s, err := testhelpers.New(testhelpers.MigrationsDir())
    if err != nil {
        log.Printf("testhelpers: %v — skipping integration tests", err)
        os.Exit(m.Run())
    }
    defer s.Close()
    testDB = s.DB
    os.Exit(m.Run())
}
```

## SchemaMode — chọn migrations cần apply

| Mode | Apply gì | Khi nào dùng |
|---|---|---|
| `SchemaBaseline` (default) | từ story-62 GIỐNG `SchemaAll` (schema mới 1 bảng/1 file, không còn file baseline) | đa số tests |
| `SchemaAll` | mọi `*.up.sql` trừ seed (`*_seed_*`) — DB trắng không dữ liệu | tests cần schema đầy đủ |
| `SchemaFull` | tất cả gồm seed | hiếm — chỉ khi cần data seed (chi nhánh mặc định, `admin`…) |

```go
testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
```

## Test isolation — `FreshTenant(t, db)`

KHÔNG share state giữa tests. Mỗi test tạo tenant riêng, auto cleanup qua `t.Cleanup()`:

```go
func TestSomething(t *testing.T) {
    if testDB == nil { t.Skip("Docker not available") }

    tenant := testhelpers.FreshTenant(t, testDB)
    // tenant.BranchID, tenant.CustomerID (TestTenant KHÔNG còn CompanyID — D15/story-60)
    // Records tự xóa khi test kết thúc — an toàn cho t.Parallel()
}
```

Ghi record kèm `branch_id` tường minh từ client đi qua `middleware.IsBranchAllowed` (branch phải nằm trong
`GetBranchIDs` của user; danh sách rỗng = không giới hạn). (`IsBranchAllowedInCompany` + bước gắn
`branch.companyId` cũ đã xoá cùng company, story-41.)

⚠ `defer` chạy TRƯỚC `t.Cleanup` ⇒ đóng pool/server bằng `defer` làm cleanup hỏng IM LẶNG: `reference/test-isolation.md`.

⚠ Mỗi `FreshTenant` tạo branch + customer **MỚI** ⇒ 2 tenant khác nhau ở MỌI tầng cùng lúc. Guard so nhiều vế (khách + chi nhánh…) PHẢI có ca **chỉ lệch đúng 1 vế** — khác khách cùng chi nhánh: `testhelpers.CreateScopedCustomer(t, db, tn, &tn.BranchID)` (hoặc `nil` cho khách chưa gắn chi nhánh). Thiếu ca đó thì vế chi nhánh luôn chặn trước, vế khách không bao giờ được kiểm (mutation tắt nó vẫn xanh).

## Fixtures có sẵn (`internal/testhelpers/fixtures.go`)

```go
// Tạo tenant tự cleanup
tenant := testhelpers.FreshTenant(t, db)

// Tạo tenant share (không cleanup) — dùng trong TestMain
tenant, err := testhelpers.CreateTestTenant(db)

// Tạo records test
payID, _ := testhelpers.CreatePayment(db, tenant, decimal.NewFromInt(100), define.PaymentForDeposit)
testhelpers.SetCustomerStatus(db, tenant.CustomerID, define.CustomerStatusCustomer)
```

**GORM Create từ map KHÔNG backfill id** — `db.Table(x).Create(&map)` không ghi id ngược vào map, `map["id"]` luôn giá trị cũ (thường 0). Lấy id thật qua `testhelpers.LastInsertID(db)` (an toàn vì Suite giới hạn 1 connection).

**`service.department_type` DB default = `1` (SpecialtyDental → FlowPercent), KHÔNG phải 0/NULL** — tạo fixture `service` cho test flow Beauty/FlowCount PHẢI set tường minh `"department_type": define.SpecialtyBeauty` (hoặc specialty FlowCount khác), nếu không toàn bộ nhánh FlowCount không chạy (rơi nhầm vào Dental) mà test vẫn có thể PASS-giả nếu assertion không đủ chặt (phát hiện qua debug print khi viết integration test cho `order-service-quantity-numtreatment-fix`).

**Assert PHỦ ĐỊNH trên bảng rỗng = lưới rỗng** — `COUNT(*) WHERE x <> 0` = 0 trên schema `testhelpers` vừa dựng **luôn đúng** (bảng chưa có dòng nào). Test khoá migration/backfill phải **tự seed fixture bẩn trước**; kiểm bằng mutation vô hiệu hoá câu `UPDATE` (`WHERE 1 = 0`) ⇒ phải ĐỎ, còn xanh là chưa từng đo gì (2026-09-01, `TestStory08_NoStaleAvailabilityAfterMigration`).
## Gin context cho handler tests

```go
c := testhelpers.TenantCtx(branchID, userID)
// c đã set: branchActiveId, branchIds, userID
// (đã bỏ, D15/story-41: companyID/companyActiveId/companyIds — company đã xoá khỏi session)
// Dùng được với tenant.ScopedDBNullBranch(c, db) và middleware.Get* helpers
```

## Pure-function tests — KHÔNG cần DB

Test logic không động DB (validators, pure functions, early-return guards): pass `nil` cho `*gorm.DB` và viết file KHÔNG có build tag:

```go
// payment_test.go — luôn chạy với go test ./...
func TestCreatePayment_NegativeAmountRejected(t *testing.T) {
    svc := NewPaymentService(nil)
    p := &crmModels.Payment{Amount: decimal.NewFromInt(-500)}
    err := svc.createPayment(nil, p, 0)
    require.Error(t, err)
}
```

## DB tests — luôn check `testDB == nil`

```go
//go:build integration

func TestVersionConflict(t *testing.T) {
    if testDB == nil { t.Skip("Docker not available") }
    tenant := testhelpers.FreshTenant(t, testDB)
    // ... test logic
}
```

**Handler dùng `GET_LOCK` KHÔNG test được qua Suite** — Suite giới hạn 1 connection (`SetMaxOpenConns(1)`), nhưng `GET_LOCK` cần checkout connection riêng → deadlock tới timeout. Skip test đó, verify bằng gọi API thật (curl) thay vì integration test. Ngoại lệ: guard/validate đứng TRƯỚC lock vẫn test được qua Suite — cho request dừng ở bước trước lock (dữ liệu bị từ chối hợp lệ, vd `work_date` sai) rồi assert mã mong đợi. Khuôn: `hr_ot_request/handler/guard_http_test.go` `TestOtUpdate_DoiUserIDCanManage`.

**Handler dùng `pagination.Paginate` — TestMain PHẢI set `db.DB = testDB` (package-global), không chỉ biến local `testDB`** — `pagination.Paginate` đọc global `erpCluvix/internal/db.DB`, KHÔNG dùng connection/scopedDB truyền vào query. Test không set global này sẽ khiến handler query nhầm DB (DB thật hoặc DB rỗng) dù `testDB`/tx truyền vào đúng.

**Service bắn goroutine chạy TRỄ cũng đọc `db.DB` + `logger.L` toàn cục — SAU khi test đã xong** (vd goroutine `time.AfterFunc` bắn sau khi service trả về; ví dụ cũ `TriggerCustomerCLVRecompute` đã bỏ 2026-10-17). Hai biến đó nil trong test ⇒ panic làm đỏ CẢ package dù test đã PASS. Set `db.DB = testDB` + `logger.Init(...)`, và trong cleanup `time.Sleep(3 * <độ trễ goroutine>)` TRƯỚC khi trả lại `db.DB`/xoá dữ liệu. Khuôn: `crm/service/order_customer_code_integration_test.go`.

## White-box vs black-box

- Test method unexported (vd `beforeSave`): dùng `package mypackage` (white-box)
- Test API public (vd `Update`): dùng `package mypackage_test` (black-box)
- Mặc định white-box trong tests cùng package — đỡ phải export thêm

## Quy tắc chung

- **NEVER mock DB** — dùng container thật. Mock đã từng pass nhưng prod migration fail.
- FK checks tự động disable bởi Suite (`SET SESSION foreign_key_checks = 0`, `suite.go`) → fixtures không cần resolve toàn bộ object graph.
- ⛔ **Hệ quả bị bỏ quên: test kiểm HÀNH VI của FK (`ON DELETE CASCADE`/`SET NULL`) sẽ XANH GIẢ.**
  Suite dùng đúng MỘT connection nên `SET SESSION` áp cho cả phiên: cascade **không chạy**, và test
  "xoá cha ⇒ con biến mất" pass vì lý do hoàn toàn khác (con vẫn còn, nhưng assert sai chiều thì
  không ai biết). Phải tự bật lại — và bật **SAU khi seed xong**, vì seed cần checks tắt (fixture
  không dựng cả đồ thị company/branch):
  ```go
  func withForeignKeyChecks(t *testing.T) {
      t.Helper()
      if err := testDB.Exec("SET SESSION foreign_key_checks = 1").Error; err != nil { t.Fatal(err) }
      t.Cleanup(func() { testDB.Exec("SET SESSION foreign_key_checks = 0") })
  }
  ```
  Đo được 2026-08-25 (`omi_message_reaction`): bật rồi thì test CASCADE đỏ đúng lúc FK còn thiếu, tắt thì luôn xanh.
- ⛔ **Test "tính hash rồi verify" là self-consistent — KHÔNG chứng minh được tương thích ngược.**
  Đổi cách marshal thì cả hai vế cùng đổi, test vẫn xanh, trong khi record đã ký trong DB khách lại
  verify FAIL. Muốn chặn hồi quy chữ ký phải so **dạng cũ vs dạng mới** trực tiếp (bytes/hash), hoặc
  neo golden hash cứng. Đổi kiểu field nằm trong canonical map (vd `int` → `*int`) BẮT BUỘC qua gate
  này trước — `go build` và test hash sẵn có đều không bắt được.
- So sánh cột JSON trước/sau UPDATE phải lấy mốc bằng cách **đọc lại từ DB**, KHÔNG dùng struct vừa `Create`: MySQL chuẩn hoá JSON khi lưu (`{"days":30}` đọc ra `{"days": 30}`) → so với bản in-memory báo "field bị đổi" giả, dễ tưởng là regression của code vừa viết.
- Mỗi DB test tạo `FreshTenant` riêng → an toàn parallel, không leak state.
- Tests pure (không cần Docker) phải luôn pass — đó là "smoke test" cho mọi commit.

## Lưu ý quan trọng

- File integration test có tag `//go:build integration` — KHÔNG đặt TestMain trong file no-tag (sẽ compile cho cả 2 modes → conflict)
- `FreshTenant` xóa theo thứ tự: payment → customer → branch → company. Nếu tests tạo records ở bảng khác (vd: appointment, treatment_course), thêm DELETE vào `t.Cleanup` thủ công.
- Migration baseline ~378KB — apply ~1-2s. SchemaAll thêm vài migration nhỏ nữa, SchemaFull thêm 678KB seed (~5-10s). Dùng baseline khi đủ.
- Khi viết test cho service mới: kiểm tra signature có nhận `*gin.Context` không. Nếu chỉ nhận `*gorm.DB` (như PaymentService, CustomerService) → đỡ phải build context, test sạch hơn.

## HTTP handler tests

Handlers dùng `Deps` pattern (xem `skill/skills/writing-backend-resources/reference/patterns-arch.md §Handler DI`):

```go
type Deps struct { DB *gorm.DB; Store storage.Store }
type Handler struct { Deps; XxxSvc *services.XxxService }
func New(d Deps) *Handler
```

Trong test: inject `testDB` qua `Deps`, test ở tầng HTTP với `httptest` thật.

```go
h := New(Deps{DB: testDB})
```

### Khi nào dùng HTTP tests vs service tests

| HTTP handler test | Service test |
|---|---|
| Routing / path param parsing | Business logic |
| Middleware interaction (tenant, auth) | DB queries, version conflict |
| JSON serialization / status code | Pure validation, early-return guards |

### File layout

```
internal/modules/<domain>/<resource>/handler/
  main_integration_test.go    # //go:build integration — TestMain (cùng pattern services)
  payment_http_test.go        # //go:build integration — HTTP-level tests
```

Reference thực tế: `internal/modules/webapp/crm/payment/handler/{main_integration_test.go, payment_http_test.go}`.

### AuthMiddleware — inject tenant context

File: `internal/testhelpers/http.go`

```go
r.Use(testhelpers.AuthMiddleware(branchID, userID))
```

Middleware này set đủ các key mà `tenant.ScopedDBNullBranch` và `middleware.Get*` helpers cần, tương đương `testhelpers.TenantCtx` nhưng dùng được với `gin.Engine`.

⚠ **(đã bỏ, D15/CRM standalone)** Trước đây còn set `companyActiveId` — đã bỏ cùng module `client/`.

### Ví dụ

```go
//go:build integration

package crm

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "erpCluvix/internal/testhelpers"
)

func TestGetPayment_NotFound(t *testing.T) {
    if testDB == nil {
        t.Skip("Docker not available")
    }
    h := New(Deps{DB: testDB})        // Deps pattern, not legacy NewHandler(db)
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(testhelpers.AuthMiddleware(1, 1, 1))
    r.GET("/:id", h.GetPayment)

    req := httptest.NewRequest(http.MethodGet, "/99999", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    assert.Equal(t, http.StatusNotFound, w.Code)
}
```

### Chạy

```bash
go test -tags=integration ./internal/modules/webapp/crm/payment/... -v
```

## Bắt lỗi SQL bị NUỐT (handler không kiểm `.Error`)

Nghi handler nuốt lỗi SQL nhưng response vẫn 200/201 → **gắn `gorm.Logger` tuỳ biến vào session** rồi gọi
endpoint thật qua `httptest`: GORM **luôn** gọi `Logger.Trace(ctx, begin, fc, err)` bất kể caller có kiểm
`.Error` hay không.

Mạnh hơn mutation: quan sát sự thật đang xảy ra, không phải dựng một sự thật giả để đối chiếu — dùng được
cả khi HTTP status hoàn toàn bình thường.

Ca thật: `CreateCustomerWarrantyCode` chạy `Update("SortOrder", ...)` lên cột không tồn tại
(1054), `.Error` bị bỏ, POST vẫn trả `201 Created`. Xem thêm `skill/skills/backend-hard-rules/SKILL.md` §"Lỗi SQL bị nuốt".

---

## Test phải chạy ở tầng handler THẬT gọi, không chỉ hàm lõi

Hàm lõi test xanh **không** chứng minh hành vi đúng: wrapper mà handler thực sự gọi có thể chạy một
validate khác **trước**, và thắng.

- Mỗi guard/validate phải có ít nhất 1 test gọi đúng hàm handler dùng, không chỉ hàm nội bộ.
- Đặt guard nghiệp vụ **trước** kiểm hạ tầng (`s3c == nil`, `h.DB == nil`) → test được mà không cần
  hạ tầng: truyền `nil` client là đủ, và file xấu không bao giờ đi tới bước ghi.

Ca thật (2026-08-13, `pkg/imaging`): guard HEIC đúng ở `ProcessFile` và test xanh, nhưng production đi
qua `ProcessAndUpload` — hàm này chạy `storage.ValidateFileType` trước, `image/heic` không nằm trong
whitelist nên user nhận "loại file không được phép" thay vì "đổi sang JPG". Cùng epic, lỗi lặp lại lần 2
khi `SaveUploadSmart` kiểm `s3c == nil` trước guard.

## Mutation test — chứng minh test có NEO hành vi

Cấy lỗi vào code sản phẩm rồi chạy lại — test **phải đỏ**. Đỏ 0/N mutation = cả lớp test đang vô dụng dù coverage đẹp.

Cách chạy (`go test -overlay`), 10 hình dạng "test xanh vì lý do sai" đã gặp thật (mới nhất: **test dùng dạng URL mà lớp parse KHÔNG đọc được** ⇒ mutation không đỏ vì test chẳng chạm tới bản vá), và các bẫy khi cấy mutation: `reference/mutation-testing.md`. Chứng minh refactor code nhiều tác dụng phụ cho kết quả y hệt (2 clone + băm mọi bảng): cùng file.

## Test DryRun — khoá HÌNH DẠNG SQL của query list, không cần DB

`gorm.Open(mysql.New(mysql.Config{Conn: sql.OpenDB(<connector từ chối kết nối>), SkipInitializeWithVersion: true}),
&gorm.Config{DryRun: true, DisableAutomaticPing: true})` — `sql.OpenDB` lười nên GORM dựng SQL rồi
dừng, test chạy mọi máy/CI mà không đụng database. Khuôn: `crm/customer/handler/list_query_test.go`.

- ⛔ **Vô giá trị** — GORM `.Count()` tự lo, nên assert vẫn XANH kể cả khi cho câu đếm nhận thẳng
  query danh sách: "COUNT không có `ORDER BY`", "COUNT không mang danh sách cột".
- ✅ **Có giá trị**: "COUNT không có **JOIN**" — `.Count()` **GIỮ NGUYÊN** join, và join thừa là thứ
  trả tiền mỗi lần load. Cùng loại: cổng rẽ nhánh tự viết, điều kiện `1 = 0` khi tập rỗng, và
  "câu đếm và câu danh sách CÙNG mệnh đề `WHERE`".
- Test phải gọi **CHÍNH hàm sản phẩm** dựng query, không dựng lại query trong test — bản chép luôn
  xanh khi code sản phẩm bị cấy lỗi.
- Handler dùng `Raw().Scan()`/`Rows()`: DryRun trả `ErrDryRunModeUnsupported` nhưng **logger vẫn nhận
  SQL** ⇒ gắn `gorm.Config{Logger: <logger bắt Trace>}` rồi assert. Handler gọi `FailQuery` ⇒ phải
  `logger.Init(logger.Config{Env: "test"})` trước, không thì panic.
- `Pluck`/`Find` dưới DryRun trả RỖNG, KHÔNG lỗi ⇒ test "lỗi SQL thì rơi về đường cũ" phải cấy lỗi:
  `db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) { tx.AddError(err) })`.
- Câu đếm chạy trước thì DryRun dừng ở đó, câu SELECT sau không bao giờ bắt được ⇒ assert "không còn
  X" trên câu sau luôn XANH GIẢ. Cần cả handler chạy hết ⇒ **driver giả trả 0 dòng** thay DryRun
  (`emptyConnector` trong `accountant/payment/handler/list_payment_sql_test.go`), kèm 1 assert đối chứng
  câu cuối (`LIMIT 20`) có chạy. Quyết định (cổng, hint) vẫn tách hàm thuần để test bảng.
- ⚠ `strings.Contains(s, "IN (SELECT")` KHỚP cả `JOIN (SELECT` ⇒ assert "không còn `id IN (subquery)`" đỏ oan
  trên chính khuôn đúng — so kèm cột: `"id IN (SELECT"`.
- Handler export ghi file `./storage/export/` ⇒ `t.Chdir(t.TempDir())` trước khi gọi.
- DryRun KHÔNG thực thi ⇒ luôn kèm 1 integration test cho phần runtime, nhất là khi handler tái
  dùng một `*gorm.DB` cho hai truy vấn (đếm rồi liệt kê): khoá `_meta.totalCount` bằng số dòng
  THẬT khớp bộ lọc, không phải số dòng của trang.

## Key i18n dùng-mà-THIẾU render ra chuỗi thô

`i18n.T` fallback trả về **chính key** khi không tìm thấy (`pkg/i18n/i18n.go:33`) → user thấy
`file_size_exceeded` trên màn hình. Build xanh, test không bắt.

- Sau khi thêm key: đối chiếu key dùng trong module với **cả 4** file `vi/en/jp/fil`.
- Key chỉ có ở `vi` → locale khác lặng lẽ rơi về tiếng Việt, không lộ ra như lỗi.

```bash
# liệt kê key dùng trong module rồi grep ngược vào từng file i18n
grep -rhoE 'i18n\.T[FL]?\(c, "[a-z0-9_]+"' <module> | grep -oE '"[a-z0-9_]+"'
```

## Smoke endpoint thật trên `air` — HTTPS, KHÔNG tự spawn server

`air` đã chạy sẵn port 8081 — tự `go run` là đụng port, kill lẫn nhau.

- Air phục vụ **HTTPS**: `curl -sk https://localhost:8081/api/...`. Gọi `http://` trả `400 Client sent an HTTP request to an HTTPS server`.
- `JWTAuth` chỉ verify chữ ký + JTI revoke ⇒ access token do user cấp dùng thẳng được, không cần dựng phiên.
- Request có tới handler hay không: đọc dòng `[GIN]` trong `backend/tmp/run.log`.
- Smoke có GHI DB/S3 → chỉ chạy trên tài khoản dev của chính user và khôi phục trạng thái cũ ngay sau đó.

## Integration test — MỘT DB dùng-một-lần cho MỖI package, "ok" nhiều package là kết quả rỗng

`go test -tags=integration ./<module>/...` chạy nhiều package **song song trên cùng một
`TEST_MYSQL_DSN`**. Mỗi `TestMain` gọi `testhelpers.New` = `dropAllTables` + migrate ⇒ chúng **đè
nhau** → `testhelpers.New` lỗi → `testDB = nil` → **mọi test trong package đó `t.Skip`** → in ra `ok`.
Output không có một dấu hiệu nào. Đo được thật (2026-08-18, story-13 zalo-personal-core):

```
./<module>/...        → ok handler 4.0s      ← xanh giả
./<module>/handler/   → FAIL (panic)         ← chạy riêng mới lộ
```

**Hệ quả về cách đọc kết quả**: câu "integration PASS" lấy từ một lệnh chạy NHIỀU package chung DSN
là **kết quả rỗng, không được tin** — kể cả khi nó xanh, kể cả khi nó chạy lâu.

Chạy đúng cách:

```bash
backend/scripts/verify_integration_isolated.sh ./internal/modules/webapp/<module>/service                                                ./internal/modules/webapp/<module>/handler
```

Script tự tạo DB `itv_<n>` riêng cho từng package, chạy TUẦN TỰ, drop sau khi xong, và **coi
`skip > 0` hoặc `run = 0` là FAIL** (không có test nào chạy cũng là hỏng, không phải "sạch"). Đặt
`ITV_PREFIX` khi nhiều phiên chạy song song để tên DB không đụng nhau. KHÔNG bao giờ trỏ vào
`DB_NAME` trong `.env`.

**Luật đọc chung**: mọi con số verify phải kèm điều kiện sinh ra nó. `skip=0` là một phần của bằng
chứng, không phải chi tiết phụ — thiếu nó thì "294 test PASS" và "294 test SKIP" trông giống hệt nhau.

- ⛔ `go test` **không in dòng `--- SKIP` nếu thiếu `-v`** ⇒ `grep -c SKIP` trên output không-`-v`
  LUÔN trả 0. Dùng nó làm đối chứng cho "không test nào bị bỏ" là một đối chứng không thể khác 0.
- Tự viết lệnh verify ⇒ bắt buộc `-v`, và in **cả `PASS` lẫn `SKIP` lẫn `FAIL`**: `FAIL=0` không kèm
  `PASS=n` thì "không có gì hỏng" và "không có gì chạy" trông y hệt nhau.
- Đừng tự viết: `backend/scripts/verify_integration_isolated.sh` đã làm đúng việc này.

## Lưới AST — khoá TẬP HÀM được phép gọi một hàm ghi, và nguồn nạp của chúng

Bất biến kiểu "chỉ N hàm này được gọi X" ⇒ test quét AST khoá **tập hàm chứa lời gọi**, không phải test
từng call site: điểm gọi thứ N+1 là đỏ ngay.

Vế hai bắt buộc: mọi hàm **nạp dữ liệu** cho các call site đó phải chứa cột mà X rẽ nhánh theo.

Đặt lưới ở test **KHÔNG cần build tag** — bất biến quan trọng mà chỉ sống sau `-tags=integration` thì
bước verify mặc định không chạy nó.

Quét `.Select(...)` phải soi **đối số của lời gọi**, không quét string literal cả file: payload JSON/stream chứa
cùng tên cột ⇒ đỏ oan.



# Gotchas CRITICAL

## Mục lục

- Dọn tài nguyên phải bằng `defer`, KHÔNG bằng nhánh `if err` (CRITICAL)
- Manual branch filter — KHÔNG dùng IsCrossBranchEnabled (CRITICAL)
- FK lookup — PHẢI dùng CrossBranchLookup, KHÔNG ScopedDBNullBranch (CRITICAL)
- `GetBranchActiveID` vs `GetCompanyID` — Company đã xoá, luôn dùng Branch Active (HIGH)
- Handler — dùng `HandleError` thay `ServerError` cho service errors (HIGH)
- Delete guard — dùng service, không scope branch (HIGH)
- i18n message — KHÔNG nhúng `err.Error()` vào response (HIGH)
- Sync-list handler: DELETE phải chạy TRƯỚC early-return khi list rỗng (CRITICAL)
- Body OPTIONAL — KHÔNG `_ = c.ShouldBindJSON(&input)`
- validation.Required — int 0 = rỗng (khác Angular)
- Struct-bind JSON body: field `*uint`/`*int` optional không thể "xoá" qua `if input.X != nil`
- FULLTEXT `ngram` ở quy mô lớn ném MySQL 188 → HTTP 500 (CRITICAL) — kèm: chỉ mục ngram dựng sai stopword làm tìm kiếm SAI ÂM THẦM (CRITICAL)
- Dòng TỔNG / `COUNT` / export tự dựng lại `WHERE` ⇒ trôi lệch khỏi bộ lọc của danh sách (HIGH)

---


Tenant scope → bind request → response/i18n. Gotcha khác:
`patterns-gotchas-datetime.md` (timezone, cột DATE) · `patterns-gotchas-data-integrity.md`
(transaction, lock, FK, decimal, domain) · `patterns-gotchas-gorm*.md` · `patterns-gotchas-http.md` ·
`patterns-gotchas-routing.md`

## Dọn tài nguyên phải bằng `defer`, KHÔNG bằng nhánh `if err` (CRITICAL)

Hàm giữ tài nguyên (file tạm, con trỏ DB, lock) mà **gọi callback của caller** ⇒ dọn bằng `defer`
kèm cờ `committed` disarm khi thành công. Callback của caller **có thể panic**, mà panic unwind bỏ
qua MỌI nhánh `if` — chỉ `defer` sống sót.

```go
committed := false
defer func() { if !committed { _ = f.Close(); _ = os.Remove(tmpPath) } }()
if err := write(f); err != nil { return err }   // `write` gọi xuống mapper của caller
...
committed = true
```

⚠ Chỉ test một nhánh là LỌT: nhánh dựng-sẵn-trong-RAM panic *trước* khi vào helper nên vẫn xanh,
chỉ nhánh ghi-dần mới lộ ra file tạm còn sót.

## Manual branch filter — KHÔNG dùng IsCrossBranchEnabled (CRITICAL)

`ScopedDBNullBranch` **luôn** áp dụng branch scope — option 18 (cross-branch) không override được.

**KHÔNG bọc branch filter bằng `IsCrossBranchEnabled`** trong bất kỳ handler nào, kể cả handler tự build raw SQL. Branch scope phải được tôn trọng theo phân quyền user.

**Chỉ duy nhất 1 endpoint được phép dùng `IsCrossBranchEnabled`:** `SearchGlobal` (`/crm/customer/search-global`, xem `skill/skills/managing-crm-options/reference/option-crm-options-p2.md` §Option 18) — đúng bản chất "tìm kiếm" của option 18 (FE label "Tìm kiếm & xem chéo dữ liệu giữa các cơ sở"). `visit_history/handler.go` (2 vị trí) và `crm/report/handler/crm.go` (6 report handler qua `buildScopeFilter`) đã BỎ check này (2026-07-28) — sai phạm vi, các endpoint đó không phải search nên luôn branch-scoped bất kể option 18. `workspace/dashboard/handler.go` cũng không dùng (bỏ 2026-07-07).

**Pattern đúng khi có manual branch filter (raw SQL, không qua ScopedDBNullBranch):**

```go
// KHÔNG wrap IsCrossBranchEnabled — branch scope luôn bắt buộc
if branchActiveID := middleware.GetBranchActiveID(c); branchActiveID > 0 {
    filter += " AND (t.branch_id = ? OR t.branch_id IS NULL)"
    args = append(args, branchActiveID)
} else if branchIDs := middleware.GetBranchIDs(c); len(branchIDs) > 0 {
    filter += " AND (t.branch_id IN (?) OR t.branch_id IS NULL)"
    args = append(args, branchIDs)
}
```

---

## FK lookup — PHẢI dùng CrossBranchLookup, KHÔNG ScopedDBNullBranch (CRITICAL)

**Phân biệt 2 loại query:**

| Loại | Ví dụ | Scope đúng |
|---|---|---|
| **Ownership check** — record là chủ thể request | `GET /appointment/:id` → load appointment | `ScopedDBNullBranch` |
| **FK lookup** — load record liên quan qua FK | Load customer của appointment | `tenant.CrossBranchLookup` |

`ScopedDBNullBranch` trên FK lookup sẽ **cắt record** nếu entity liên quan thuộc branch khác → trả `null` dù FK hợp lệ. Example ❌/✅ → Xem `skill/skills/backend-hard-rules/SKILL.md` §Tenant scope.

**Áp dụng cho:**
- Load related entity: customer, order_service_used, payment, treatment_course... qua FK id
- Batch load: `WHERE id IN ?` hoặc `WHERE fk_col IN ?`
- Child records: `WHERE parent_id = ?`, `WHERE treatment_plan_id = ?`
- `AttachRelation`, `AttachRelationArray` (mặc định dùng `CrossBranchLookupScope` nội bộ — không cần override)

**Ownership check giữ `ScopedDBNullBranch`:**
- Main resource load: `GET /:id` → `ScopedDBNullBranch(c, h.DB, "appointment").Where("id = ?", id)`
- Timeline chủ thể: `ScopedDBNullBranch(c, h.DB, "customer").Where("id = ?", id)` — chống IDOR

**Precedent:** appointment list trả `customer = null` sau khi bỏ option 18 khỏi `ScopedDBNullBranch` — vì customer FK lookup vẫn dùng `ScopedDBNullBranch` và customer.branch_id ≠ appointment.branch_id. Fix 2026-06-30.

**Exception — validate FK con khi WRITE (khác load để hiển thị):** phải so khớp trực tiếp `WHERE id = ? AND branch_id = ?` với `branch_id` cụ thể đang lưu ở record cha, KHÔNG dùng `CrossBranchLookup` (không filter gì — chỉ đúng cho hiển thị, không đúng cho xác nhận FK thuộc đúng branch record cha). Bug thật cùng lớp trước D15 (khi trục còn là company): `verifyFKInCompany` (emr_bed, emr_room) dùng helper company-only cho việc validate WRITE, fix 2026-07-08 — cùng cảnh giác áp dụng cho `branch_id` hiện tại.

**`ScopedDBNullBranch`/`CrossBranchLookup` không tự suy FROM-clause:** (1) chỉ dùng để `Count()`/`Distinct()` mà quên chain `.Table("tên_bảng")` tường minh → GORM không xác định được FROM table, lỗi runtime KHÔNG bị `go build` bắt. (2) query dùng alias (`Table("table t")`) mà không truyền alias qua tham số thứ 4 của `ScopedDBNullBranch(c, db, "table", "t")` → điều kiện branch filter build theo tên bảng gốc, không khớp FROM đã alias → list trả rỗng/không lọc âm thầm, không lỗi compile/panic, chỉ lộ khi test tay.

---

## `GetBranchActiveID` vs `GetCompanyID` — Company đã xoá, luôn dùng Branch Active (HIGH)

Rule + example ❌/✅ → Xem `skill/skills/backend-hard-rules/SKILL.md` §Tenant scope (rule "`GetCompanyID` (JWT) bị cấm").

Lý do bổ sung: JWT claim `company_id` **KHÔNG tồn tại** (JWT chỉ có `uid`); trước D15 còn `GetCompanyID` (luôn 0) làm bẫy. `GetBranchActiveID` được middleware load từ DB — `ScopedDB*` dùng nó nội bộ, nhất quán.

⚠ **(đã bỏ, D15)** Trước 1 thương hiệu/DB, mục này còn ghi: precedent `config/emr_ca_config/handler` dùng `GetCompanyID` trả 0 (fix 2026-06-10), và `GetCompanyActiveID` tự fallback `GetCompanyIDs` khi user có ĐÚNG 1 company. Cột `company_id`/2 helper này đã xoá hoàn toàn (story-62/41) — `GetBranchActiveID` không có cơ chế fallback tương đương, dùng thẳng `GetBranchIDs(c)` khi cần danh sách branch user được gán.

---

## Handler — dùng `HandleError` thay `ServerError` cho service errors (HIGH)

`response.ServerError` chỉ dùng cho lỗi DB/internal thật sự. **Không dùng để catch tất cả error từ service** — service có thể trả validation error hoặc sentinel error → sẽ bị map thành 500 sai.

```go
// ❌ NEVER — validation/sentinel error từ service thành 500
if err := svc.Create(&appt, userID); err != nil {
    response.ServerError(c, err)
    return
}

// ✅ ALWAYS — dispatch đúng: validation→422, conflict→409, DB→500
if err := svc.Create(&appt, userID); err != nil {
    response.HandleError(c, err)
    return
}
```

**`HandleError` dispatch:**
- `validator.ValidationErrors` → 422
- `*errs.LocalizedError` → 422
- Sentinel errors (ErrVersionConflict, ErrDoctorScheduleConflict, ...) → i18n.TryT → 409 hoặc 422
- MySQL 1062/1451/... → 500
- Custom validation (`*validation.Errors`) → 422 (qua fallback `Validate(c, key)`)

Precedent: `crm/appointment/handler/appointment.go` — `CreateAppointment` dùng `ServerError` → mọi error từ `validateAppointmentCreate` + `checkDoctorConflict` đều thành 500. Fix 2026-06-10.

---

## Delete guard — dùng service, không scope branch (HIGH)

Guard đếm ràng buộc nghiệp vụ trước khi xoá customer/patient phải đếm **toàn company**, không scope theo branch. FK constraint không phân biệt branch — nếu scope branch, record ở branch khác sẽ không bị đếm, guard pass, DELETE fail với FK 1451.

```go
// ❌ NEVER — record ở branch khác không bị đếm → guard sai
tenant.ScopedDBNullBranch(c, h.DB, "emr_medical_record").Table("emr_medical_record").
    Where("patient_id = ?", id).Count(&count)

// ✅ ALWAYS — đếm toàn bộ theo FK key
h.DB.Table("emr_medical_record").Where("patient_id = ?", id).Count(&count)
```


---

## i18n message — KHÔNG nhúng `err.Error()` vào response (HIGH)

DB error (FK 1451, ...) chứa tên bảng/constraint → leak schema về FE.

```go
// ❌ NEVER — raw SQL error về FE
response.Validate(c, i18n.TF(c, "customer_delete_error", err.Error()))

// ✅ ALWAYS — log chi tiết server-side, FE nhận message chung
logger.L.Error(fmt.Sprintf("DeleteCustomer id=%d: %v", id, err))
response.Validate(c, i18n.T(c, "customer_delete_conflict"))
```

Với `*errs.LocalizedError`, `Error()` trả về **i18n key** (`pkg/errs/common.go:14`) → `response.Validate(c, err.Error())` in thẳng chuỗi `image_dimension_exceeded` ra màn hình. Luôn `response.HandleError(c, err)`.

Precedent: `crm/customer/handler/customer.go` `DeleteCustomer` — BE-038, fix 2026-06-11.

---

## Sync-list handler: DELETE phải chạy TRƯỚC early-return khi list rỗng (CRITICAL)

Pattern "xoá row cũ theo payment_id/parent_id rồi insert lại theo list mới nhất" (vd `order_revenue_*`, `order_commission_*` sync theo `listRevenues`/`listDoctors`/`listDirectSales`):

```go
// ❌ Sai — early-return khi list rỗng nằm TRƯỚC DELETE
if len(list) == 0 {
    return // DELETE chưa chạy — row cũ (member đã bị gỡ khỏi list) còn nguyên
}
tx.Exec("DELETE FROM order_revenue_x WHERE payment_id = ?", paymentID)

// ✅ Đúng — DELETE chạy ngay sau guard đầu, TRƯỚC mọi early-return do list rỗng
tx.Exec("DELETE FROM order_revenue_x WHERE payment_id = ?", paymentID)
if len(list) == 0 {
    return
}
```

**Triệu chứng nếu sai:** member bị gỡ khỏi `listRevenues`/`listDoctors`/`listDirectSales` vẫn giữ nguyên row doanh số/hoa hồng cũ (không bị xoá) — doanh số hiện sai/thừa cho người đã không còn phụ trách. Fix 2026-07-07 ở `payment_commission.go` (6 hàm `handleRevenue*`/`handleXCommission`).

---

## Body OPTIONAL — KHÔNG `_ = c.ShouldBindJSON(&input)`

- Bỏ qua error của `ShouldBindJSON` cho body optional = nuốt cả lỗi SAI KIỂU — handler chạy tiếp với dữ liệu cũ/zero value → hành động sai âm thầm.
- Đúng: chỉ `io.EOF` (body rỗng) là hợp lệ:
  ```go
  if err := c.ShouldBindJSON(&input); err != nil && !errors.Is(err, io.EOF) {
      response.Validate(c, i18n.T(c, "..._invalid")); return
  }
  ```
- Đã cắn: `hr_ot_request` ApproveOtRequest — body `{"hours":"2h"}` bị nuốt, duyệt với hours GỐC của đơn thay vì giờ người duyệt siết lại.

## validation.Required — int 0 = rỗng (khác Angular)

- `pkg/validation` isEmpty: `reflect.Int → v.Int() == 0` — enum có giá trị 0 hợp lệ (vd `IcdTypeNational=0`) KHÔNG được đưa vào `requireFields`/`Required`.
- Angular `Validators.required` pass số 0 → bug chỉ nổ ở BE, FE không thấy gì.

## Struct-bind JSON body: field `*uint`/`*int` optional không thể "xoá" qua `if input.X != nil`

`encoding/json` không phân biệt "key vắng mặt" vs "key: null" khi Unmarshal vào con trỏ — cả 2 đều ra `nil`. Handler pattern `if input.X != nil { item.X = input.X }` (giữ nguyên khi absent) vì vậy KHÔNG BAO GIỜ xoá được field khi user chủ động chọn "không có" (vd bỏ assistant/doctor đã gán). Fix: thêm field `ClearX bool json:"clear_x"` riêng, FE gửi `true` khi cố ý xoá; handler check `ClearX` trước, fallback `input.X != nil` sau. Đã áp dụng: `nursing_note.ClearAssistant`, `treatment_followup.ClearDoctor`.

## FULLTEXT `ngram` ở quy mô lớn ném MySQL 188 → HTTP 500 (CRITICAL)

`MATCH … AGAINST` trên chỉ mục **ngram** ném `Error 188 (FTS query exceeds result cache limit)` khi tập ứng viên lớn — bigram tiếng Việt cực dày (`ca` có mặt ở 52% số dòng), lỗi xuất hiện từ ~1 triệu dòng và phủ hầu hết cụm 2 từ ở 2 triệu.
Không bắt mã này ⇒ rơi vào `pagination.IsQueryError` ⇒ **HTTP 500 cho gần như mọi từ khoá ≥ 2 ký tự**.
Bắt bằng `errors.As(*mysqldriver.MySQLError)` + `Number == 188` rồi thoái hoá xuống `LIKE`. KHÔNG so khớp chuỗi thông báo lỗi.
Tách hàm nhận diện khỏi mã **1191** (thiếu chỉ mục): hai nguyên nhân, hai cách chữa, hai mốc throttle log riêng.
Thoái hoá chỉ là lưới an toàn — `LIKE` trong cửa sổ 180 ngày vẫn ~20s ở 2 triệu dòng. Chữa gốc là **bắt buộc lọc `channel_id`**: 259ms so với 12.679ms.

### Chỉ mục ngram dựng sai stopword → tìm kiếm SAI ÂM THẦM (CRITICAL)

Parser `ngram` loại token nào **CHỨA** stopword (parser mặc định chỉ loại token **BẰNG** stopword), mà danh sách mặc định của InnoDB có `a` và `i` MỘT ký tự ⇒ tiếng Việt thủng lỗ chỗ. Triệu chứng không phải "tìm không ra" mà **vừa sót vừa thừa**: `giao` → bigram gi·ia·ao đều chứa a/i ⇒ MATCH 0 trong khi LIKE 14; `anh` → an·nh, `an` bị loại còn trơ `nh` ⇒ khớp bừa mọi chuỗi có "nh" (465 so với 243).

**Cấu hình stopword gắn theo BẢNG, cố định bởi chỉ mục FULLTEXT ĐẦU TIÊN — không phải theo từng index.** Còn bất kỳ FULLTEXT nào khác trên bảng thì index mới KẾ THỪA stopword BẬT, dù `SET SESSION innodb_ft_enable_stopword = OFF` đã có hiệu lực (`SELECT @@session…` = 0). Đo trên bản sao `customer` 100k dòng: còn `index_customer` → 0/7 từ khoá khớp `LIKE`; bỏ HẾT fulltext trước rồi ADD → 7/7.

- Dựng: bỏ **HẾT** fulltext trên bảng → `SET SESSION innodb_ft_enable_stopword = OFF` **cùng file/kết nối** với `ALTER` (`SET GLOBAL` KHÔNG ăn) → `ADD FULLTEXT … WITH PARSER ngram`.
- Stopword chỉ áp lúc **DỰNG** chỉ mục, không phải lúc truy vấn ⇒ tầng service không cần đổi gì.
- Truy vấn: `AGAINST('"kw"' IN BOOLEAN MODE)` — nháy kép BẮT BUỘC, thiếu thì ngram OR các bigram rời ⇒ thừa kết quả. `NATURAL LANGUAGE MODE` luôn rộng hơn `LIKE` kể cả khi chỉ mục đúng.
- ⚠ **CÙNG một nội dung migration cho kết quả KHÁC NHAU giữa các lần chạy** trên DB dùng-một-lần (7/7 đúng vs 3/10 sai), không tìm ra biến điều khiển ⇒ sau khi apply trên **từng DB khách** phải đối chứng `LIKE` vs `MATCH`; `MATCH` = 0 trong khi `LIKE` > 0 là chỉ mục dựng SAI. Lệch vài % theo hướng `MATCH` > `LIKE` là bình thường (khác biệt xử lý dấu).
- Sai thì `DROP INDEX` — handler bắt lỗi câu `MATCH` rồi rơi về `LIKE` (chậm nhưng ĐÚNG). Không có chỉ mục còn hơn chỉ mục sai. Ngoại lệ: `omnichat/service/search.go` chỉ gọi `MATCH` nên mất chỉ mục là lỗi 1191, phải dựng lại tới khi đối chứng khớp.


## Dòng TỔNG / `COUNT` / export tự dựng lại `WHERE` ⇒ trôi lệch khỏi bộ lọc của danh sách (HIGH)

Handler list nào cũng có ít nhất 3 đường chạy trên **cùng một bộ lọc**: danh sách (+ `COUNT(*)` của
`pagination.Paginate`), export, và dòng tổng `SUM(...)`. Chép `WHERE` lần thứ hai cho dòng tổng là
mở một đường trôi: filter thêm sau này chỉ được gắn vào đường danh sách, người viết không có cách
nào biết là quên.

Thật, `GET /crm/treatment-services/service-used-with-appointment` (2026-09-07): dòng tổng chỉ áp
`created_day` + `appointment_date`, bỏ qua `branch_id` / `source_id` / `created_by` / `created_at` /
`sourceCustomer_id` / `_keyword` và cả 3 filter JSON. Đo trên DB clone 524.288 dòng với bộ lọc ngày
mặc định của FE:

| Lọc thêm | Danh sách | Dòng tổng BÁO |
|---|---|---|
| (không) | 4.402 | 4.402 / 6.678.560.330 ✓ |
| bác sĩ #5 | 284 | 4.402 / 6.678.560.330 ✗ |
| chi nhánh 5 | 258 | 4.402 / 6.678.560.330 ✗ |
| nguồn 3 | 490 | 4.402 / 6.678.560.330 ✗ |
| từ khoá | 42 | 4.402 / 6.678.560.330 ✗ |

Nhìn KHÔNG ra bằng mắt: con số vẫn hợp lý, chỉ là con số của bộ lọc khác. Không có test nào đỏ.

**Rule:** tách `WHERE`/`JOIN` ra một hàm `build<X>Query(c)` rồi **gọi lại hàm đó** cho từng đường —
đừng chép. Mẫu sẵn có trong repo: `buildTreatmentServiceQuery`,
`buildServiceUsedWithAppointmentQuery` (`crm/treatment_service/handler`).

- **KHÔNG gắn `ORDER BY` trong hàm builder.** `sql_mode` có `ONLY_FULL_GROUP_BY`; sort chỉ áp ở
  đường danh sách/export, đường tổng lấy bản chưa sort.
- Mỗi lần gọi builder phải dựng `tenant.ScopedDB*` MỚI, không tái dùng biến `*gorm.DB` cũ
  (xem `patterns-gotchas-gorm.md` §scopedDB reuse).
- **Đừng nuốt lỗi câu tổng**: `Scan(&agg)` hỏng để `agg` ở zero-value ⇒ FE hiện tổng = 0, nhìn y
  hệt "bộ lọc không có dữ liệu". Check `pagination.IsQueryError(tx.Error)` → `pagination.FailQuery`.

Đối chiếu khi sửa: chạy ≥5 bộ lọc khác nhau, số dòng của dòng tổng phải **khớp đúng** số dòng danh
sách. Repo có 298 endpoint dùng `pagination.Paginate` — mẫu này còn lặp lại ở nơi khác.

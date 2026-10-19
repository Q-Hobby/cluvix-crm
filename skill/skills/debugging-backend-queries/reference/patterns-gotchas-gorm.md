# Gotchas GORM v2 — read/query

## Mục lục

- `ScanRows` với dest `map` — quét đúng 1 dòng, kiểu Go không đổi
- GORM v2 — Scan vào `*interface{}` (CRITICAL)
- GORM v2 — Scan 1 cột JSON/text vào `[]byte` — fail âm thầm, không panic (CRITICAL)
- Go: `new(expression)` — compile error hoặc silent zero (CRITICAL)
- Transaction error — KHÔNG dùng `_ =` (CRITICAL)
- Preload — bắt buộc kèm ORDER BY khi có `sort_order` (HIGH)
- Subquery trong WHERE không được thừa hưởng tenant scope từ outer query
- `only_full_group_by` — cột non-aggregated trong SELECT với GROUP BY
- Subquery / Session reuse gotchas — tách file
- Thêm field mới vào cached struct → zero value = production outage khi deploy
- `datatypes.JSON` — KHÔNG gọi `.UnmarshalJSON(field)` (HIGH)
- LEFT JOIN — driving table phải là bảng luôn tồn tại
- GORM v2 — Scan vào `*string` / primitive pointer → RowsAffected không tin cậy (CRITICAL)
- `Pluck` KHÔNG thay SELECT khi builder đã `.Select(...)` (CRITICAL)
- `Count()` thay Select và bỏ ORDER BY — assert về chúng là lưới rỗng (HIGH)
- `clause.OrderBy{Expression}` bị VỨT khi gộp với `.Order("chuỗi")` (HIGH)
- `json.RawMessage` — `len("null") = 4 > 0`, guard `len > 0` không đủ (HIGH)
- `[]byte` field trả FE bị base64-encode — dùng `json.RawMessage` (HIGH)
- Raw().Scan() không check Error → silent zero
  - Chiều ngược lại: 0 dòng thì `Scan` trả `err = nil` ⇒ nhánh `NotFound` là code chết
- Cột thiếu trong `Select` → zero-value là giá trị HỢP LỆ → hỏng im lặng (CRITICAL)

---


## `ScanRows` với dest `map` — quét đúng 1 dòng, kiểu Go không đổi

Duyệt tập kết quả lớn bằng MỘT con trỏ (thay vì phân trang `LIMIT/OFFSET` nhiều câu):

```go
base := q.Session(&gorm.Session{NewDB: true})   // clone=1 ⇒ getInstance() tạo Statement mới mỗi lần
rows, err := q.Session(&gorm.Session{}).Rows()  // chạy ĐÚNG câu caller dựng, không bọc derived table
defer rows.Close()
for rows.Next() {
    m := map[string]interface{}{}
    if err := base.ScanRows(rows, &m); err != nil { return err }
}
if err := rows.Err(); err != nil { return err }  // BẮT BUỘC: bắt lỗi xảy ra GIỮA CHỪNG khi stream
```

- Khuôn trên hợp lệ vì nhánh `case map[string]interface{}, *map[string]interface{}` trong `scan.go`
  dùng **`if initialized || rows.Next()`** — là `if`, KHÔNG phải `for` ⇒ mỗi lời gọi quét đúng 1 dòng.
- **Kiểu Go giữ nguyên**: nhánh map đơn và nhánh `*[]map[string]interface{}` (thứ `Find` dùng) chạy
  cùng `prepareValues` + `scanIntoMap`. Đổi kiểu (vd `int64` → `[]uint8`) sẽ làm mọi chỗ trả thẳng
  `r["id"]` ghi ra giá trị khác đi mà không có lỗi nào.
- Thiếu `rows.Err()` thì mất kết nối giữa chừng thành "đọc xong bình thường" ⇒ thiếu dữ liệu im lặng.

## GORM v2 — Scan vào `*interface{}` (CRITICAL)

```go
// ❌ NEVER — panic khi query trả row
var id any
db.Table("source").Select("id").Where(...).Limit(1).Scan(&id)

// ✅ ALWAYS — typed struct
var src struct{ ID int64 }
db.Table("source").Select("id").Where(...).Limit(1).Scan(&src)
if src.ID > 0 { ... }

// ✅ Hoặc concrete pointer
var id *int64
db.Table("source").Select("id").Where(...).Limit(1).Scan(&id)
if id != nil { ... }
```

**`Create(&map)` với untyped nil — cũng panic:**
```go
// ❌ NEVER
m := map[string]any{"zalo_info": nil}
db.Table("t").Create(&m)

// ✅ ALWAYS — omit hoặc conditional set
m := map[string]any{...}
if zaloInfoJSON != nil {
    m["zalo_info"] = zaloInfoJSON
}
db.Table("t").Create(&m)
```

Precedent: `client/customer-c/handler/zalo_user.go` — `var sourceID any` + `.Scan(&sourceID)` → PANIC prod.

## GORM v2 — Scan 1 cột JSON/text vào `[]byte` — fail âm thầm, không panic (CRITICAL)

```go
// ❌ NEVER — GORM coi []byte dest là slice-nhiều-dòng (mỗi phần tử 1 uint8), lỗi:
// "converting driver.Value type []uint8 to a uint8: invalid syntax"
var raw []byte
tx.Table("order_service_used").Select("listRevenues").Where("id = ?", id).Scan(&raw)
// lỗi KHÔNG được check (.Error bị bỏ qua) → raw rỗng/sai, code fail âm thầm, return sớm

// ✅ ALWAYS — struct + Take(), check .Error
var row struct {
    ListRevenues []byte `gorm:"column:listRevenues"`
}
if err := tx.Table("order_service_used").Select("listRevenues").
    Where("id = ?", id).Take(&row).Error; err != nil {
    return
}
```

Precedent: `service/payment_commission.go` `handleRevenueTelesale/DoctorConsultant/DirectSale` — dùng `.Scan(&listRaw)` khiến toàn bộ tính năng ghi nhận doanh số không hoạt động, chỉ phát hiện qua test trực tiếp DB.

---

## Go: `new(expression)` — compile error hoặc silent zero (CRITICAL)

`new(T)` nhận **type**, không phải value hay expression. Có 2 trường hợp lỗi:

**Case 1 — compile error**: argument là method call, local variable, struct field, hoặc non-type expression.
```go
// ❌ compile error — pu.Mul(...) là value, không phải type
item.TotalCost = new(pu.Mul(decimal.NewFromFloat(qty)))
// ❌ compile error — warehouseId là biến local, không phải type
svc.Pick(db, id, new(warehouseId), qty)
// ❌ compile error — time.Now().Unix() là method chain trả value
r.UpdatedAt = new(time.Now().Unix())
// ❌ compile error — row.UsageID là struct field accessor, không phải type
StockUsageId: new(row.UsageID)

// ✅ CORRECT — temp var rồi lấy địa chỉ
now := time.Now().Unix()
r.UpdatedAt = &now
usageID := row.UsageID
d.StockUsageId = &usageID
```

**Case 2 — silent bug (compile được, giá trị bị bỏ)**: `new(int(x))` — Go thấy type `int`, bỏ `x`, trả `*int` trỏ vào 0.
```go
// ❌ BUG — UpdatedBy luôn = 0 (x bị bỏ qua)
item.UpdatedBy = new(int(middleware.GetUserID(c)))

// ✅ CORRECT — dùng temp var
updatedBy := int(middleware.GetUserID(c))
item.UpdatedBy = &updatedBy
```

Rule chung: **dùng temp var + `&`**, không dùng `new()` cho pointer to computed value.

Precedent: `hr_user_contract/handler/handler.go` — UpdatedBy = 0 (2026-06-14). `inventory/service/stock_usages.go` — 6 compile errors từ `new(decimal.Mul(...))`, `new(uint(x))` (2026-06-19). `inventory/service/material_recall.go` — 4 compile errors từ `new(time.Now().Unix())`, `new(int(userID))`, `new(row.UsageID)` (2026-06-21).

---

## Transaction error — KHÔNG dùng `_ =` (CRITICAL)

`_ = db.Transaction(...)` compile được nhưng lỗi transaction bị nuốt hoàn toàn — API vẫn trả 200 dù data chưa lưu.

```go
// ❌ NEVER — lỗi bị ignore, dữ liệu mất âm thầm
_ = h.DB.Transaction(func(tx *gorm.DB) error { ... })

// ✅ ALWAYS
if err := h.DB.Transaction(func(tx *gorm.DB) error { ... }); err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{"error": "save_failed"})
    return
}
```

Precedent: `public/quiz/handler/submit.go` — submit quiz trả 200 dù answers không được lưu. Fix 2026-06-15.

---

## Preload — bắt buộc kèm ORDER BY khi có `sort_order` (HIGH)

`Preload("Questions.Options").Preload("Questions")` không có ORDER BY → thứ tự questions/options không ổn định giữa các lần query.

```go
// ❌ NEVER — thứ tự random
db.Preload("Questions.Options").Preload("Questions").Where("id = ?", id).First(&result)

// ✅ ALWAYS khi model có sort_order
db.Preload("Questions", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).
    Preload("Questions.Options", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).
    Where("id = ?", id).First(&result)
```

Áp dụng cho: `psych_quiz_question`, `psych_quiz_option`, bất kỳ model nào có `sort_order`.

---

## Subquery trong WHERE không được thừa hưởng tenant scope từ outer query

```go
// ❌ NGUY HIỂM — outer ScopedDB chỉ scope order_service_used, không scope subquery
q.Where("treatment_plan_id IN (SELECT id FROM order_treatment_plan WHERE appointment_id = ?)", apptID)

// ✅ ĐÚNG — thêm branch scope vào subquery
branchID := middleware.GetBranchActiveID(c)
q.Where(
    "treatment_plan_id IN (SELECT id FROM order_treatment_plan WHERE appointment_id = ? AND (branch_id = ? OR branch_id IS NULL))",
    apptID, branchID,
)
```

**Rule:** Mỗi raw subquery trong WHERE phải tự scope `(branch_id = ? OR branch_id IS NULL)` — KHÔNG trông cậy vào outer `ScopedDBNullBranch`.

---

## `only_full_group_by` — cột non-aggregated trong SELECT với GROUP BY

**Lỗi:** `Expression #N of SELECT list is not in GROUP BY clause and contains nonaggregated column`

**Nguyên nhân:** Dev MySQL thường tắt `only_full_group_by` → query pass local, nhưng prod MySQL bật → fail.

```go
// ❌ Sai — emr_medical_record.diagnosis không trong GROUP BY
Select(`icd10_code AS code, COALESCE(ei.name, emr_medical_record.diagnosis, '') AS name, COUNT(*) AS cnt`)

// ✅ Đúng — bọc cột non-aggregated bằng MIN() hoặc ANY_VALUE()
Select(`icd10_code AS code, COALESCE(ei.name, MIN(emr_medical_record.diagnosis), '') AS name, COUNT(*) AS cnt`)
```

**Rule:** Mọi cột trong SELECT của query có GROUP BY mà không nằm trong GROUP BY → PHẢI bọc `MIN()` / `MAX()` / `ANY_VALUE()`. Đặc biệt coi chừng cột trong `COALESCE`, `CONCAT`, `IF`.

**Alias-based GROUP BY không tương đương group theo chính biểu thức** khi biểu thức bọc cột
nullable: `Group("doctor_id, ...")` (alias của `COALESCE(doctorAppointment_id, 0)`) → lỗi 1055,
dù `DATE_FORMAT`/`FLOOR` alias vẫn chạy được (verified MySQL 8.0.36). Fix: `Group()` bằng
chính biểu thức gốc `COALESCE(appointment.doctorAppointment_id, 0)`, không dùng alias.

---

## Subquery / Session reuse gotchas — tách file

`.Order("")` không xoá ORDER BY, `Session({})` giữ nguyên Group/Having, `Paginate` mutate `q`
in-place khiến `.Session()` gọi sau không hồi tố → xem `patterns-gotchas-gorm-subquery.md`.

---

## Thêm field mới vào cached struct → zero value = production outage khi deploy

Khi thêm field (`Status int`, `IsActive bool`...) vào struct đang cache Redis, entries cũ không có key đó → deserialize thành zero value.

```go
// ❌ NEVER — Status=0 từ cache cũ sẽ block tất cả user nếu check Status != UserStatusActive
if err := ch.Get(cacheKey, &s); err == nil {
    return s
}

// ✅ ALWAYS — guard zero value để force re-read từ DB
if err := ch.Get(cacheKey, &s); err == nil && s.Status != 0 {
    return s
}
```

Quy tắc chung: sau khi thêm mandatory field vào cached struct, phải có guard kiểm tra field đó khác zero — hoặc flush toàn bộ cache key liên quan trước khi deploy.

---

## `datatypes.JSON` — KHÔNG gọi `.UnmarshalJSON(field)` (HIGH)

`datatypes.JSON` là `[]byte`. Gọi `field.UnmarshalJSON(field)` là noop — target struct không được populate, zero value âm thầm.

```go
// ❌ NEVER — RefreshMinutes luôn = 0
var qrCfg struct{ RefreshMinutes int `json:"refresh_minutes"` }
cfg.QrConfig.UnmarshalJSON(cfg.QrConfig)  // noop

// ✅ ALWAYS — dùng json.Unmarshal trực tiếp
import "encoding/json"
var qrCfg struct{ RefreshMinutes int `json:"refresh_minutes"` }
json.Unmarshal(cfg.QrConfig, &qrCfg)
```

Áp dụng cho mọi `datatypes.JSON` field: `GpsConfig`, `WifiConfig`, `QrConfig`, `Methods`, v.v.

---

## LEFT JOIN — driving table phải là bảng luôn tồn tại

```go
// ❌ NEVER — user_profile là optional; user chưa có profile → zero rows → zero values silently
db.Table("user_profile").
    Joins("LEFT JOIN user ON user.id = user_profile.user_id").
    Where("user_profile.user_id = ?", userID).Scan(&row)

// ✅ ALWAYS — user luôn tồn tại; user_profile là optional → RIGHT side
db.Table("`user` u").
    Joins("LEFT JOIN user_profile p ON p.user_id = u.id").
    Where("u.id = ?", userID).Scan(&row)
```

Rule: driving table (LEFT) phải là bảng mandatory. Bảng optional → đặt RIGHT side của JOIN.

---

## GORM v2 — Scan vào `*string` / primitive pointer → RowsAffected không tin cậy (CRITICAL)

`RowsAffected` có thể = 0 ngay cả khi tìm thấy row khi scan vào primitive pointer.

```go
// ❌ Không tin cậy
var code *string
tx := db.Table("t").Select("code").Where(...).Limit(1).Scan(&code)
if tx.RowsAffected == 0 { ... } // false-positive: row tồn tại nhưng RowsAffected = 0

// ✅ Dùng Pluck vào slice
var codes []string
db.Table("t").Where(...).Limit(1).Pluck("code", &codes)
if len(codes) == 0 { ... }
```

`Pluck` vào con trỏ scalar (`*uint`) trên cột NULLable: KHÔNG trả nil mà lỗi `sql: Scan called without calling Next`; `%v` in ra ĐỊA CHỈ chứ không phải giá trị. Đọc cột nullable → `Scan` vào struct có field con trỏ.

## `Pluck` KHÔNG thay SELECT khi builder đã `.Select(...)` (CRITICAL)

`Pluck` chỉ tự đặt SELECT khi `len(Statement.Selects) != 1`. `.Select("a, b, c")` (1 chuỗi nhiều cột) = **1 phần tử** → Pluck giữ nguyên toàn bộ cột, scan vào `[]uint` báo `Scan error on column index 0: destination not a pointer` và trả về slice toàn `0`.

Kèm `.Select("<1 cột>")` ngay trước `Pluck`, và LUÔN check `.Error` — nuốt lỗi ở đây biến query hỏng thành "không tìm thấy record".

```go
tx := buildQuery(c).Where("id IN ?", ids).Select("u.id").Pluck("u.id", &out)
if pagination.IsQueryError(tx.Error) { pagination.FailQuery(c, tx.Error); return }
```

## `Count()` thay Select và bỏ ORDER BY — assert về chúng là lưới rỗng (HIGH)

Ngược với `Pluck` ở trên: `.Count()` LUÔN thay danh sách cột bằng `count(*)` và bỏ `ORDER BY`, dù builder đã `.Select(...)` / `.Order(...)`.

⇒ Test kiểu "câu đếm không mang ORDER BY" hoặc "không mang danh sách cột của câu SELECT" **luôn xanh**, kể cả khi cho hàm dựng câu đếm trả thẳng câu SELECT. Hai tính chất đó do GORM bảo đảm, không phải do handler — đừng dùng làm test.

Thứ test khoá được ở câu đếm: có `GROUP BY` không · join theo điều kiện nào · scope tenant còn không.

## `clause.OrderBy{Expression}` bị VỨT khi gộp với `.Order("chuỗi")` (HIGH)

- `q.Order(clause.OrderBy{Expression: clause.Expr{SQL: "…", Vars: …}})` rồi `.Order("col DESC")` (vd `query.ApplySort`) ⇒ GORM chỉ nối `Columns`, **vứt Expression**, không lỗi, build xanh.
- Cần ORDER BY có tham số mà sau đó còn `ApplySort`/`.Order(chuỗi)` ⇒ dùng dạng CHUỖI. Chỉ nhúng literal khi giá trị do server tự format (vd `time.DateOnly`), không bao giờ từ input người dùng.
- Khoá bằng test DryRun assert chuỗi `ORDER BY (…) DESC` có mặt trong SQL.

## `json.RawMessage` — `len("null") = 4 > 0`, guard `len > 0` không đủ (HIGH)

Khi FE gửi `field: null`, Go binding tạo `json.RawMessage("null")` — `len = 4 > 0`.
Guard `if len(in.SomeJson) > 0` sẽ ghi JSON literal `null` vào DB, **overwrite data cũ**.

```go
// ❌ Không đủ — sẽ ghi "null" vào DB khi FE gửi null
if len(in.ClinicalExamJson) > 0 {
    rec.ClinicalExamJson = datatypes.JSON(in.ClinicalExamJson)
}

// ✅ Thêm kiểm tra string "null"
if len(in.ClinicalExamJson) > 0 && string(in.ClinicalExamJson) != "null" {
    rec.ClinicalExamJson = datatypes.JSON(in.ClinicalExamJson)
}
```

Áp dụng cho mọi field nhận `json.RawMessage` — `VitalSigns`, `ClinicalExamJson`, v.v.

---

## `[]byte` field trả FE bị base64-encode — dùng `json.RawMessage` (HIGH)

GORM `[]byte` + `gorm:"type:json"` scan/ghi JSON column OK, NHƯNG `encoding/json.Marshal` (serialize struct ra FE) base64-encode `[]byte` thành STRING thay vì xuất JSON array/object → FE nhận chuỗi base64, vỡ contract. Cột JSON đọc-ra-FE khai `json.RawMessage` (RawMessage = []byte alias nên GORM scan/ghi vẫn OK; Marshal xuất thẳng JSON). Integration test DB round-trip KHÔNG bắt (GORM xử lý []byte↔JSON bình thường) — chỉ test HTTP-response shape mới lộ.

---

## Raw().Scan() không check Error → silent zero

```go
// ❌ DB fail (timeout, lock) → Scan không populate → biến giữ zero-value → response trả 0
// UI hiển thị "Giá trị tồn: 0₫" — trông như data thật, không có alert nào bắt được
h.DB.Raw(`SELECT COALESCE(SUM(total_cost), 0) AS cogs FROM stock_usages ...`).Scan(&cogs)

// ✅ Luôn check res.Error — đặc biệt với dashboard/aggregate query
res := h.DB.Raw(`SELECT COALESCE(SUM(total_cost), 0) AS cogs FROM stock_usages ...`).Scan(&cogs)
if res.Error != nil {
    response.ServerError(c, res.Error)
    return
}
```

Áp dụng: mọi raw SQL aggregate (SUM, COUNT, AVG) trong dashboard/analytics handler.

### Chiều NGƯỢC LẠI cùng một hàm: 0 dòng thì `Scan` trả `err = nil` ⇒ nhánh `NotFound` là CODE CHẾT

Mục trên nói về chiều "DB fail mà không ai check". Chiều còn lại tốn kém không kém: `Scan` (và
`Find`) coi **0 dòng là kết quả hợp lệ**, KHÔNG trả `gorm.ErrRecordNotFound` — chỉ `First`/`Take`/
`Last` mới trả. Nên khuôn dưới đây **không bao giờ** vào được nhánh 404:

```go
// ❌ id không tồn tại (hoặc thuộc tenant khác) → err = nil, item = nil → rơi xuống trả 200 + data null
var item map[string]interface{}
if err := tenant.ScopedDBNullBranch(c, h.DB, "note").Table("note").
    Where("id = ?", id).Limit(1).Scan(&item).Error; err != nil {
    response.NotFound(c); return
}
response.Data(c, item)

// ✅ check err (lỗi DB thật) RỒI check rỗng (không có record) — hai chuyện khác nhau
if err := ...Scan(&item).Error; err != nil { response.ServerError(c, err); return }
if len(item) == 0 { response.NotFound(c); return }
```

Vì sao khó thấy: build xanh, test xanh, request trả **200** nên không ai coi là lỗi; nhánh `if err
!= nil` nằm ngay đó làm người đọc tin rằng ca "không tìm thấy" đã được xử lý.

**Tồn kho đã quét 2026-09-07** — 33 điểm dùng khuôn `Scan(&x).Error → response.*`, phân loại:
- **29 điểm scan vào SLICE** (list/report/dashboard) — **ĐÚNG, đừng sửa**: rỗng là kết quả hợp lệ,
  ở đây chỉ cần check `err`.
- **4 điểm đọc 1 record**: `crm/note/handler` `GetNote` (đã vá); `config/source`,
  `config/group_source`, `config/appointment_type` — 3 handler toggle status, id không tồn tại thì
  biến `current` giữ `0` ⇒ `newVal = 1` ⇒ trả **200 "updated"** cho record không có thật (UPDATE
  trúng 0 dòng nên KHÔNG hỏng dữ liệu — chưa vá, mức độ thấp).
- Mẫu ĐÚNG để đối chiếu: `crm/payment/handler/helpers.go` `verifyPaymentCompanyMatches` — 0 dòng →
  con trỏ `nil` → rơi vào guard `== nil` sẵn có ⇒ **fail-closed** đúng ý đồ.

⚠ Đừng suy ra "cứ `Scan` là sai": phân biệt theo **đích scan**, không theo tên hàm. Scan vào slice
= list, rỗng hợp lệ. Scan vào 1 map/struct/primitive = đọc 1 record, PHẢI có nhánh rỗng riêng.

---

## Cột thiếu trong `Select` → zero-value là giá trị HỢP LỆ → hỏng im lặng (CRITICAL)

Cột nào **code rẽ nhánh theo nó** mà thiếu trong `Select` ⇒ struct nhận zero-value; zero-value thường
là một giá trị hợp lệ (`0` = loại mặc định, `false`, `""`) nên KHÔNG log, KHÔNG lỗi.

Hệ quả kép: nhánh còn lại không bao giờ chạy, và mọi guard chỉ sống trong nhánh đó thành **code chết**
— test của guard vẫn xanh vì chúng gọi thẳng hàm, không đi qua đường nạp dữ liệu.

Trước khi thêm guard rẽ theo một cột enum/flag: `grep` MỌI câu `Select` nạp struct đó, không chỉ chỗ
khai field.

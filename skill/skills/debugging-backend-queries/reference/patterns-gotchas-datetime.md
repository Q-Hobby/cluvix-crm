# Gotchas Datetime

## Mục lục

- Timezone — Day Boundary (CRITICAL)
- Cột SQL `DATE`/`DATETIME` (không phải bigint unix) — format hiển thị ở BE, KHÔNG đẩy ra FE parse
- Đổi format string ngày hiển thị (dash↔slash...) — PHẢI trace consumer FE thật, không suy đoán (CRITICAL)
- Cột `DATE` + `parseTime=True` → Go string field nhận RFC3339 khi load lại

---


Tách từ `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md`. Hợp đồng handler → `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md`.

## Timezone — Day Boundary (CRITICAL)

```go
// ❌ NEVER — UTC midnight, VN hiển thị 07:00 sáng
baseDay := (timestamp / 86400) * 86400
now := time.Now().Truncate(24 * time.Hour).Unix()
cacheKey := fmt.Sprintf(".._%d", time.Now().Unix()/86400)

// ✅ ALWAYS
loc := tz.LocFromContext(c, db)          // từ handler
// hoặc:
loc := tz.Location(tx, uint(companyID))  // từ service

baseDay := tz.StartOfDay(loc)

// Đầu ngày của 1 timestamp cụ thể:
t := time.Unix(createdTime, 0).In(loc)
baseDay = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).Unix()
```

**Raw SQL groupBy ngày:**
```go
offsetSec := tz.OffsetSec(loc)
// FLOOR((col + offsetSec) / 86400) * 86400 - offsetSec
```

**Duration arithmetic (elapsed days) — OK dùng `/86400`:**
```go
daysAgo := (now - lastVisitDate) / 86400  // ✅ OK — đây là duration, không phải day-boundary
```

**Tóm tắt:**
- Tạo timestamp "đầu ngày" → `tz.StartOfDay` hoặc `time.Date(..., 0,0,0, loc)`
- Tính số ngày elapsed → `/86400` OK
- Cache key theo "hôm nay" → `tz.StartOfDay(loc)`

Precedent: `customer_care.day_need_care` lệch 7h → fix 2026-05-29.

---

## Cột SQL `DATE`/`DATETIME` (không phải bigint unix) — format hiển thị ở BE, KHÔNG đẩy ra FE parse

Bảng ERP đa số dùng `BIGINT` unix cho timestamp (đúng Hard Rule), nhưng vẫn còn vài cột legacy kiểu
SQL `date` thật (`customer.birthday`, `paid_date`, `registration_date`...). Khi `SELECT` cột này raw
(không `DATE_FORMAT`) vào `map[string]interface{}` rồi `c.JSON()`:

```go
// ❌ NEVER — Select raw cột `date`, để Go marshal time.Time mặc định
Select("id, birthday, ...")
// → parseTime=True trong DSN khiến driver trả time.Time
// → c.JSON marshal thành chuỗi ISO có offset: "2024-07-04T00:00:00+07:00"
// → FE base fields (addFormattedTimestampFields/formatTimestampFields) CHỈ nhận number/Date,
//   gặp string thì return '' — cột hiển thị RỖNG hoàn toàn, không lỗi/không log gì (bug thật, birthday
//   list customer-care/birthday im lặng trống cột "Ngày sinh" cho tới khi phát hiện — 2026-07-27).

// ✅ ALWAYS — format sẵn ở BE bằng DATE_FORMAT, FE chỉ view thẳng field, không parse/convert
Select("id, DATE_FORMAT(birthday, '%d/%m/%Y') as birthday, ...")
```

**Lý do ưu tiên format ở BE thay vì để FE tự parse:**
- BE là nơi biết chính xác kiểu cột gốc (`date` vs bigint) — FE chỉ nhận JSON, không phân biệt được
  "string này là ISO datetime cần convert" hay "string này đã là display string" nếu không có convention rõ.
- Tránh double-handling timezone: ngày-only (birthday, ngày hết hạn văn bản...) **không có ý nghĩa giờ/tz**,
  nếu FE lại đưa qua `new Date()` + format theo timezone trình duyệt/company sẽ có nguy cơ lệch 1 ngày
  (cùng lớp bug với `toISOString()` ở `patterns-gotchas-http.md`/FE) — format string sẵn ở BE loại bỏ
  hoàn toàn nguy cơ này vì không còn bước convert timezone nào ở FE nữa.
- **FE on-screen đã thống nhất `dd/mm/yyyy` từ 2026-07-27** (`BaseApiService.formatTimestampFields`,
  `CustomDatePipe`, + toàn bộ callsite dash lẻ đã sửa) — khớp Intl `vi-VN` (`toLocaleDateString`) và
  PrimeNG datepicker input (đều đã sẵn slash từ trước).
- **CHƯA phải convention toàn app** — BE export vẫn CÒN MIXED: `pkg/export.fmtUnixDate` (dash
  `"02-01-2006"`, dùng cho phần lớn ~85 export function field ngày thường) song song với
  `fmtUnixDateSlash`/`installment.go` (slash, chỉ 1 nhóm nhỏ cột SQL `date` thật — xem
  `skill/skills/working-on-ops/reference/export.md`). Nhiều chỗ BE khác sinh text hiển thị trực tiếp (chart label
  `appointment.go`, timeline HTML `customer_detail/service/timeline_builders.go`, response client ZMA
  `client/handler/helpers.go`, `inventory/stock_entry`...) cũng còn dash, CHƯA được audit/đổi. Trước khi
  đổi thêm bất kỳ chỗ nào trong nhóm này — xác nhận phạm vi với user, đừng giả định "đã xong toàn app".

**KHÔNG áp dụng** cho cột cần giá trị raw để tính toán/filter tiếp (vd tuổi qua `TIMESTAMPDIFF`, so sánh
range) — chỉ format cột dùng riêng cho **hiển thị**, giữ nguyên cột gốc (hoặc thêm alias riêng) nếu logic
khác còn cần giá trị thật.

---

## Đổi format string ngày hiển thị (dash↔slash...) — PHẢI trace consumer FE thật, không suy đoán (CRITICAL)

Khi đổi `Format("02-01-2006")` → `Format("02/01/2006")` (hay bất kỳ đổi separator/thứ tự khác) cho 1
field trả về FE — **"field này chỉ để hiển thị" là suy đoán, không phải sự thật đã verify.** FE hoàn toàn
có thể tự parse lại string đó cho mục đích KHÁC hiển thị (dùng làm key lookup, sort theo ngày...) mà tên
field/comment BE không hề gợi ý.

**2 regression thật đã xảy ra (2026-07-28), cùng 1 buổi migrate dash→slash, do tin "chỉ hiển thị":**

```go
// ❌ Case 1 — chart lịch hẹn (formatChartXY, appointment.go): đổi "x" sang slash tưởng an toàn vì
// nhìn giống 1 label hiển thị trên chart. Nhưng FE (appointment-chart-scatter.ts) tự sinh
// xAxisLabels bằng dash rồi map.get(date) so khớp STRING Y HỆT với field "x" — đổi slash → miss
// 100% → chart hiện toàn 0, không throw/log gì.

// ❌ Case 2 — gallery ảnh điều trị (dateKey, treatment_course.go): đổi dateKey sang slash. FE
// (list-image.ts) có regex .replace(/(\d{2})-(\d{2})-(\d{4})/, '$2/$1/$3') CHỈ khớp dash, swap
// ngày/tháng cho new Date() sort đúng. Đổi slash → regex miss → new Date("02/01/2026") bị JS hiểu
// nhầm US-format (MM/DD) → sort sai thứ tự âm thầm.
```

**Quy trình bắt buộc trước khi đổi format 1 field ngày đã tồn tại (không áp dụng cho field MỚI thêm):**
1. `grep` FE cho **tên field JSON** (không chỉ tên hàm BE) — tìm mọi nơi field đó được đọc.
2. Với mỗi nơi đọc được: xác định nó **hiển thị thẳng** (`{{ }}`, `[innerHTML]`) hay **tái xử lý**
   (`new Date(...)`, `.replace(/regex/)`, dùng làm `Map`/object key rồi so khớp với giá trị tự sinh
   ở nơi khác, `.split()`...). Chỉ nhóm đầu là an toàn đổi ngay.
3. Field dùng làm key nội bộ (map/group-by) mà JSON key đó cũng được FE dùng để lookup — coi như
   "tái xử lý", không phải "hiển thị", dù bản chất vẫn là 1 ngày tháng.
4. Không tìm được consumer nào (endpoint có vẻ không ai gọi) → vẫn đổi được (an toàn theo nghĩa
   không phá gì đang chạy) nhưng flag rõ trong PR/note "không tìm thấy caller FE, có thể unused".

**KHÔNG tự tin dựa vào:** tên field (`"x"`, `label`, `dateKey` đều "nghe" như chỉ để hiển thị), comment
BE (viết lúc tạo field, không cập nhật khi FE sau này thêm logic đọc lại), hay việc field đó "trông giống"
field khác đã verify an toàn (`fmtUnixDate`/`fmtUnixDateTime` trong `pkg/export` an toàn vì kết thúc ở
Excel — KHÔNG suy diễn field khác cùng tên hàm helper cũng an toàn nếu nó KHÔNG nằm trong luồng export).

---

## Cột `DATE` + `parseTime=True` → Go string field nhận RFC3339 khi load lại

- Entity có field `gorm:"type:date"` kiểu `string`/`*string`: DSN `parseTime=True` khiến SELECT-lại trả `"2006-01-02T00:00:00Z"` (không phải `"2006-01-02"` như lúc INSERT).
- Code `time.Parse("2006-01-02", item.DateField)` trên record ĐÃ LOAD (flow update/submit/approve từ record cũ) → fail, nổ ở mọi write-path đọc lại date.
- Fix: hook `AfterFind` normalize date fields về 10 ký tự (`s[:10]`), hoặc parse cả 2 format. Áp cho MỌI entity có date-string reload-rồi-parse. Đã cắn: `HrLeaveRequest.AfterFind` (from_date/to_date).
- **`AfterFind` KHÔNG chạy cho raw-map scan**: list handler `Find(&[]map[string]interface{})` bypass hook → cột DATE leak RFC3339 thẳng ra JSON (vỡ render/so sánh string/datepicker FE). Fix ở SELECT: `DATE_FORMAT(x.col,'%Y-%m-%d') AS col`. Đã cắn: `hr_ot_request` list; `hr_leave` list (from_date/to_date) ĐANG dính — nợ chưa fix.

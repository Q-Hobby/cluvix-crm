# Timezone Service

## Mục lục

- Vấn đề cốt lõi
- API của TimezoneService
  - Methods trả về unix timestamp (giây) — dùng cho API
  - Methods trả về Date — dùng cho datepicker default value
- Rule bắt buộc
  - 1. Không dùng `getTime()` để gửi API
  - 2. Không dùng `setHours()` để tính timestamp
  - 3. Date range từ datepicker → dùng `dateHmsToUnix`
  - 4. Date từ timepicker (có giờ cụ thể) → truyền giờ từ Date
  - 5. `baseApiService.unixTimesTampFields` — đã fix, không cần làm thêm
- `toTz()` Date — LƯU Ý QUAN TRỌNG
- `toUnix()` — giới hạn (bug đã fix ở `buildFilterTimestamp`, 2026-08-04)
- Performance
- Hiển thị field date-only nhận từ BE — KHÔNG tự parse/convert ở FE
- Bảng cheat sheet

---


> **Đọc file này trước khi viết bất kỳ code nào liên quan đến timestamp, date range, hay date filter trong Angular FE.**

---

## Vấn đề cốt lõi

Cluvix là multi-tenant ERP. Mỗi company có thể dùng timezone khác nhau (lưu trong `company.timezone`).
`new Date().getTime() / 1000` và `setHours()` luôn dùng **browser local timezone** — sẽ sai khi browser TZ ≠ company TZ.

**Toàn bộ unix timestamp gửi lên API phải đi qua `TimezoneService`.**

---

## API của TimezoneService

File: `src/app/core/services/timezone.service.ts`  
Inject sẵn trong `BaseApp` và `BaseApiService` → mọi component/service kế thừa đều có `this.timezoneService`.  
Nếu class không extend hai base trên → `inject(TimezoneService)` thủ công.

### Methods trả về unix timestamp (giây) — dùng cho API

| Method | Dùng khi |
|---|---|
| `todayStartUnix()` | Đầu ngày hôm nay (00:00:00) theo company TZ |
| `todayEndUnix()` | Cuối ngày hôm nay (23:59:59) theo company TZ |
| `monthStartUnix()` | Đầu tháng này theo company TZ |
| `monthEndUnix()` | Cuối tháng này theo company TZ |
| `dateHmsToUnix(date, h, m, s)` | Ngày lấy từ `date`, giờ/phút/giây chỉ định — dùng cho datepicker value |
| `toUnix(date, 'start'\|'end')` | Convert string/number sang unix — **KHÔNG dùng trên `toTz()`-output Date** |

### Methods trả về Date — dùng cho datepicker default value

| Method | Dùng khi |
|---|---|
| `now()` | Thời điểm hiện tại theo company TZ |
| `todayStart()` | Đầu ngày hôm nay — Date |
| `monthStart()` | Đầu tháng — Date |
| `monthEnd()` | Cuối tháng — Date |

---

## Rule bắt buộc

### 1. Không dùng `getTime()` để gửi API

```typescript
// ❌ NEVER — browser timezone
const ts = Math.floor(date.getTime() / 1000);

// ❌ NEVER — toTz() output → getTime() sai khi browser TZ ≠ company TZ
const ts = this.timezoneService.todayStart().getTime() / 1000;

// ✅ ALWAYS — company timezone
const ts = this.timezoneService.todayStartUnix();
```

### 2. Không dùng `setHours()` để tính timestamp

```typescript
// ❌ NEVER
const d = new Date();
d.setHours(0, 0, 0, 0);
const ts = Math.floor(d.getTime() / 1000);

// ✅ ALWAYS
const ts = this.timezoneService.todayStartUnix();
```

### 3. Date range từ datepicker → dùng `dateHmsToUnix`

`dateHmsToUnix(date, h, m, s)` là method chuẩn cho datepicker value:
- Nhận Date từ picker (browser-local hoặc `toTz()`-output đều được)
- Dùng `getFullYear/Month/Date` (calendar date user chọn) + h/m/s → unix đúng company TZ

```typescript
// ❌ NEVER
const from = Math.floor(dr[0].getTime() / 1000);
const to   = Math.floor(dr[1].getTime() / 1000) + 86399;

// ✅ ALWAYS
const from = this.timezoneService.dateHmsToUnix(dr[0], 0, 0, 0);
const to   = dr[1]
  ? this.timezoneService.dateHmsToUnix(dr[1], 23, 59, 59)
  : this.timezoneService.todayEndUnix();
```

### 4. Date từ timepicker (có giờ cụ thể) → truyền giờ từ Date

```typescript
// ❌ NEVER — mất giờ phút giây
const ts = Math.floor(appointmentDate.getTime() / 1000);

// ✅ ALWAYS
const d = appointmentDate;
const ts = this.timezoneService.dateHmsToUnix(d, d.getHours(), d.getMinutes(), d.getSeconds());
```

### 5. `baseApiService.unixTimesTampFields` — đã fix, không cần làm thêm

`beforeSave()` trong `BaseApiService` đã dùng `dateHmsToUnix` cho tất cả field trong `unixTimesTampFields`. Chỉ cần khai báo field name đúng là đủ.

**CRITICAL — KHÔNG convert thủ công trong component nếu field đã có trong `unixTimesTampFields`:**

```typescript
// ❌ NEVER — double conversion: component convert → BaseApiService convert lại
// result: new Date(unixSeconds) hiểu nhầm là ms → Jan 1970 → ra số sai (~1/1000)
if (formValue.created_time instanceof Date) {
  formValue.created_time = this.timezoneService.dateHmsToUnix(d, ...); // thừa!
}

// ✅ ALWAYS — khai báo trong service, để BaseApiService xử lý
public override readonly unixTimesTampFields = ['created_time', 'treatment_timeEnd'];
// không cần làm thêm gì trong component
```

Dấu hiệu bug: giá trị timestamp trong DB nhỏ ~1000x so với kỳ vọng (vd: `1781515` thay vì `1781515000`).

**Chiều ĐỌC — field trong `unixTimesTampFields` về component là `Date`, KHÔNG phải số giây:** `afterLoad()` đã quy đổi. Model khai `number` nên `tsc` KHÔNG bắt được, và `Date <= unixSeconds` coerce `Date` sang **ms** ⇒ điều kiện luôn sai, im lặng. Cần so sánh mốc thời gian → chuẩn hoá qua `utilsService.toUnixSeconds()`, hoặc hỏi BE thay vì tự tính lại.

Precedent: badge "Đang áp dụng" của `tax_rate` chọn sai mức (FE hiện 10% trong khi BE xuất hoá đơn 8%). Fix 2026-08-26.

**Vế HIỂN THỊ của cùng lỗi đó:** `{{ x * 1000 | date }}` trên field đã convert ⇒ `Date * 1000` ra năm ~58000; format không có năm (`'HH:mm dd/MM'`) khiến ngày sai vẫn trông hợp lý. Bỏ `* 1000`, hoặc dùng khuôn nhận cả hai dạng: `v instanceof Date ? new Date(v) : new Date(Number(v) * 1000)`.

**Trước khi sửa, xác định field có THẬT SỰ đã convert không** — cùng một service vẫn có 2 loại hàm:
- Hàm kế thừa `BaseApiService.get/list` hoặc có gọi `afterLoad` → **đã là `Date`**.
- Hàm gọi `this.apiService.get(...)` thẳng (vd `appointmentScheduleByHourGrouped`, `listByRecord`, endpoint report/print) → **vẫn là giây**, giữ `* 1000`.
- Biến lặp trên mảng lồng (`@for (item of model.coupon)`) và object lồng (`model.customer?.appointment?.x`) → không được convert.

---

## `toTz()` Date — LƯU Ý QUAN TRỌNG

`timezoneService.now()`, `todayStart()`, `monthStart()` v.v. trả về Date mà:
- `getFullYear/Month/Date/Hours/Minutes/Seconds` = giá trị wall-clock theo **company TZ** ✓
- `getTime()` = **KHÔNG dùng** để tính unix (browser-relative, sai khi browser TZ ≠ company TZ) ✗

```typescript
const d = this.timezoneService.todayStart();
// ✅ OK — wall-clock parts đúng
d.getDate()   // ngày theo company TZ
d.getHours()  // giờ theo company TZ (= 0 vì start)
// ❌ KHÔNG làm
d.getTime() / 1000  // sai!

// ✅ Thay bằng
this.timezoneService.todayStartUnix()
```

---

## `toUnix()` — giới hạn (bug đã fix ở `buildFilterTimestamp`, 2026-08-04)

`toUnix(date, mode)` gọi `toTz(new Date(date))` bên trong. Nếu `date` là output của `toTz()`
(hoặc Date từ `p-datePicker` — y/m/d là ý định lịch của người dùng, không phải "moment"), double-
conversion lệch ngày khi browser TZ khác company TZ (đo được: lệch đúng 24h khi 2 offset cách xa
— vd company `America/New_York`, browser `Asia/Saigon`). **Không dùng `toUnix` cho Date thuộc loại
này.** `toUnix` chỉ đúng khi `date` là **raw moment thật** chưa gán ý định lịch nào — `new Date()`,
hoặc dựng thẳng từ unix timestamp server/URL (`new Date(unixSeconds * 1000)`) mà mục đích là suy
ra "ngày công ty tương ứng với thời điểm đó" (vd `form-beauty.ts`, `by-date.ts` — dựng Date từ
`model.created_time`/URL param rồi `toUnix` để tính lại start/end của đúng ngày đó).

**`buildFilterTimestamp()` (filter khoảng-ngày cho `p-table`, dùng ở 53+ call site và mọi
`kind:'dateRange'` của `BaseListV2`) đã từng VI PHẠM chính rule này** — gọi `toUnix()` trên Date
luôn luôn thuộc loại "đã có ý định lịch" (`todayStart()`/`monthStart()`/`fromUnix()`/`p-datePicker`
value), KHÔNG BAO GIỜ là raw moment. Đã sửa: giờ dùng `dateHmsToUnix()` nội bộ (trích y/m/d trực
tiếp, không tái diễn giải qua `toTz`) — cùng công thức `todayStartUnix()`.

→ Rule chung: **bất kỳ hàm nào build unix timestamp từ 1 Date "đã có ý định lịch" (kể cả do chính
`TimezoneService` sinh ra) phải trích `getFullYear/Month/Date` trực tiếp** (như `dateHmsToUnix`),
KHÔNG được đi qua `toUnix`/`toTz` lần nữa. Chỉ input KHÔNG có ý định lịch trước đó (`new Date()`,
raw unix từ server) mới cần `toTz`.

---

## Performance

`wallClockToUnix()` (được gọi bởi `dateHmsToUnix`, `todayStartUnix`, v.v.) tạo `Intl.DateTimeFormat` mới mỗi lần. **Không gọi trong vòng lặp per-item.**

```typescript
// ❌ SLOW — tạo formatter N lần
items.forEach(item => {
  item.ts = this.timezoneService.dateHmsToUnix(date, 0, 0, 0);
});

// ✅ FAST — tính 1 lần, dùng lại
const dayTs = this.timezoneService.dateHmsToUnix(date, 0, 0, 0);
items.forEach(item => {
  item.ts = dayTs;
});
```

---

## Hiển thị field date-only nhận từ BE — KHÔNG tự parse/convert ở FE

Khác chiều gửi API (trên) — chiều NHẬN: field cột SQL `date`/`datetime` thật (không phải bigint unix,
vd `customer.birthday`) khi BE trả raw sẽ bị Go marshal thành chuỗi ISO có offset
(`"2024-07-04T00:00:00+07:00"`), KHÔNG phải number/Date nên `BaseApiService.formatTimestampFields`
(chỉ nhận `number`/`Date`) trả `''` — cột hiển thị RỖNG im lặng, không lỗi (bug thật đã gặp ở
`customer-care/birthday`, 2026-07-27).

**Fix đúng — BE format sẵn, FE chỉ view:**
```typescript
// ❌ NEVER — tự parse chuỗi ISO rồi qua new Date()/TimezoneService để hiển thị lại
// (ngày sinh không có ý nghĩa giờ/tz, convert qua Date + format theo tz trình duyệt/company
// có nguy cơ lệch 1 ngày — cùng lớp lỗi với dùng getTime()/toISOString() ở trên)
const d = new Date(item.birthday);
birthdayFormatted: this.timezoneService.format(d, {...}),

// ✅ ALWAYS — BE đã DATE_FORMAT sẵn thành chuỗi dd/mm/yyyy (convention DUY NHẤT toàn app từ
// 2026-07-27, khớp CustomDatePipe/formatTimestampFields FE), FE chỉ gán thẳng
birthdayFormatted: item?.birthday || '',
```

BE side: xem `../../../skill/skills/debugging-backend-queries/reference/patterns-gotchas.md` §"Cột SQL DATE/DATETIME... format hiển
thị ở BE". Field nào BE chưa format sẵn (còn trả raw date/timestamp lạ) → báo BE sửa tại nguồn, không
tự vá bằng logic parse ở FE.

---

## Bảng cheat sheet

| Tình huống | Code đúng |
|---|---|
| Hôm nay đầu ngày → API | `timezoneService.todayStartUnix()` |
| Hôm nay cuối ngày → API | `timezoneService.todayEndUnix()` |
| Đầu tháng → API | `timezoneService.monthStartUnix()` |
| Cuối tháng → API | `timezoneService.monthEndUnix()` |
| Datepicker value → start-of-day unix | `timezoneService.dateHmsToUnix(date, 0, 0, 0)` |
| Datepicker value → end-of-day unix | `timezoneService.dateHmsToUnix(date, 23, 59, 59)` |
| Datepicker value → end-of-day **milliseconds** (BE lưu ms) | `date.getTime() + 86_399_999` |
| Timepicker value → exact unix | `timezoneService.dateHmsToUnix(d, d.getHours(), d.getMinutes(), d.getSeconds())` |
| Default value cho datepicker | `timezoneService.now()` / `timezoneService.monthStart()` |
| `*Unix()` method → datepicker | `timezoneService.todayStart()` (Date, không phải unix) |

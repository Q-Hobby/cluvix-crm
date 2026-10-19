# Print flow

## Mục lục

- 0. HAI NHÁNH IN — xác định đúng nhánh TRƯỚC khi sửa
- 1. Kiến trúc tổng quan (NHÁNH A)
- 2. `openPrint` vs `printCommon` — khi nào dùng cái nào
- 3. `PrintLayout` — cơ chế trigger `window.print()`
  - Trigger logic
  - `triggerPrint()` làm gì
  - **CRITICAL: `company` phải truthy**
- 4. `PrintContext` — các field và `layoutType`
  - `layoutType` values
- 5. Pattern chuẩn cho Print Component
- 6. Caller — Pattern mở print từ BaseList/BaseReport
- 7. `apiService.get` — params phải dùng `{ params: { ... } }`
- 8. `PrintShareImport` — import minimal cho print pages
- 9. Gotchas — Lỗi thường gặp khi viết print
  - Trang trắng thừa khi in
  - BE: array response có thể nil → FE crash
- 10. Files và route map
- 11. BasePrintEmrForm (EMR print forms — exam-form/follow-up/nursing-care/procedure/surgery-*)

---


Tài liệu cho AI agents hiểu toàn bộ hệ thống in ấn FE.
Đọc file này trước khi sửa bất kỳ component in nào.

---

## 0. HAI NHÁNH IN — xác định đúng nhánh TRƯỚC khi sửa

Hệ thống có **2 cơ chế in khác nhau**, đừng áp luật của nhánh này sang nhánh kia:

| | Nhánh A — PrintLayout | Nhánh B — EMR |
|---|---|---|
| Thư mục | `src/app/prints/` | `src/app/modules/emr/prints/` |
| Route | `/print/*` (`prints.routes.ts`) | `/emr/print/*` (`modules/emr/emr-print.routes.ts`) |
| Layout bọc ngoài | ✅ `PrintLayout` (header/footer chung) | ❌ không có |
| Gọi `window.print()` | ❌ CẤM — PrintLayout lo | ✅ component TỰ gọi |
| `openPrint` + cache sessionStorage | ✅ chuẩn | ❌ không dùng |
| Base class | `BasePrint` | `BasePrintMedicalRecord` → `BasePrintEmrForm` |
| Nội dung | Phiếu thu/đơn hàng/báo cáo/lịch hẹn | Bệnh án BA-xx, phiếu khám/điều dưỡng/phẫu thuật |

Trước 2026-08-14 hai nhánh nằm lẫn trong cùng `src/app/prints/` nên checklist §5
("KHÔNG gọi window.print()") mâu thuẫn với 7 file buộc phải gọi. Đã tách thư mục —
**mục §1-§10 dưới đây chỉ áp cho nhánh A**; nhánh B xem §11.

---

## 1. Kiến trúc tổng quan (NHÁNH A)

```
Caller (BaseList/BaseReport component)
  │
  ├── openPrint(url, cacheKey, fetch$)   ← CHUẨN — có loading + cache
  │     │
  │     ├── showLoading('openPrint')
  │     ├── fetch$ → sessionStorage[cacheKey] = JSON.stringify(data)
  │     ├── lắng nghe postMessage('print-dialog-opening')
  │     └── printCommon(url + ?cacheKey=...)
  │
  └── printCommon(url)                   ← ĐƠN GIẢN — không loading, không cache
        └── iframe hidden append vào DOM

PrintLayout (wrapper component ở /print route)
  │
  ├── PrintHeaderLayout  (logo + company info)
  ├── <router-outlet />  (PrintXxx component)
  └── PrintFooterLayout  (ký tên)
  │
  └── triggerPrint() khi logo load xong hoặc fallback 800ms
        └── window.print() + postMessage('print-dialog-opening') → hideLoading

PrintXxx Component (BasePrint)
  ├── tryLoadFromCache(cacheKey)   ← đọc sessionStorage nếu có
  ├── hoặc gọi API trực tiếp (fallback nếu không có cache)
  ├── applyData(data)              ← set model + gọi printData.setModel()
  └── printData.setModel({company, branch?, customer?, layoutType?})
```

---

## 2. `openPrint` vs `printCommon` — khi nào dùng cái nào

| | `openPrint` | `printCommon` |
|---|---|---|
| Loading indicator | ✅ showLoading → hideLoading | ❌ không |
| Data prefetch | ✅ fetch$ trước khi mở iframe | ❌ print component tự fetch |
| sessionStorage cache | ✅ lưu + truyền cacheKey | ❌ không |
| PostMessage trigger | ✅ hide loading khi dialog mở | ❌ không |
| Dùng khi | Đa số trường hợp — chuẩn | Chỉ print đơn giản không cần loading |

**Quy tắc:** Mọi print button mới → dùng `openPrint`. Chỉ dùng `printCommon` khi print component không có loading và data tự fetch.

```typescript
// ✅ CHUẨN
printPaymentInDay(datePrint: any) {
    const url = this.route.serializeUrl(
        this.route.createUrlTree([`/print/print-payment-in-day/${datePrint}`]),
    );
    const fetch$ = this.reportRevenueService.getPrint({
        params: { '_ngay_thang[gte]': datePrint, '_ngay_thang[lte]': parseInt(datePrint) + 86399 },
    });
    this.openPrint(url, `print-payment-in-day-${datePrint}`, fetch$);
}

// ❌ THIẾU LOADING — dùng printCommon
printPaymentInDay(datePrint: any) {
    const url = ...;
    this.printCommon(url);  // không có loading tím
}
```

---

## 3. `PrintLayout` — cơ chế trigger `window.print()`

**File:** `src/app/layout/print/print.layout.ts`

`PrintLayout` là parent component bao quanh tất cả print pages (route `/print`). Nó tự động gọi `window.print()` — print component KHÔNG gọi window.print() trực tiếp.

### Trigger logic

```
printData.setModel({ company: X, layoutType: Y })
    ↓
PrintLayout detects companyId() is truthy
    ↓
Case 1: layoutType = 'none'
    → setTimeout(triggerPrint, 200)  ← fast path

Case 2: layoutType khác 'none'
    → Đợi logo load (onLogoLoaded callback)
    → triggerPrint() ngay khi logo xong
    → Fallback timer 800ms nếu logo không load
```

### `triggerPrint()` làm gì

```typescript
private triggerPrint(): void {
    if (this.printTriggered()) return;          // guard chạy 1 lần
    this.printTriggered.set(true);
    requestAnimationFrame(() => {
        setTimeout(() => {
            if (window.parent !== window) {
                window.parent.postMessage('print-dialog-opening', '*'); // → hideLoading
            }
            window.print();
        }, 100);
    });
}
```

### (đã bỏ, D15/CRM standalone) `company` phải truthy

Trước D15, `PrintContext.company` (company_id) là field REQUIRED để load header/footer — thiếu
truthy check (`companyActiveId() ?? undefined` thay vì fallback `companyListId()[0]`) làm effect
không bao giờ chạy. Verify 2026-09-28: field `company` đã xoá hoàn toàn khỏi `PrintContext`
(`src/app/layout/print/print-data.service.ts`) cùng company — hồ sơ thương hiệu (tên, logo...) giờ
đọc qua `authBusinessProfile`, không cần truyền qua `PrintContext` nữa.

---

## 4. `PrintContext` — các field và `layoutType`

**File:** `src/app/layout/print/print-data.service.ts`

```typescript
interface PrintContext {
    branch?: number;        // optional — branch_id
    customer?: CustomerModel; // optional — hiển thị footer "Bệnh nhân ký"
    layoutType?: string;    // quyết định footer behavior
    doctorName?: string;    // optional
}
```

### `layoutType` values

| Value | Footer behavior | Dùng khi |
|---|---|---|
| `'default'` (hoặc undefined) | Footer chỉ hiện khi có `customer` | Print order, list |
| `'with-accountant'` | Footer luôn hiện (Kế toán trưởng / Người lập) | Payment, phiếu thu, báo cáo |
| `'none'` | Không footer, trigger print nhanh 200ms | Print lịch sử điều trị nhiều trang |

**Rule:** Phiếu tài chính (payment, báo cáo) → `'with-accountant'`. Phiếu bệnh nhân → truyền `customer`. In danh sách không cần footer → `'none'`.

---

## 5. Pattern chuẩn cho Print Component

```typescript
// ✅ CHUẨN — mirror PrintOrder, PrintPayment
export class PrintXxx extends BasePrint implements OnInit {
    modelService = inject(XxxService);
    private destroyRef = inject(DestroyRef);

    ngOnInit(): void {
        this.activatedRoute.params
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe((params: any) => {
                const cacheKey = this.activatedRoute.snapshot.queryParams['cacheKey'];
                const cached = this.tryLoadFromCache(cacheKey);
                if (cached) { this.applyData(cached); return; }

                this.modelService.getXxx(params.id)
                    .subscribe((data: any) => this.applyData(data));
            });
    }

    applyData(data: any): void {
        this.model.set(data);
        this.printData.setModel({
            branch: data?.branch_id,
            customer: data?.customer,
            layoutType: 'with-accountant',  // hoặc layoutType phù hợp
        });
        setTimeout(() => void this.handlePrintAfterInit());
    }
}
```

**Checklist khi viết Print Component:**
- [ ] Extend `BasePrint` (không phải `BaseList`)
- [ ] Gọi `tryLoadFromCache(cacheKey)` trước khi fetch API
- [ ] `applyData()` tách thành method riêng (dùng cho cả cache + API path)
- [ ] `this.models.set([this.model()])` — KHÔNG dùng `models.update(current => [...current, ...])` → tích lũy duplicate rows khi route change
- [ ] Chọn đúng `layoutType`
- [ ] KHÔNG gọi `window.print()` — PrintLayout lo
- [ ] Fetch 1 observable → direct `.subscribe()`, KHÔNG bọc trong `forkJoin({ key: obs$ })`

---

## 6. Caller — Pattern mở print từ BaseList/BaseReport

```typescript
// ✅ CHUẨN (khi có fetch$ riêng)
printXxx(id: number) {
    const url = this.route.serializeUrl(
        this.route.createUrlTree([`/print/print-xxx/${id}`]),
    );
    this.openPrint(url, `print-xxx-${id}`, this.modelService.printXxx(id));
}

// ✅ CHUẨN (khi print component tự đủ data từ :id)
printXxx(id: number) {
    const url = this.route.serializeUrl(
        this.route.createUrlTree([`/print/print-xxx/${id}`]),
    );
    this.openPrint(url, `print-xxx-${id}`, this.modelService.getXxx(id));
}
```

`openPrint` và `printCommon` đều có sẵn từ `BaseApp` → không cần inject thêm.

---

## 7. `apiService.get` — params phải dùng `{ params: { ... } }`

`apiService.get(path, options)` nhận `ApiOptions` object. `options.params` mới là HTTP query params.

```typescript
// ✅ ĐÚNG — options.params được Angular HttpClient đọc
this.apiService.get('print/payment-in-day', {
    params: { '_ngay_thang[gte]': from, '_ngay_thang[lte]': to },
});

// ❌ SAI — object flat không có params key → gửi request KHÔNG có query params
this.apiService.get('print/payment-in-day', {
    '_ngay_thang[gte]': from,   // không đúng cấu trúc ApiOptions
    '_ngay_thang[lte]': to,
});
```

`ApiService` LUÔN trả body (`Observable<T>`) — `ApiOptions` cố ý KHÔNG có `observe`, đừng thêm lại.
Cần `HttpEvent` (thanh tiến độ upload, đọc header response) thì tiêm thẳng `HttpClient`, xem
`header-sync.service.ts` / `teamchat.service.ts#uploadFile`. Trước 2026-09-08 interface có khai
`observe`/`responseType` nhưng `createHttpOptions` vứt đi cả hai: 13 chỗ `importData` tưởng mình
nhận tiến độ upload mà thực tế chỉ nhận body, còn `get<Blob>(..., { responseType: 'blob' })` bị đem
đi `JSON.parse` và ném `Unexpected token 'P'` ("PK" = magic bytes của .xlsx/zip). Nay `responseType`
đi đúng xuống `HttpClient` (neo bằng `api.service.spec.ts`), nên tải file KHÔNG cần tự tiêm
`HttpClient` nữa.

---

## 8. `PrintShareImport` — import minimal cho print pages

```typescript
// src/app/share/import/print-share-import.ts
export const PrintShareImport = [
    CommonModule, PhoneFormatPipe, TableModule,
    CurrencyFormatPipe, CustomDatePipe,
    TranslatePipe, NumberFormatPipe, SafeHtmlPipe,
];
```

Print component import `[...PrintShareImport]` — KHÔNG dùng `ListShareImports` (quá nặng).

`PrintShareImport` đã bao gồm `TranslatePipe` — KHÔNG import lại trong `imports: [...]` của component.

---

## 9. Gotchas — Lỗi thường gặp khi viết print

### Trang trắng thừa khi in

**Nguyên nhân:** `.sheet` có `min-height` + `page-break-after: always` áp dụng cả khi print.

**Fix trong `@media print` của SCSS:**
```scss
@media print {
  .sheet {
    min-height: 0 !important;
  }
  .sheet:last-child {
    page-break-after: auto !important;
  }
}
```

`page-break-after: always` giữ ở global `.sheet` (cần cho multi-page). Override `:last-child` để trang cuối không sinh trang trắng.

### BE: array response có thể nil → FE crash

Khi BE trả `[]map[string]interface{}` nhưng không có dữ liệu → Go marshal thành `null`, KHÔNG phải `[]`.

```go
// ✅ Guard trước c.JSON
if historyPayment == nil {
    historyPayment = []map[string]interface{}{}
}
```

FE dùng `@for` trên `null` → runtime error.

---

## 10. Files và route map

| Route | Component | Caller |
|---|---|---|
| `/print/print-order/:id` | `PrintOrder` | `order-plan-in-customer.ts` → `openPrint` |
| `/print/print-payment/:id` | `PrintPayment` | payment caller → `openPrint` |
| `/print/print-payment-in-day/:datePrint` | `PrintPaymentInDay` | `by-date.ts` → `openPrint` |
| `/print/print-treatment/:id` | `PrintTreatment` | `layoutType: 'none'` |
| `/print/print-labo-order/:id` | `PrintLaboOrder` | labo caller |
| `/print/print-prescription/:id` | `PrintPrescription` | med caller |
| `/print/print-reception-payment` | `PrintReceptionPayment` | reception |
| `/print/print-appointment-byhour-list` | `PrintAppointmentByhourList` | dashboard |
| `/print/print-appointment-bydoctor-hour-list` | `PrintAppointmentBydoctorList` | dashboard |

**Reference files:**
- `PrintLayout`: `src/app/layout/print/print.layout.ts`
- `PrintDataService`: `src/app/layout/print/print-data.service.ts`
- `BasePrint`: `src/app/share/base/base-print.ts`
- `BasePrintMedicalRecord`: `src/app/share/base/base-print-medical-record.ts` (nhánh B)
- `BasePrintEmrForm`: `src/app/modules/emr/prints/base-print-emr-form.ts` (nhánh B)
- `BaseApp.openPrint/printCommon`: `src/app/share/base/BaseApp.ts`
- Route config nhánh A: `src/app/prints/prints.routes.ts`
- Route config nhánh B: `src/app/modules/emr/emr-print.routes.ts`

---

## 11. NHÁNH B — BasePrintEmrForm (exam-form/follow-up/nursing-care/procedure/surgery-*)

**Thư mục: `src/app/modules/emr/prints/`** (tách khỏi `src/app/prints/` ngày 2026-08-14 — xem §0).
Nhánh này KHÔNG có `PrintLayout` bọc ngoài, nên component **tự gọi `window.print()`** và tự dựng
header — luật "KHÔNG gọi window.print()" ở §5 chỉ áp cho nhánh A.


### Khuôn dùng chung nhánh B — SỬA 1 CHỖ, KHÔNG chép lại (gom 2026-08-18)

Trước đó 8 trang in (2 mẫu BA + 6 phiếu) mỗi cái tự chép header + khuôn trang → in 1 bộ hồ sơ
ra 3 kiểu khác nhau. Thêm mẫu mới PHẢI dùng 2 khuôn này:

| Việc | Dùng | Ghi chú |
|---|---|---|
| Header trang in | `<app-print-emr-form-header [model] msCode authorityLabel [centerOnPage]>` | slot `[headerLeftExtra]` / `[headerCenter]` / `[headerRight]`. `variant="clinic"` = tờ giao bệnh nhân (logo + địa chỉ + hotline), mặc định `byt` = 3 cột form BYT |
| Khổ giấy + preview A4 | `@use '../emr-print-page' as print;` + `@include print.emr-sheet()` | thêm `emr-watermark()` nếu có logo mờ, `emr-fill-lines($fill-sm-width)` nếu có dòng `.f-line` |
| Mẫu tự dựng khung riêng (BA-xx, bìa) | `margin: print.$emr-page-margin` | 1 nguồn lề duy nhất: `20mm 20mm 25mm 20mm` cho cả hồ sơ (đục lỗ/đóng gáy không ăn chữ) |

- **Lề PHẢI ở `@page`, không phải `padding` của `.sheet`**: padding chỉ tạo lề trang ĐẦU → phiếu
  nhiều trang (theo dõi điều trị, chăm sóc) từ trang 2 in sát mép giấy. Bug này tồn tại tới 2026-08-18.
- **KHÔNG dùng `:host` trong SCSS trang in**: mọi component print đều `ViewEncapsulation.None` →
  Angular không rewrite `:host`, browser bỏ luôn rule (CSS chết). Dùng `body`.
- **Vùng để trống cho viết tay: dùng `.write-line` (`emr-fill-lines()`), KHÔNG `dots(n)`.** `dots(n)`
  là chuỗi chấm liền = MỘT từ không có chỗ ngắt → trong cột hẹp không wrap được, tràn đè sang khối
  bên cạnh; độ dài phụ thuộc cỡ chữ nên không bao giờ thẳng lề phải. `dots(n)` chỉ dùng trong ô bảng
  bề rộng cố định (vd ô sinh hiệu `dots(9)`).
- **Khối `flex` (cột trái nội dung + bảng bên phải): mục kế tiếp muốn in NGANG bảng PHẢI đặt trong
  `<div class="flex-1 min-w-0">`** — để ngoài `flex` là bị đẩy xuống dưới đáy bảng, chừa mảng trắng
  bên trái. Tổng chiều cao cột trái phải NHỎ HƠN chiều cao bảng bên phải: đếm `.write-line`
  (28px/dòng) trước khi thêm.
- **Dòng ký "…, ngày … tháng … năm …": địa danh rút từ địa chỉ chi nhánh bằng `split(',').pop()`
  CHỈ hợp lệ khi địa chỉ có ≥2 phân đoạn**; 1 phân đoạn → trả rỗng, in `Ngày … tháng … năm …`.
  Không guard: số nhà bị in làm địa danh và đủ dài để vỡ dòng ký.
### Song ngữ VI/EN (nhánh B)

Bản EN in **ĐỒNG THỜI** với bản VI trên cùng tờ — KHÔNG phải đổi ngôn ngữ. Vì vậy chuỗi EN nằm thẳng trong
template, **KHÔNG đi qua `| translate`**: hồ sơ nộp theo TT 32/2023 phải giữ nguyên bản tiếng Việt, cơ chế
đổi ngôn ngữ sẽ làm mất nó.

- Rule `.en` (cùng dòng) / `.en-b` (xuống dòng riêng) nằm **top-level** trong `_emr-print-page.scss` — chỉ cần
  `@use '../emr-print-page' as print;` là có, KHÔNG phải include mixin nào. Style: italic, `#555555`, cùng cỡ chữ.
- ⚠ **Trong ô bảng hẹp, ô có `whitespace-nowrap` + `overflow-hidden`, ô bề rộng cố định (`w-[120px]`), và
  `.lbl` nowrap: BẮT BUỘC `.en-b`, cấm `.en` inline kể cả nhãn ngắn.** Chèn EN cùng dòng ở những chỗ đó
  **cắt mất giá trị (tên/số CCCD bệnh nhân) trong khi `tsc` và build đều xanh** — không có tín hiệu nào,
  chỉ lộ khi in thử.
- `.en` inline trong `.f-line` CHỈ an toàn khi đủ 3 điều: nhãn có `.lbl-long`, giá trị có `.val-wrap`, và ô
  được nới theo nhãn đã ghép; thiếu 1 trong 3 → giữ `.en-b`.
- Ghép EN vào nhãn thì bỏ dấu `:` của phần tiếng Việt: `Giới tính<span class="en">/ Gender:</span>`, không
  để thành `Giới tính:/ Gender:`.
- Nhãn ghép dài xấp xỉ bề rộng khối (vd khối ký `min-w-[45%]`) → thêm `whitespace-nowrap` để khối tự nới
  thay vì vỡ 2 dòng.
- Chia cột bằng `<colgroup>` % , KHÔNG flex `nowrap`. Nhãn dài: thêm `.lbl-long` (`flex:0 1 auto; white-space:normal`).
- Nghiệm thu layout KHÔNG chấp nhận "build xanh": phải in ra PDF khổ A4 thật và đo lại các ô nhãn dài nhất.

- Header là custom element (inline) → host của `PrintEmrFormHeader` đã set `block relative z-[1]`
  để nổi trên watermark (selector `.sheet > div` cũ không match custom element).

### Bìa hồ sơ bệnh án HC-01 (in kèm MỌI mẫu BA)

`print-ba-cover/` — `<app-ba-cover-front [model]="…"/>` (trang bìa) + `<app-ba-cover-back/>`
(2 trang cuối: Phần 1 thành phần HSBA, Phần 2 kiểm soát lưu trữ + ký giao/nhận). Mẫu HC-01,
Phụ lục TT 32/2023.

- Chèn ở **2 chỗ duy nhất**: `print-ba-ngoai-tru-frame.html` (phủ BA-02/03/04/05/06/07/general)
  và `print-ba-18.html` (RHM có template riêng). Sửa bìa KHÔNG cần đụng 9 mẫu.
- `ba-cover-back` KHÔNG có input — 2 trang cuối là mẫu điền tay 100% của bộ phận lưu trữ.
- Bìa dùng chung `@page` của mẫu bệnh án (lề 20mm). `@page` là khai báo global, không set riêng
  theo element → **KHÔNG** dùng margin âm để nong ra lề 10mm như docx gốc (Chrome cắt nội dung
  ngoài lề `@page`).
- Ngắt trang bằng cả `page-break-*` + `break-*`; `@media print` phải reset `min-height`/`padding`
  của preview A4 (§9) — không reset là ra trang trắng.

`:id` trong route `/emr/print/<kind>/:id` LUÔN được hiểu là `medical_record_id` (`loadMedicalRecord()` gọi thẳng `modelService.get(id)` trên bệnh án) — KHÔNG PHẢI id riêng của dòng ghi chú/phiếu (các bảng `emr_treatment_followup`/`emr_nursing_note`/`emr_patient_document` có sequence auto-increment ĐỘC LẬP, số có thể trùng ngẫu nhiên với `medical_record_id` khác → in NHẦM bệnh nhân).

Caller mở trang in PHẢI tự resolve đúng `medical_record_id` trước khi gọi — không truyền thẳng id của row đang thao tác.

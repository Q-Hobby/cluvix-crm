# Gotchas Form & List

## Mục lục

- `pTemplate` deprecated — phải dùng `#template` (Angular 17+)
- `shopspring/decimal` → `p-inputnumber` cần `Number()` cast
- `filter[status]` — PHẢI wire qua `valueChanges` + `dt.filter()`
- `apiService.get()` — KHÔNG tự unwrap `.data`
- `<audio>` — bind `[src]` trực tiếp, KHÔNG dùng `<source>`
- Date filter — pattern chuẩn (tham khảo `AppointmentInMenu`)
- Unix timestamp trong nested `lines[]` — `beforeSave` + `p-datePicker` (CRITICAL)
- `TagSeverity` type alias — phải ở file scope, không trong class
- `ApiService` bỏ qua `observe` option — `observe: 'events'` là dead code
- Auto-fill field theo control khác — guard bằng `.pristine`, không ghi đè field user đã sửa
- p-table virtualScroll → blank table, dùng infinite-scroll thủ công
- Action gọi 2 API nối tiếp: `finalize()` phải gắn ở API CUỐI
- GlobalLoadingService là ref-counter, không phải boolean
- Dialog cần data async lúc mở — prefetch ở caller, KHÔNG tự fetch bên trong dialog
- Picker có whitelist từ API — khởi tạo `[]`, không `null` (ca gốc `CompanyFormControl`, đã xoá D15)
- Custom CVA multiselect wrapper — onClear() phải tự refresh options
- `app-department-user-form-control` cần truyền `branchId` để filter (input `companyId` đã bỏ, D15) — component tự lo giữ selection
- `Validators.required` coi số `0` là HỢP LỆ — ô số bắt buộc phải thêm `Validators.min(1)`
- KHÔNG push FormGroup vào `FormArray.controls` thô — mất parent link, status stale (CRITICAL)
- Dialog `onClose` gate theo payload — bỏ sót reload khi đóng qua X/Esc/click-ngoài/back
- Record BE auto-create (best-effort, không qua handler Create chuẩn) thiếu field → `Validators.required` chặn Lưu câm lặng

---


> Đọc khi gặp triệu chứng liên quan p-table, p-inputnumber, filter, date picker, print field.

---

## `pTemplate` deprecated — phải dùng `#template` (Angular 17+)

`pTemplate="header"` / `pTemplate="body"` / `pTemplate="emptymessage"` là cú pháp cũ. **Tất cả `p-table` trong codebase phải dùng `#` template ref:**

```html
<!-- ❌ CŨ — không dùng -->
<ng-template pTemplate="header">...</ng-template>
<ng-template pTemplate="body" let-model>...</ng-template>
<ng-template pTemplate="emptymessage">...</ng-template>

<!-- ✅ ĐÚNG — Angular 17+ control flow -->
<ng-template #header>...</ng-template>
<ng-template #body let-model let-i="rowIndex">...</ng-template>
<ng-template #emptymessage>...</ng-template>
<ng-template #loadingbody>...</ng-template>
```

Áp dụng cho cả `p-datepicker` footer: `<ng-template #footer>` thay vì `pTemplate="footer"`.

## `shopspring/decimal` → `p-inputnumber` cần `Number()` cast

BE set `decimal.MarshalJSONWithoutQuotes = true` (main.go) → response từ struct trả **number**. Riêng endpoint map-scan (`[]map[string]interface{}`) thiếu `ParseDecimalFields` có thể trả **string** (`"15000000"`). `p-inputnumber` bound vào FormControl cần **numeric value** — cast defensive, nếu không field blank trong edit mode.

```typescript
// ❌ blank trong edit mode — decimal string không display
base_salary: new FormControl(model.base_salary ?? null),

// ✅ CORRECT
base_salary: new FormControl(model.base_salary ? Number(model.base_salary) : null),
```

Áp dụng cho mọi field type `decimal.Decimal` / `*decimal.Decimal` từ BE khi bind vào `p-inputnumber`.

## `filter[status]` — PHẢI wire qua `valueChanges` + `dt.filter()`

`(onChange)="getList()"` trên p-select filter sẽ gọi lại toàn bộ API. Đúng pattern là wire qua reactive:

```typescript
// ❌ SAI — gọi API thay vì filter table
<p-select (onChange)="getList()" .../>

// ✅ CORRECT — trong initFilter()
this.rfFilterModel.controls['filter[status]'].valueChanges
  .pipe(takeUntilDestroyed(this.destroyRef))
  .subscribe((val) => {
    this.dt.filter(val ?? '', 'filter[status]', 'equals');
  });
```

`this.destroyRef` được kế thừa từ `BaseList` (protected) — KHÔNG inject lại trong subclass.

## `apiService.get()` — KHÔNG tự unwrap `.data`

`apiService.get()` trả **raw HTTP body** (`{ data: {...}, _meta: {...} }`). Khác với `BaseApiService.list()` (đã unwrap). Mọi service method dùng `apiService.get()` phải tự pipe:

```typescript
// ❌ response.items === undefined — searchHandler đọc sai
historyCall(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/history-call`, params);
}

// ✅ ĐÚNG
historyCall(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/history-call`, params).pipe(
        map((res: any) => res?.data ?? res),
    );
}
```

**List/paginated endpoint khác shape** — `pagination.PaginateTyped` trả `{items, _meta}`, KHÔNG có `data`. Áp `res?.data ?? res` cho endpoint dạng này sẽ luôn ra `[]` (res là object, không phải array).

```typescript
// ❌ EmrAuditLogService.listByRecord từng làm vậy → dialog audit log luôn rỗng dù BE có data
const data = res?.data ?? res ?? [];
// ✅ list/paginated → đọc res.items
const data = res?.items ?? [];
```

## `<audio>` — bind `[src]` trực tiếp, KHÔNG dùng `<source>`

Angular không bind đúng khi `[src]` trên `<source>` bên trong `<audio>`:

```html
<!-- ❌ audio player không load được URL -->
<audio controls><source [src]="url"></audio>

<!-- ✅ ĐÚNG -->
<audio controls preload="none" [src]="url" style="height:32px;width:190px"></audio>
```

## Date filter — pattern chuẩn (tham khảo `AppointmentInMenu`)

```typescript
// initFilter() — date range
this.rfFilterModel.get('dateRange')!.valueChanges
  .pipe(debounceTime(500), distinctUntilChanged(dequal), takeUntilDestroyed(this.destroyRef))
  .subscribe((range) => { /* dispatch dt.filter */ });

// dropdown fields
(['field1', 'field2'] as const).forEach((field) => {
  this.rfFilterModel.get(field)!.valueChanges
    .pipe(debounceTime(300), distinctUntilChanged(dequal), takeUntilDestroyed(this.destroyRef))
    .subscribe((value) => this.dt.filter(value, field, 'in'));
});

// quick select handler — patchValue kích hoạt valueChanges
onQuickSelectDateRange({ start, end }: { start: Date; end: Date }) {
  this.rfFilterModel.patchValue({ dateRange: [start, end] });
}
clearDateRange() {
  this.rfFilterModel.patchValue({ dateRange: null });
}
```

HTML date picker:
```html
<p-datepicker formControlName="dateRange" selectionMode="range"
  appendTo="body" hourFormat="24" (onClear)="clearDateRange()">
  <ng-template #footer>
    <app-date-picker-footer (rangeChange)="onQuickSelectDateRange($event)" />
  </ng-template>
</p-datepicker>
```

Import: `DatePickerFooter` từ `@/share/components/date-picker-footer`.

## Unix timestamp trong nested `lines[]` — `beforeSave` + `p-datePicker` (CRITICAL)

`unixTimesTampFields` trong `BaseApiService` chỉ convert **top-level fields**. Nested fields như `lines[].expired_at` KHÔNG được convert tự động.

**Trước khi save** — service phải override `beforeSave()`:
```typescript
protected override beforeSave<P extends object>(saveParams: P): P {
  const params = super.beforeSave(saveParams) as any;
  if (Array.isArray(params.lines)) {
    params.lines = params.lines.map((line: any) => {
      if (line.expired_at instanceof Date)
        return { ...line, expired_at: this.timezoneService.dateHmsToUnix(line.expired_at, 0, 0, 0) };
      return line;
    });
  }
  return params as P;
}
```

**Khi init form / FEFO patch** — `p-datePicker` nhận `Date`, không nhận Unix int64:
```typescript
// buildLineGroup — load từ model API (Unix int64)
const expiredAt = line.expired_at ? new Date(line.expired_at * 1000) : null;

// autoFefoLine — FEFO trả về Unix int64
expired_at: a.ExpiredAt ? new Date(a.ExpiredAt * 1000) : null,
```

## `TagSeverity` type alias — phải ở file scope, không trong class

```typescript
// ❌ TS error — type alias không được phép trong class body
export class MyList extends BaseList<T> {
  type TagSeverity = 'success' | 'warn';  // ❌
}

// ✅ File scope (trước @Component)
type TagSeverity = 'success' | 'secondary' | 'info' | 'warn' | 'danger' | 'contrast';

@Component({ ... })
export class MyList extends BaseList<T> {
  readonly severity: Record<number, TagSeverity> = { 1: 'success', 2: 'warn' };
}
```

## `ApiService` bỏ qua `observe` option — `observe: 'events'` là dead code

`ApiService.createHttpOptions()` hardcode `observe: 'body'` bất kể options được truyền vào. Truyền `{ reportProgress: true, observe: 'events' }` không có tác dụng:

```typescript
// ❌ Options này bị bỏ qua hoàn toàn
this.apiService.post(url, body, { reportProgress: true, observe: 'events' })

// ✅ Dùng HttpClient trực tiếp nếu cần theo dõi upload progress thực sự
this.http.post(url, body, { reportProgress: true, observe: 'events' })
  .pipe(filter(e => e.type === HttpEventType.UploadProgress))
```

`reportProgress` được forward nhưng không có tác dụng nếu `observe !== 'events'`.

## Auto-fill field theo control khác — guard bằng `.pristine`, không ghi đè field user đã sửa

`setValue`/`patchValue` KHÔNG tự đổi `dirty`/`pristine` — chỉ user gõ tay mới mark dirty. Auto-fill 1 field theo lựa chọn ở field khác (vd chọn loại → tự điền tên) phải check `.pristine` trước khi set, nếu không sẽ ghi đè giá trị user đã tự sửa mỗi lần đổi lựa chọn:

```typescript
if (nameCtrl.pristine) nameCtrl.setValue(option.label);
```

---

## p-table virtualScroll → blank table, dùng infinite-scroll thủ công

`[virtualScroll]="true"` trên `p-table` phá render (0 dòng dù data có) khi kết hợp
`[scrollHeight]="'flex'"` trong layout list hiện tại — đã revert 2 lần thực tế. KHÔNG dùng
virtualScroll cho `p-table` trong codebase này. List lớn cần lazy-load → BE phân trang theo
nhóm (không phân trang thẳng theo row), FE gắn scroll listener thủ công:
```ts
table.el.nativeElement.querySelector('.p-datatable-table-container')
  .addEventListener('scroll', ...) // load thêm khi gần đáy
```
Sau mỗi lần load, phải check `scrollHeight <= clientHeight` qua `afterNextRender()` để tự load
tiếp nếu trang hiện tại chưa tràn viewport — nếu không, browser không bắn `scroll` event và
phần data còn lại vĩnh viễn không tải được.

---

## Action gọi 2 API nối tiếp: `finalize()` phải gắn ở API CUỐI

Nút kiểu "Lưu và duyệt" / "Lưu và bắt đầu" gọi `create()` rồi `approve(id)`/`start(id)` trong nhánh
`next`. Gắn `finalize(() => flag.set(false))` lên observable của API ĐẦU → cờ tắt ngay khi `create()`
xong, trong khi API thứ 2 còn đang chạy → **nút bật lại, bấm lần 2 tạo chứng từ TRÙNG**.

Gắn `finalize` ở API cuối (cả nhánh thành công lẫn lỗi), hoặc tắt cờ thủ công trong từng nhánh kết thúc
thật. Kiểm thêm: 2 nút submit khác nhau trên cùng form phải chặn lẫn nhau (mỗi handler check cả 2 cờ).

---

## GlobalLoadingService là ref-counter, không phải boolean

`showLoading(key)`/`hideLoading(key)` cộng/trừ vào `Map<string, number>`. Code có nhánh
return sớm SAU khi đã gọi `showLoading()` mà không đi qua `finalize()` (vd guard
`if (seq !== requestSeq) return`) → phải tự gọi `hideLoading(key)` ngay tại nhánh đó, nếu
không count không bao giờ về 0 → loading global stuck vĩnh viễn.

`showLoading(key, labelKey?)`/`track(key, stream$, labelKey?)` — `labelKey` optional (i18n key)
để loader tím hiện text theo ngữ cảnh thay vì "Loading..." cố định. CUD (POST/PUT/PATCH/DELETE)
qua `loading.interceptor.ts` tự suy luận (`loading-label.util.ts`). **GET thì interceptor CHỈ chạy
progress bar, không gắn nhãn** — overlay của export `?_export=1` do component tự bật nên nhãn suy từ
chính key `showLoading('exportXxx')`. Đặt tên key có `export`/`import` là đủ; key không nhận diện
được → rơi về "Đang tải...".

---

## Dialog cần data async lúc mở — prefetch ở caller, KHÔNG tự fetch bên trong dialog

Component cha gọi `loadingService.track(key, obs$)` rồi mới `dialogWrapperService.open(...)`
với data đã có sẵn — giống pattern `frontend/skill/skills/building-list-and-export-views/reference/fe-print.md` ("Data prefetch trước khi mở iframe").
KHÔNG để dialog tự gọi API trong `ngOnInit` rồi gate template `@if (model)`: nếu `subscribe()`
thiếu `error` handler, fetch lỗi → `model` không bao giờ set → spinner (gate theo model, không
theo `loading()`) treo vĩnh viễn, không có lối thoát ngoài đóng dialog thủ công.

---

## Picker có whitelist từ API — khởi tạo `[]`, không `null`

> ⚠ Lịch sử (D15): ca gốc là `CompanyFormControl.restrictToCompanyIds`, control đã xoá cùng company (story-61).

Bài học chuyển được cho mọi picker có whitelist từ API: khởi tạo signal truyền vào bằng `[]` (không
phải `null`) — tránh flicker hiện TẤT CẢ option trong lúc chờ API trả whitelist.

---

## Custom CVA multiselect wrapper — onClear() phải tự refresh options

`p-multiselect.clear()` (nút "x") chỉ emit `(onClear)`, KHÔNG emit `(onChange)` — nếu wrapper
chỉ có logic refresh `items` bên trong `onChange`/`writeValue`, `onClear()` phải tự gọi lại
hàm refresh đó, không được chỉ set giá trị rồi trông chờ round-trip từ form cha.
Với control dùng `formControlName` (Reactive Forms), round-trip này KHÔNG xảy ra —
Angular gọi `setValue(value, { emitModelToViewChange: false })` để chống loop,
`writeValue()` sẽ không được gọi lại cho chính control vừa đổi.
→ `onClear()` phải tự gọi lại `loadItems()`/hàm refresh, y hệt `writeValue()`.

`p-select` (chọn 1) thì NGƯỢC lại: `clear()` emit CẢ `(onChange)` (value `null`) LẪN `(onClear)` — bind cả 2 vào
handler có side-effect (confirm, xoá data) ⇒ chạy 2 lần (2 dialog chồng). Chỉ bind `(onChange)` và xử lý nhánh `null`.

Không nên guard refresh bằng `if (cache.getAll().length > 0)` trước khi gọi lại —
nếu cache đang tạm rỗng do `invalidateMemoryCache()` (CUD/Dexie sync nơi khác), guard
fail silently và `items` kẹt stale/rỗng vĩnh viễn. Gọi refresh không điều kiện.

---

## `app-department-user-form-control` cần truyền `branchId` để filter — component tự lo giữ selection

> Input `[companyId]` cũ đã bỏ (D15) — component nay chỉ có `[department]`, `[branchId]`, `[activeOnly]`.

Component đã filter sẵn theo `[branchId]` input và chỉ clear selection khi user
hiện tại KHÔNG còn thuộc filter mới (`user-form-control.ts` ngOnChanges) — nhiều form dùng
component này chỉ truyền `[department]` mà quên bind `[branchId]="f['branch_id'].value"`, khiến
dropdown load toàn bộ user thay vì lọc theo chi nhánh đang chọn.

---

## `Validators.required` coi số `0` là HỢP LỆ — ô số bắt buộc phải thêm `Validators.min(1)`

- `required` chỉ bắt null/''/undefined; control số set 0 (vd `quantity: 0` sau khi chọn dịch vụ) vẫn valid → không báo đỏ, lọt xuống BE.
- Fix: `[Validators.required, Validators.min(1)]`. Đã dính: `quantity` trong `order-plan-form.ts addServiceUsed()`.

---

## KHÔNG push FormGroup vào `FormArray.controls` thô — mất parent link, status stale (CRITICAL)

```ts
// ❌ SAI — Array.push thường: row không được set parent → validity của row KHÔNG bubble lên
// FormArray → array kẹt status cũ (vd INVALID vĩnh viễn dù mọi ô đã valid) → form chặn submit im lặng
const rows = this.rfModel.controls['services_used']['controls'] as FormGroup[];
const key = rows.push(fb) - 1;

// ✅ ĐÚNG — FormArray.push: set parent + tự re-aggregate validity
const arr = this.rfModel.controls['services_used'] as FormArray;
const key = arr.length;
arr.push(fb);
```

- Áp dụng cho MỌI thao tác cấu trúc trên FormArray: thêm row = `arr.push(fb)`, xóa row = `arr.removeAt(i)` — KHÔNG `controls.push()`/`controls.splice()` thô.
- Triệu chứng nhận diện: `rfModel.invalid === true` nhưng duyệt mọi control lá đều VALID; thao tác nào đó gọi đúng API chuẩn (`removeAt`, `patchValue` cả array) thì form "tự nhiên" hợp lệ lại.
- Đã dính: `order-plan-form.ts addServiceUsed()` + cùng pattern ở `service-combo-form.ts`, `med-prescription-form.ts` (cả push lẫn splice), `med-customer-prescription-form.ts` (fix 2026-07-22).

---

## Dialog `onClose` gate theo payload — bỏ sót reload khi đóng qua X/Esc/click-ngoài/back

`dialogWrapperService.open()` mặc định `closable/dismissableMask/closeOnEscape=true` — mọi đường đóng này gọi `ref.close()` KHÔNG payload. Nếu `onClose.subscribe((result) => { if (result?.saved) onConnected(); })` gate reload theo payload (chỉ set khi bấm đúng nút trong dialog), user đóng dialog theo cách tự nhiên khác sau khi lưu thành công → list cha không refresh (data đã lưu DB, chỉ list stale).

Fix: `onClose.subscribe(() => onConnected())` LUÔN reload (rẻ, vô hại); toast/side-effect cần biết kết quả thật thì đặt NGAY trong callback thành công của action đó, không phụ thuộc payload lúc đóng dialog.

---

## Record BE auto-create (best-effort, không qua handler Create chuẩn) thiếu field → `Validators.required` chặn Lưu câm lặng

Record tạo tự động ở BE (map insert trực tiếp trong hook/service, không qua handler Create form-facing) dễ bỏ trống field mà form edit khai `Validators.required`. Nếu nhánh backfill default khi edit (`ngOnInit`) chỉ xử lý 1-2 field cũ (vd `customer_care_result_id`/`staff_id`) mà quên field mới/khác → nút Lưu disable, không network request, không toast lỗi — dễ tưởng nhầm là bug FK/BE.

Debug "không lưu được" mà console/network sạch → so toàn bộ `Validators.required` của `buildForm()` với field thực sự được set ở nguồn tạo record (BE service/hook), không chỉ tin field hiển thị trên UI. Đã dính: `customer_care` auto-tạo bởi `treatment_course.go createCustomerCare` Bước 1 (option `user_option id=19`) thiếu `channel_care`/`customer_care_type_id`/`branch_id`/`company_id` — fix 2026-08-08 (lịch sử: module care + cột company đã bỏ, D2/D15; bài học vẫn đúng).

Đã dính: `LivechatConnectDialog`, `MetaPageSelectDialog` (fix check-component 2026-07-23).

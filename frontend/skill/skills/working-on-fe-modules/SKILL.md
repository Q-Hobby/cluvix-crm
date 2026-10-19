---
name: working-on-fe-modules
description: >-
  Base class patterns (BaseApp, BaseList, BaseForm, dialog footer, import barrels),
  shared services (unit conversion, LocaleFormatService, ECharts), IndexedDB sync,
  service worker, breadcrumb, và pattern riêng module CRM/Config. Dùng skill này
  KHI bắt đầu làm việc trên bất kỳ module FE nào — trước khi tạo component/service
  mới, thêm route mới, hoặc đụng IndexedDB table — để tránh dựng lại pattern đã có sẵn.
---

# Working on FE modules

> Gotchas chung (LazyImgPipe, interceptor dedup, forkJoin, monaco, FormArray, p-table,
> destroyRef, Tailwind arbitrary-value): `reference/fe-patterns-gotchas.md`
> IndexedDB sync (table-registry, bump INDEXEDDB_DB_VERSION): `reference/fe-indexeddb.md`
> Breadcrumb (route `data.breadcrumb`, nhãn động tên bản ghi, vì sao detail mất cấp cha): `reference/fe-breadcrumb.md`
> Custom Service Worker: `reference/fe-service-worker.md`
> Module Config — toggle user_option: `reference/module-config-option-crm.md`



Tài liệu cho AI agents hiểu hierarchy base class và khi nào dùng cái nào.
Đọc file này trước khi tạo component mới hoặc sửa list/form/detail.

---

## 1. Hierarchy tổng quan

```
BaseApp (root @Directive)
  ├── BaseList<T>        — list/table với pagination, sort, filter
  ├── BaseForm<T>        — dialog form (create/update)
  ├── BaseDetail<T>      — single record view (routed)
  ├── BaseDetailPanel<T> — panel chi tiết SlideOver (xổ từ mép phải)
  ├── BaseReport<T>      — report với export Excel
  └── BaseDetailDialog<T>— detail trong dialog (không route)

BasePrint (@Injectable)  — print pages (KHÔNG extend BaseApp)
BasePrintMedicalRecord   — EMR discharge print (extend @Directive riêng)
```

---

## 2. `BaseApp` — root, inject sẵn 15 services

**File:** `src/app/share/base/BaseApp.ts`

Mọi class extend BaseApp đều có sẵn (KHÔNG cần inject lại):

| Property | Type | Dùng cho |
|---|---|---|
| `noti` | NotificationService | toast success/warn/error |
| `route` | Router | navigate, serializeUrl |
| `fb` | FormBuilder | tạo FormGroup/FormControl |
| `translate` | TranslateService | instant(key) |
| `confirmationService` | ConfirmationService | popover confirm (target: anchor) — dùng `AppConfirmDialog` cho modal confirm |
| `dialogWrapperService` | DialogWrapperService | mở dialog |
| `loadingService` | GlobalLoadingService | show/hide loading tím |
| `storageService` | StorageService | localStorage wrapper |
| `timezoneService` | TimezoneService | now(), monthStartUnix() |
| `authService` | AuthService | login/logout |
| `rbacService` | RbacService | permissions |
| `isDesktop/isTablet/isMobile` | signal\<boolean\> | device detection |
| `isSubmitting` | signal\<boolean\> | form submit state |
| `loading` | signal\<boolean\> | table loading state |

⚠ Redeclare 1 property đã có (vd `private rbacService = inject(RbacService)`) → **TS2415 "incorrectly extends base class"** (lỗi khó hiểu, không chỉ thẳng nguyên nhân) — dùng `this.rbacService` kế thừa.

**Form chỉ mở qua dialog** (không nhúng template nào khác): gọi thẳng `dialogWrapperService.open(FormComponent, { header, data })`, KHÔNG tạo component wrapper riêng để convert `@Input` → `DynamicDialogConfig.data`. Trong `FormComponent`:
- `private dialogConfig = inject(DynamicDialogConfig)` (bắt buộc, KHÔNG `optional`) — đọc field trực tiếp từ `config.data`, không khai báo `@Input`.
- `private dialogRef = inject(DynamicDialogRef)`, đóng bằng `dialogRef.close(payload)`.
- Ref: `CustomerForm`, `FormNursingNote`.
- Component vừa mở qua dialog VỪA nhúng template khác (dual-mode) → mới cần `inject(DynamicDialogConfig, { optional: true })` (ví dụ `frontend/skill/skills/building-list-and-export-views/reference/fe-patterns-import-export.md`).

Print methods (từ BaseApp):
- `printCommon(url)` — mở iframe ẩn, không loading
- `openPrint(url, cacheKey, fetch$)` — fetch → loading → sessionStorage → iframe
- `openPrintWindow(url, cacheKey, fetch$)` — fetch → window.open tab mới

---

## 3. `BaseList<T>` — list/table CRUD

**File:** `src/app/share/base/base-list.ts`
**Pattern:** extend + inject `modelService` + gọi `initBaseList()` trong `ngOnInit`

```typescript
@Component({ ... imports: [...ListShareImports], standalone: true })
export class CustomerList extends BaseList<CustomerModel> implements OnInit {
    modelService = inject(CustomerService);

    ngOnInit() {
        this.defaultSearch.set({ status: 1 });   // filter mặc định
        this.requiredSearch.set({ type: 'A' });  // filter bắt buộc, không đổi
        this.initBaseList();                      // load state + gọi getList()
    }

    // override nếu cần custom search (default dùng modelService.list)
    override searchHandler() {
        this.modelService.customList({ params: this.searchParams() }).subscribe(...);
    }
}
```

### Signals chính

| Signal | Default | Mô tả |
|---|---|---|
| `models` | `[]` | mảng item hiển thị |
| `page` | `1` | trang hiện tại |
| `perPage` | `10` | rows per page |
| `sort` | `'-id'` | sort field (prefix `-` = DESC) |
| `search` | `{}` | filter user thay đổi |
| `defaultSearch` | `{}` | filter mặc định (merge vào params) |
| `requiredSearch` | `{}` | filter bắt buộc (không thể xóa) |
| `loading` | `false` | skeleton rows |
| `selectedRecords` | `[]` | checked rows |
| `cols` | `[]` | column definitions |

Toggle/action theo từng row trong list (switch, revoke...) PHẢI khoá in-flight theo id riêng
(`Set<id>` signal, vd `revokingId`/`updatingBranchId` ở `omni-channel-settings.ts`) — signal
`loading` global không đủ, bấm chồng request sẽ race ghi đè sai state.

### `searchParams` computed

```typescript
// Tự động merge:
readonly searchParams = computed(() => ({
    page: page(),
    'per-page': perPage(),
    sort: sort(),
    ...defaultSearch(),
    ...search(),
    ...requiredSearch(),
}));
```

### Methods thường dùng

```typescript
this.getList()                    // reset page + reload
this.updateRow(updatedModel)      // update 1 item trong models() in-place
this.deleteRecord(event, id)      // confirm dialog + DELETE
this.createRecord()               // mở FormDialog (cần formComponent set)
this.updateRecord(model)          // mở FormDialog với data
this.openPrint(url, key, fetch$)  // print với loading
```

### `initBaseList()` làm gì

1. Load persisted state (cols, filter, sort) từ localStorage
2. Gọi `afterBaseListInit()` hook (override nếu cần)
3. Gọi `getList()` → `searchHandler()`

### CRITICAL: `route` trong `BaseApp` là `Router` — KHÔNG khai báo lại

`BaseApp` đã inject `protected route: Router`. Nếu child khai báo `private route = inject(ActivatedRoute)` → conflict type error.  
→ Dùng tên khác: `private activatedRoute = inject(ActivatedRoute)`.  
→ Dùng `this.route.navigate(...)` cho Router (từ BaseApp), `this.activatedRoute.queryParams` cho ActivatedRoute.

### CRITICAL: List endpoint không có CRUD chuẩn — override `searchHandler` hoặc dùng `override list()`

Khi `modelService.list()` gọi sai endpoint (vd BE dùng custom path), override `searchHandler()` trong component:
```typescript
override searchHandler() {
  this.myService.customList(this.searchParams()).subscribe(res => {
    this.models.set(res.items ?? []);
    this.paginationInfo.set(res._meta ?? null);
    this.loading.set(false);
  });
}
```
Hoặc override `list()` trong service nếu muốn giữ `searchHandler` default.

**CRITICAL:** override `searchHandler()` PHẢI giữ `.pipe(takeUntilDestroyed(this.destroyRef), ...)` — bản gốc `BaseList.searchHandler()` có sẵn, copy-paste thiếu → subscription leak khi component destroy giữa lúc request đang chạy. Nếu override không có custom logic gì khác (chỉ gọi lại y hệt default) → xoá override, dùng default của `BaseList`.

❌ `this.modelService.list({})` — quên `searchParams()` → page/sort/filter bị BE ignore im lặng,
không lỗi, không crash, chỉ luôn trả đúng 1 trang mặc định.

### CRITICAL: `@Input` + `BaseList` — phải có `ngOnInit` riêng

`BaseList` KHÔNG tự gọi `initBaseList()`. Nếu child component nhận `@Input` (vd `brokerId`) và dùng `@Input` trong `searchHandler` hay `initRequiredFilter`, PHẢI tự implement `ngOnInit`:

```typescript
// ❌ Sai — list không bao giờ load
export class ServiceCommission extends BaseList<any> {
  @Input({ required: true }) brokerId!: number;
  override searchHandler() { this.brokerSvc.list(this.brokerId, ...) }
}

// ✅ Đúng
export class ServiceCommission extends BaseList<any> implements OnInit {
  @Input({ required: true }) brokerId!: number;
  ngOnInit() { this.initBaseList(); }
  override searchHandler() { this.brokerSvc.list(this.brokerId, ...) }
}
```

### Pattern: BE trả flat alias → FE remap trước `addLabels`

Khi BE SELECT trả `customer_fullname` (flat) thay vì nested `customer.fullname`, service `addLabel` sẽ đọc sai:

```typescript
// BE trả: { customer_fullname: 'A', customer_phone: '09...' }
// addLabel đọc: item.customer.fullname → undefined ❌

// ✅ Remap trước khi gọi addLabels:
const nested = raw.map((item: any) => ({
  ...item,
  customer: {
    fullname: item.customer_fullname ?? '',
    nickname: item.customer_nickname ?? null,
    phone: item.customer_phone ?? '',
  },
}));
const itemsWithLabels = await this.modelService.addLabels(nested);
```

---

## 4. `BaseForm<T>` — dialog form

**File:** `src/app/share/base/base-form.ts`
**Pattern:** extend + implement `buildForm()` + set `modelService`

```typescript
@Component({ ... })
export class CustomerForm extends BaseForm<CustomerModel> implements OnInit {
    modelService = inject(CustomerService);

    // BẮT BUỘC implement — gán rfModel
    protected buildForm(model: CustomerModel): void {
        this.rfModel = this.fb.group({
            id: [model?.id],
            fullname: [model?.fullname, Validators.required],
            phone: [model?.phone, [Validators.required, phoneVnValidator]],
            company_id: [model?.company_id],
            version: [model?.version ?? 0],
        });
    }
}
```

### Signals chính

| Signal | Mô tả |
|---|---|
| `id` | null = create mode, number = edit mode |
| `model` | data từ API hoặc dialog config |
| `clone` | structuredClone của model (so sánh dirty) |
| `isEditMode` | computed: `!!model()?.id` |
| `submitted` | true sau khi bấm submit (validation) |

### `isFieldInvalid()` — không thêm gate touched

→ Xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.13 (hiện lỗi ngay khi render, KHÔNG gate `touched || dirty || submitted()`).

### `getPayload()` — 2 mode

```typescript
// mode mặc định — rfModel.value (chỉ enabled fields)
const payload = this.getPayload();

// sendRaw mode — rfModel.getRawValue() (kể cả disabled)
this.sendRaw.set(true);
const payload = this.getPayload();
```

### `initialValue` @Input

```typescript
// Caller truyền vào khi mở dialog
this.dialogWrapperService.open(CustomerForm, {
    data: { model: customer },
});

// Trong form component — merge với model
@Input() initialValue: Partial<CustomerModel> = {};
```

### Footer chuẩn — `app-dialog-footer`

**BẮT BUỘC** dùng `app-dialog-footer` cho mọi form dialog, không tự build footer:

```html
<app-dialog-footer
  [isSubmitting]="isSubmitting()"
  (cancelForm)="closeForm($event)"
  (saveForm)="onSubmit()"
/>
```

- `[isSubmitting]` nhận `boolean` — khi `true` disable nút Save
- Cần disable thêm điều kiện khác (vd. file chưa chọn) → pass computed: `[isSubmitting]="isSubmitting() || !selectedFile()"`
- `[saveLabel]` và `[cancelLabel]` override label nếu cần (default: `common.action.save` / `common.action.cancel`)
- `DialogFooter` đã có trong `FormShareImports` — không cần import thêm
- **KHÔNG có slot cho action button khác Cancel/Save** (vd Delete) — dialog cần nút phụ phải tự build custom footer (ngoại lệ chấp nhận được của rule BẮT BUỘC trên), hoặc mở rộng component `DialogFooter` trước khi dùng chung.
- Nút Save hardcode `severity="success"` (xanh), KHÔNG có Input override — action Save mang tính destructive (vd "Xác nhận huỷ") cần severity khác thì cũng phải tự dựng footer riêng (copy cấu trúc), không chỉ khi cần thêm nút thứ 3.
- `DialogFooter` (`share/components/dialog-footer.ts`) tự nó hardcode `bg-white border-gray-200` (chưa qua token B.18) — dark-mode audit từng form riêng lẻ sẽ KHÔNG bắt được lỗi này vì nó nằm trong component dùng chung, chưa sửa tính đến 2026-08-06.

---

## 5. `BaseDetail` / `BaseReport` / `BasePrint`

→ Xem `skill/skills/building-list-and-export-views/SKILL.md`.

### `BaseDetailPanel<T>` — panel chi tiết dạng SlideOver

- Panel chi tiết mới → `extends BaseDetailPanel<T>` (`share/base/base-detail-panel.ts`): vẽ ngay bằng `seed` rồi gọi API, `loadSeq` chống response về trễ đè bản ghi đang xem, `loadErrorKey` + nút "Thử lại", dọn `model` khi đóng.
- Hook bắt buộc `currentId`/`currentSeed`/`fetchDetail`; tuỳ chọn `ensureCache`/`decorate`/`onLoadStart`. Giữ tên input id riêng (`paymentId`, `appointmentId`) qua `currentId()` để chỗ gọi không phải đổi.
- Trang gọi → `protected readonly detail = detailPanelHost<T>()` (`share/base/detail-panel-host.ts`), bọc `@defer (when detail.visible(); prefetch on idle)`:

```html
<app-payment-detail-panel [(visible)]="detail.visible" [paymentId]="detail.id()" [seed]="detail.seed()" />
```

- `[(visible)]="detail.visible"` — two-way binding vào `WritableSignal` nằm trong object chạy đúng (runtime `twoWayBindingSet`), không cần tách `[visible]` + `(visibleChange)`.

---

## 6. Gotchas — Form & List

→ Xem `frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-gotchas-form.md` (pTemplate → `#ref`, decimal `Number()` cast, `dt.filter`, `apiService.get()` unwrap, `<audio>` src, date filter, nested unix `lines[]`, `TagSeverity`, `ApiService` bỏ qua `observe`).

---

## 7. Import barrels — dùng đúng cái

| Import | Dùng cho | Nặng? |
|---|---|---|
| `ListShareImports` | BaseList, BaseDetail components | ~90 items |
| `PrintShareImport` | BasePrint components | 8 items |
| `FormShareImports` | BaseForm dialogs | vừa |
| `DetailShareImports` | BaseDetail components | vừa |

```typescript
// ✅ Print component — import nhẹ
imports: [...PrintShareImport]

// ✅ List component — import đầy đủ
imports: [...ListShareImports]

// ❌ NEVER — mix sai barrel
imports: [...ListShareImports]  // trong print component — quá nặng
```

---

## AppConfirmDialog — dialog xác nhận chuẩn

→ Xem `frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-confirm-dialog.md` (thay `ConfirmationService` cho modal confirm mới; popover `target:` giữ nguyên).

---

## Unit conversion / LocaleFormatService / ECharts directives

→ Xem `skill/skills/working-on-fe-modules/SKILL.md`

---

## Tab bar tự dựng (không dùng `p-tabs`)

```html
<div [hidden]="activeTab() !== N">
  @defer (when activeTab() === N) { <app-tab-content /> }
</div>
```

- `@defer` chỉ "dựng 1 lần", KHÔNG tự ẩn khi điều kiện sai → để trần thì mọi tab đã mở hiện chồng lên nhau.
- `@if` trần thì ngược lại: huỷ component khi rời tab → gọi lại API, mất vị trí cuộn, mất nội dung đang soạn dở.
- Đổi tab → reset `scrollTop` vùng cuộn (tab dài sang tab ngắn giữ nguyên scroll = nhìn như tab rỗng).
- Tab bar dùng `<button role="tab">`, xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` §B.19.
- Trang khám EMR (`emr/examination/examination.ts`): `reload()` cùng lượt khám chỉ fetch nền — chỉ ĐỔI lượt khám mới bật `loading` dựng lại cây. Tab con KHÔNG được dựa vào remount sau khi lưu để nạp dữ liệu mới; phải phản ứng theo input (vd Kết luận `shouldReloadSameRecord`: version tới mới hơn + không có thay đổi chưa lưu ⇒ `loadExisting`). Guard rời trang đọc `hasPendingChanges()` của 2 form — giá trị do code tự ghi sau khi nạp KHÔNG được bật cờ chưa-lưu.
- `EMR_SPECIALTY_FIELDS` (`define/emr.define.ts`) có khoá TRÙNG giữa chuyên khoa (7↔22: `hba1c`, `blood_glucose_*`; 7↔15: `ef`, `ecg_finding`) ⇒ đổi chuyên khoa A→B chỉ hỏi/xoá khoá thuộc `fields(A) \ fields(B)`, không xoá mọi khoá của A.


Unit conversion, LocaleFormatService, ECharts chart directives.

---

## Unit conversion — FE service chịu trách nhiệm, component không biết

Khi BE nhận unix timestamp nhưng UX dùng đơn vị khác (ngày, giờ), **service** tự convert — component gửi giá trị UX, service convert trước khi POST.

```typescript
// Component gửi số ngày
{ expires_days: 7, quiz_id: 1, customer_id: 5 }

// Service convert trước khi gọi API
createSession(body: any): Observable<any> {
  const { expires_days, ...rest } = body;
  const expires_at = Math.floor(Date.now() / 1000) + (expires_days ?? 7) * 86400;
  return this.apiService.post(`crm/psych-session`, { ...rest, expires_at });
}
```

Áp dụng khi: expires_days → expires_at, giờ → phút, ngày → unix.

---

## LocaleFormatService — dùng trước khi tự viết format ngày/giờ/số

`share/services/locale-format.service.ts` có sẵn `formatDate()`/`formatTime()`/`formatNumber()`
theo `currentLocale()` (company config, KHÔNG hardcode `'vi-VN'`). Check trước khi tự
`toLocaleDateString()`/pad string thủ công trong component.

> **Không còn route "public" (no authGuard) nào trong FE ERP** (đã xóa 2026-07-10, xem lịch sử ERP). Tính năng cho khách/đối tác truy cập
> qua link/QR phải đi qua ZMA (client module BE + PartnerAuth), không thêm route bỏ qua
> `authGuard` trong `app.routes.ts` của ERP FE.

## ECharts directives — dùng sẵn, không tự init echarts

16 directive ở `share/directive/echart/index.ts`. Input shape chính:

| Directive | data | Ghi chú |
|---|---|---|
| `appGradientAreaChart` / `appLineChart` | `[{name, data: number[]}]` + `[categories]` + `[colors]` | multi-series |
| `appBarHorizontalChart` | `number[]` hoặc `[{value, itemStyle:{color}}]` per-bar + `[categories]` | `yAxis inverse` — item đầu ở TRÊN cùng |
| `appDonutChart` / `appPieChart` | `[{value, name, itemStyle?}]` | `name` phải translate sẵn (`TranslateService.instant`) |

Palette 5 màu series: `frontend/skill/skills/using-primeng-components/reference/ui.md`; primary đọc từ CSS var trong `chart-colors.ts`. Skeleton/empty state pattern: `crm/dashboard/component/chart-appointment-status.ts`.

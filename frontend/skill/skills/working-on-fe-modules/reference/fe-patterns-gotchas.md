# Gotchas chung FE

## Mục lục

- LazyImgPipe — list view có nhiều ảnh S3
- Fire-and-forget từ interceptor cần in-flight dedup guard
- Interceptor 404 redirect toàn app — không phải lỗi router
- forkJoin → mergeMap(n) cho multi-file upload
- `canvas.toBlob()` không truyền type → mặc định PNG
- Grep blast radius trước khi đổi giá trị constant dùng chung
- ngx-monaco-editor-v2
- Full-page editor — thoát layout-content padding
- Form control → API check — dùng `switchMap` (tham khảo `frontend/skill/skills/frontend-hard-rules/reference/fe-patterns-rules.md` §1b)
- FormArray rebuild nhiều lần — per-row subscription tích lũy
- Broker display trong edit form — không fallback về affiliate
- Build/Tooling/Git gotchas
- p-table `[loading]` không set → `emptymessage` không bao giờ hiện
- `BaseApp`/`BaseDetailDialog` không có sẵn `destroyRef`
- Tailwind arbitrary-value class động (`w-[{{value}}px]`) không được JIT generate
- Tailwind v3: dấu `!` phải đứng TRƯỚC — `text-xl!` không sinh ra gì
- Popover/dropdown load-lần-đầu — cần loading signal, không để trống thoáng qua
- BE đổi field từ string sang number → audit FE phải quét đủ 5 dạng
- Overlay loading chặn toàn trang cho MỌI request non-GET
- Omnichat có topbar RIÊNG — widget thêm vào `AppTopbar` KHÔNG tới omnichat
- Màu phần tử trên topbar — dùng biến `--topbar-*`, KHÔNG token surface
- Widget nổi `position: fixed` góc phải-dưới va với ô soạn tin omnichat
- Kết luận component "không dùng" — grep class + selector, không chỉ route

---


Các gotcha dễ mắc khi làm FE. Đọc khi gặp triệu chứng liên quan.

---

→ AutoComplete single-select value shape, `searchHandler` override + async `addLabels`, `Subject+switchMap`
search, `clearFilter()` restore defaultSearch, `XxxDisplayModel` typed display fields: đã tách sang
`frontend/skill/skills/building-list-and-export-views/reference/fe-patterns-gotchas-search.md`.

---

## LazyImgPipe — list view có nhiều ảnh S3

Khi list render nhiều note/record có ảnh S3, browser bắn tất cả request cùng lúc → HTTP/1.1 limit 6 conn/domain → một số timeout → "có cái được cái không".

**Fix:** `LazyImgPipe` inject `loading="lazy" decoding="async"` vào tất cả `<img` trong HTML string.

```html
<!-- ✅ List view — dùng lazyImg -->
[innerHTML]="model.note | sanitizeQuill | lazyImg | safeHtml"

<!-- ❌ Print template — KHÔNG dùng lazyImg (lazy loading block ảnh trước khi in) -->
[innerHTML]="model.note | sanitizeQuill | safeHtml"
```

`LazyImgPipe` đã có trong `ListShareImports`. Regex: `/<img\b(?![^>]*\bloading\s*=)/gi` — không inject duplicate nếu đã có `loading`.

---

## Fire-and-forget từ interceptor cần in-flight dedup guard

Method được gọi từ interceptor (per-response) không có guard → burst N concurrent responses → N lần gọi đồng thời.

```typescript
// ❌ NEVER — 5 request về cùng lúc → 5 lần gọi fetchAndSyncMe → 5 GET /api/auth/me
void authService.fetchAndSyncMe();

// ✅ ALWAYS — in-flight guard dedup concurrent calls
private _fetchMeInFlight = false;

async fetchAndSyncMe(): Promise<void> {
  if (this._fetchMeInFlight) return;
  this._fetchMeInFlight = true;
  try {
    // ... fetch logic
  } finally {
    this._fetchMeInFlight = false;
  }
}
```

Áp dụng cho mọi side-effect method fire-and-forget được gọi từ interceptor (per-request/response).

---

## Interceptor 404 redirect toàn app — không phải lỗi router

`response-handler.interceptor.ts` case 404 (`url` khớp regex `/\/(api|crm)\//`) gọi `router.navigate(['/auth/notfound'])` cho **TOÀN APP**, không riêng component gọi API đó. Bất kỳ tab/component con nào fetch lỗi 404 — kể cả fetch optional, kể cả do input tạm thời rỗng lúc component remount (Back/Forward) — sẽ văng cả trang sang `/auth/notfound`, dù route hiện tại vẫn match đúng. Thấy triệu chứng "trang tự nhiên ra 404" mà URL vẫn đúng → tìm request nào (kể cả phụ) đang trả 404, không phải lỗi định tuyến. BE nên tránh trả 404 cho case "input rỗng/không hợp lệ" — dùng 422 (xem `skill/skills/backend-hard-rules/SKILL.md` §Response Format).

---

→ Auth-specific gotchas: `frontend/skill/skills/handling-fe-auth-and-i18n/reference/fe-patterns-gotchas-auth.md`

---

## forkJoin → mergeMap(n) cho multi-file upload

`forkJoin(tasks$)` bắn tất cả upload song song → server overload + browser concurrency limit → một số upload fail.

```typescript
// ❌ forkJoin — n requests cùng lúc
return forkJoin(tasks$).pipe(
  map(() => ({ html: doc.body.innerHTML, failed, total })),
);

// ✅ mergeMap(3) — tối đa 3 request song song
return from(tasks$).pipe(
  mergeMap((task$) => task$, 3),
  toArray(),
  map(() => ({ html: doc.body.innerHTML, failed, total })),
);
```

Import: `mergeMap, toArray` từ `rxjs/operators`; `from` từ `rxjs`. Bỏ `forkJoin`.

---

## `canvas.toBlob()` không truyền type → mặc định PNG

`canvas.toBlob(callback)` không truyền `type` → mặc định encode PNG (không phải JPEG). Luôn truyền tường minh `canvas.toBlob(cb, 'image/jpeg', quality)` khi capture ảnh từ camera/canvas — tránh mismatch giữa bytes thật (PNG) và `File.type`/filename gán thủ công là `image/jpeg`/`.jpg`.

---

## Grep blast radius trước khi đổi giá trị constant dùng chung

Trước khi đổi giá trị 1 constant trong `define/*.ts`, `grep -rln "<TÊN_CONSTANT>" src/app` để biết blast radius — nhiều constant tưởng scope hẹp (vd `CONFIG_FILE_MAX_SIZE` tưởng chỉ 1 form) thực ra dùng chung nhiều module không liên quan.

---

## ngx-monaco-editor-v2

**Setup assets — CRITICAL:**
- Copy `node_modules/monaco-editor/min/vs/` → `src/assets/monaco/min/vs/` (15MB)
- Thêm `src/assets/monaco/` vào `.gitignore`
- `provideMonacoEditor({ baseUrl: 'assets/monaco/min/vs' })` trong `app.config.ts`
- KHÔNG dùng `{ baseUrl: '/assets' }` hay `{ baseUrl: 'assets' }` — Monaco load `${baseUrl}/loader.js`, sai path thì editor không bao giờ khởi tạo (silent fail, không có lỗi console rõ ràng)
- angular.json assets glob cũng được nhưng yêu cầu restart `ng serve`, không tin cậy trong dev

**Height gotcha:** Component có built-in `:host { height: 200px }` — PHẢI override cả `height: 100%` trong inline style, không chỉ `position: absolute; inset: 0`:
```html
<div style="position: relative; flex: 1; min-height: 0;">
  <ngx-monaco-editor
    [options]="editorOptions"
    (onInit)="onEditorInit($event)"
    style="position: absolute; top: 0; left: 0; right: 0; bottom: 0; width: 100%; height: 100%;"
  />
</div>
```

**ControlValueAccessor gotcha:** Component implement CVA + provide `NG_VALUE_ACCESSOR`. Khi đặt trong `<form [formGroup]>` mà không có `formControlName`/`ngModel`, Angular gọi `setDisabledState(true)` → `editor.readOnly = true`. Fix: đặt Monaco NGOÀI thẻ `<form>`. Sync value vào FormControl thủ công:
```typescript
onEditorInit(editor: any) {
  this.monacoEditor = editor;
  editor.setValue(this.htmlContent || '');
  this.monacoDisposable = editor.onDidChangeModelContent(() => {
    const val = editor.getValue();
    this.htmlContent = val;
    this.rfModel?.get('template_html')?.setValue(val, { emitEvent: false });
  });
}
```

**insideNg flag:** Mặc định Monaco chạy `zone.runOutsideAngular()` — phù hợp cho performance. Không cần set `[insideNg]="true"` trừ khi cần Monaco trigger Angular CD trực tiếp.

---

→ Signal/reactivity gotchas (computed(), CVA+OnPush, @Input stale, dataSelected): `frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-gotchas-signals.md`

---

## Full-page editor — thoát layout-content padding

`layout-content` có `padding: 2rem`; `layout-content-wrapper` có `padding-top: 4rem`; topbar `position: fixed; height: 4rem; z-index: 999`.

Để trang chiếm toàn viewport phía dưới topbar, dùng `position: fixed` trên wrapper component:
```html
<div style="position: fixed; top: 4rem; left: 0; right: 0; bottom: 0; z-index: 50; overflow: hidden; background: white;">
  <!-- nội dung full-page -->
</div>
```
- KHÔNG dùng `height: 100dvh` — bị tính từ top viewport, bị padding layout che
- KHÔNG dùng `h-full` trên router component — parent `layout-content` không có explicit height
- `z-index: 50` đủ để phủ breadcrumb/content nhưng dưới topbar (z-index: 999)

## Form control → API check — dùng `switchMap` (tham khảo `frontend/skill/skills/frontend-hard-rules/reference/fe-patterns-rules.md` §1b)

Pattern gốc Subject + switchMap: → Xem `frontend/skill/skills/frontend-hard-rules/reference/fe-patterns-rules.md` §1b "Load-on-change".

Variant khi cần check cả initial value hoặc gộp nhiều control: `merge(of(ctrl.value ?? null), ctrl.valueChanges).pipe(switchMap(v => this.service.check(v)), takeUntilDestroyed(this.destroyRef))` — 2 subscribe riêng (initial + valueChanges) sẽ race, last-resolve-wins. Áp dụng cho `checkTrigger`, `checkAvailable`, `validateField` — bất kỳ call API nào phụ thuộc value của form control.

---


## FormArray rebuild nhiều lần — per-row subscription tích lũy

Khi 1 method (vd `pathDataFields()`) bị gọi lại nhiều lần (mỗi lần `searchHandler`/`loadRecords` chạy) và mỗi lần đều `formArray.clear()` rồi tạo lại control + `valueChanges.subscribe(...).pipe(takeUntilDestroyed(this.destroyRef))` cho từng row → subscription cũ KHÔNG được huỷ (chỉ mất khi component destroy), tích lũy theo số lần load.

```typescript
// ❌ mỗi lần load → thêm N subscription mới, không huỷ subscription lần trước
pathDataFields(fields) {
  this.fieldsCtr.clear();
  fields.forEach((f) => {
    const group = this.fb.group({...});
    group.get('edit')?.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(...);
    this.fieldsCtr.push(group);
  });
}

// ✅ Subject local, next() trước mỗi lần rebuild để huỷ batch subscription trước
private reload$ = new Subject<void>();

pathDataFields(fields) {
  this.reload$.next();
  this.fieldsCtr.clear();
  fields.forEach((f) => {
    const group = this.fb.group({...});
    group.get('edit')?.valueChanges
      .pipe(takeUntil(this.reload$), takeUntilDestroyed(this.destroyRef))
      .subscribe(...);
    this.fieldsCtr.push(group);
  });
}
```

---

## Broker display trong edit form — không fallback về affiliate

- Chỉ load broker từ `customer.affiliate_id` trong CREATE mode (`!this.isEditMode()`).
- Edit mode + `model.broker` undefined (`broker_id = null`) → `brokerSelected = null`, không load affiliate — null nghĩa là staff đã xóa, affiliate fallback sẽ hiển thị sai broker.
- Khi load broker từ affiliate (`brokerService.get(affiliateId)`) và kết quả null (broker đã xóa) → clear cả `brokerSelected = null` và `rfModel.patchValue({ broker_id: null })` để tránh submit orphan `broker_id`.
- ❌ KHÔNG dùng `!this.model()` thay `!this.isEditMode()` — falsy check trên signal có thể thay đổi ngữ nghĩa.

---

## `location.reload()` đúng cho dialog, SAI cho màn hình chặn toàn trang

Sau khi đổi scope (tenant/company) phải reload cứng để dựng lại cache IndexedDB/memory. Nhưng
`location.reload()` chỉ đúng khi component là **dialog nổi trên một trang thật** — reload trả user về
trang đó.

Component **chính là trang chặn** (`/choose-company`, `/2fa-setup`) → `location.reload()` nạp lại đúng
màn hình vừa thoát ra: dữ liệu đã ghi đúng mà user vẫn thấy form, không có đường vào app. Dùng
`window.location.href = '/'` — vừa rời trang chặn, vừa giữ nguyên tác dụng reload cứng.

⚠ Chép đuôi submit từ một component "giống giống" là cách lọt bẫy này: `switch-company` (dialog) và
`choose-company` (trang) chỉ khác nhau ở chỗ đó, và `npx tsc --noEmit` không thấy gì.

## Build/Tooling/Git gotchas

Tách sang `frontend/skill/skills/using-primeng-components/reference/fe-patterns-gotchas-build.md` — tsc vs ng build (AOT), `ng build` ghi đè `public/` git-tracked, `frontend/` có `.git` riêng, `acceptButtonStyleClass` silently no-op.

---

## p-table `[loading]` không set → `emptymessage` không bao giờ hiện

PrimeNG source: `emptymessage` template chỉ render khi `isEmpty() && !loading`.
Nếu component override `searchHandler()` custom (không qua `BaseList.getList()`) mà quên `this.loading.set(true/false)` (chỉ toggle `loadingService` global spinner) → `loading` signal kẹt `true` mặc định — "không có dữ liệu" vĩnh viễn không hiện dù data rỗng thật.
Luôn set cả 2: `this.loading.set(true)` đầu load, `.set(false)` trong `finalize()`.

---

## `BaseApp`/`BaseDetailDialog` không có sẵn `destroyRef`

Chỉ `BaseForm`/`BaseList` inject sẵn `protected destroyRef`. Component extends `BaseApp`/`BaseDetailDialog` trực tiếp (dialog detail, không qua list/form) → PHẢI tự `private destroyRef = inject(DestroyRef);`.

---

## Tailwind arbitrary-value class động (`w-[{{value}}px]`) không được JIT generate

Tailwind JIT quét literal string trong source lúc build — class ghép bằng interpolation Angular (`class="w-[{{colW}}px]"`) không được quét, chỉ "hoạt động" nếu đúng chuỗi đó tình cờ tồn tại literal ở file khác trong repo. Đổi giá trị biến → vỡ layout im lặng, không lỗi biên dịch.
Dùng `[style.width.px]="colW"` thay vì nhét biến vào class Tailwind arbitrary-value.

---

## Tailwind v3: dấu `!` phải đứng TRƯỚC — `text-xl!` không sinh ra gì

Dự án dùng **Tailwind v3.4** → `!` đứng **trước**: `!text-xl`, `!leading-none`. Cú pháp đuôi (`text-xl!`) là của **v4** — ở v3 nó **không sinh class nào**, phần tử im lặng rơi về mặc định. Không lỗi build, không cảnh báo, không hit khi grep tên class.

Đo thật 2026-09-04: `<i class="pi text-xl! leading-none!">` ra **13px** (mặc định `.pi { font-size: 1rem }`) thay vì 16px — mà comment ngay trong file lại tự nhận "icon 20px". Đường lây phổ biến nhất: copy snippet từ tài liệu/mẫu viết cho v4.

Cỡ chữ/icon quan trọng → khai bằng SCSS thay vì utility, khỏi phụ thuộc cú pháp bang.

---

## Popover/dropdown load-lần-đầu — cần loading signal, không để trống thoáng qua

Danh sách async load lúc mở popover/panel lần đầu (macro, tag picker, token registry...) không có
loading signal → khoảng trống giữa lúc mở và lúc data về trông như "không có gì"/lỗi. Pattern chuẩn:
```ts
loading = signal(true);
// ...subscribe: set(false) ở cả next và error
```
```html
@if (loading()) {
  <div class="text-center py-2"><i class="pi pi-spin pi-spinner text-base text-muted"></i></div>
} @else { ... }
```
Icon trần, KHÔNG kèm text (danh sách thường có tiêu đề riêng ở trên). Ref: `omnichat-thread.ts`
(`macrosLoading`), `omnichat-sidebar.ts` (`tagsLoading`), `quick-reply-form.ts` (`tokensLoading`).

---

## BE đổi field từ string sang number → audit FE phải quét đủ 5 dạng

`"0.00"` là **truthy**, `0` là **falsy** — mọi chỗ kiểm truthiness trên field đó đổi hành vi (khối đang luôn
hiện sẽ ẩn khi giá trị 0).

Quét đủ **5 dạng**, thiếu dạng nào là false-negative im lặng:
`@if`/`*ngIf` · `x || 'fallback'` · ternary `x ? a : b` · `x ?? y` · `[hidden]`

- Ca thật đã lọt lưới vì chỉ quét `@if`/`*ngIf`: `{{ x?.unit_price || '—' | currencyFormat }}` → khi thành `0`
  rơi vào `'—'`, pipe trả `'-'` thay vì `0`.
- ⚠ **`x === 0` KHÔNG an toàn** với field đang được đổi kiểu — nó là dạng NGUY HIỂM NHẤT, không phải dạng
  "type-strict nên miễn nhiễm". Trước khi vá `"0.00" === 0` luôn `false` ⇒ nhánh đó **chưa từng chạy**;
  sau khi vá nó mới bắt đầu chạy. Ca thật: cột "Trạng thái thanh toán" ở *Workspace → Doanh số của tôi*
  trước giờ luôn hiện "Thanh toán 1 phần" cho mọi dòng.
- **Quét PHẢI đi cả 2 chiều**: (1) từ FE grep tên cột → tìm consumer; (2) **từ `git diff` backend/ suy ra
  endpoint epic THỰC SỰ vá → truy consumer FE của chúng**. Chỉ chiều (1) sẽ mù với field mà BE đặt alias khác
  tên cột và FE đổi tên lại (`oso_payment` → `payment`).
- ⚠ **Tầng thứ 3 hay bị bỏ sót — FORM cũng là consumer của LIST**: `BaseList.updateRecord` truyền thẳng row
  của list vào form sửa, nên `*/form/*.ts` phải audit y như template list. Bỏ tầng này là bỏ đúng chỗ **hỏng
  DỮ LIỆU** (không phải hiển thị): ca thật `hr/contract/form/contract-form.ts` dùng `x ? Number(x) : null`
  trên `standard_workdays` — khi field thành `0` (falsy) thì giá trị 0 bị **âm thầm ghi đè thành default 26
  lúc lưu**. File này từng bị soi và bỏ qua 2 lần vì lập luận "field vốn đã number".
- Giữ hành vi cũ bằng điều kiện tường minh `!= null`, KHÔNG bỏ trần.
- **Phân biệt false positive theo schema thật, không theo tên field**: `quantity`/`percent_success` thường là
  `int`; `isColVisible('totalRevenue')()` là config cột hiển thị (`BaseList`), không đọc giá trị.
- Field qua `pagination.Paginate` hoặc struct typed `decimal.Decimal` vốn ĐÃ là number từ trước → không phải
  hồi quy, đừng đổi hành vi.

---

→ `@Input` stale khi route-reuse, `dataSelected` reset: `frontend/skill/skills/building-angular-signal-forms/reference/fe-patterns-gotchas-signals.md`

## Overlay loading chặn toàn trang cho MỌI request non-GET

`loading.interceptor.ts` bật overlay chặn tương tác toàn trang cho mọi POST/PUT/PATCH/DELETE — kể cả request chạy ngầm như `PATCH mark-read`.

- Màn dạng realtime/chat (đã có phản hồi nội tuyến riêng: bong bóng optimistic, trạng thái từng tin) → thêm path prefix vào `skipLoadingUrls`.
- GET đi nhánh top progress bar, không chặn tương tác.


## Khung chat: mốc thời gian là MILLISECOND, và phân trang phải đếm ngược từ tin mới nhất

Module omnichat (`modules/omnichat/`):

- `_formatTime(unixMs)` và `new Date(m.sent_at)` coi `last_message_at`/`sent_at` là **mili** — khớp hợp đồng BE. Field trộn đơn vị lọt xuống FE thì nhãn ngày nhảy về 1970 mà không lỗi. Helper chuẩn hoá có sẵn: `omnichat.types.ts:omniToSeconds` (chỉ cho nhóm `waiting_since`/`last_activity_at` vốn là GIÂY).
- `GET /messages` trả trang **MỚI NHẤT** (`offset` đếm ngược, `data` tăng dần, kèm `has_more`). Gọi một lần không phân trang = cắt mất tin mới nhất trên mọi hội thoại dài hơn `count`.
- Offset trang kế lấy từ bộ đếm row-đã-tải-từ-server, **KHÔNG** `messages.length`: mảng hiển thị còn chứa bong bóng optimistic (`tmp-…`) và tin đến ngoài lượt tải → offset vượt số row thật, nhảy cóc mất một khúc lịch sử.
- Prepend tin cũ: chốt `scrollHeight` **trước khi** phát sự kiện tải (lúc dữ liệu về thì DOM đã render, không đọc lại được mốc cũ), bù `scrollTop += scrollHeight - anchor` trong `ngAfterViewChecked`, và **phải có nhánh xoá mốc khi lần tải hỏng** — không thì mốc treo lại chặn vĩnh viễn nhánh bám đáy.

## Omnichat có topbar RIÊNG — widget thêm vào `AppTopbar` KHÔNG tới omnichat

`AppTopbar` được MỌI layout domain include, **trừ** `layout/omnichat/omnichat.layout.ts` — module đó dùng `OmnichatTopbar` riêng.
⇒ Thêm widget vào `AppTopbar` thì nó **không hiện trên omnichat**; muốn hiện phải khai lại trong `layout/omnichat/omnichat-topbar.ts`.
`layout/print/*` cũng không include `AppTopbar` ⇒ widget tự động không hiện khi in.
Kiểm nhanh layout nào không thừa hưởng: `grep -rn 'AppTopbar' src/app/layout/`.

## Màu phần tử trên topbar — dùng biến `--topbar-*`, KHÔNG token surface

- Nền topbar đổi theo theme (`src/assets/layout/layout.css`, 20 theme: xanh/tím chữ trắng hoặc trắng chữ xám) ⇒ token surface (`text-muted-color`, `bg-emphasis`, `text-[var(--p-text-color)]`) chỉ đúng trên nền trắng, trên topbar màu chữ chìm hẳn.
- Chữ/icon: `var(--topbar-item-text-color)`; hover: `var(--topbar-item-hover-bg)`.
- Nút cần nổi bật: cặp màu nhấn `--topbar-menu-button-bg` / `-text-color` / `-hover-bg` (màu nút mở menu) — mọi theme đã chọn để tương phản với nền (vàng trên xanh/tím, primary trên trắng).

## Widget nổi `position: fixed` góc phải-dưới va với ô soạn tin omnichat

Omnichat là màn DUY NHẤT có ô soạn tin ở góc phải-dưới ⇒ mọi widget nổi `fixed` neo góc đó đều va vào nó.
Bố cục 4 cột: `nav | 360px danh sách | khung chat minmax(480px,1fr) | 340px panel khách`. Panel **mở** ⇒ widget chỉ phủ panel khách, vô hại. Panel **thu gọn** (`--panel-w: 0`) ⇒ khung chat nở sát mép phải, widget đè đúng vùng nút gửi/đính kèm.
Cách né: giữ lại dải phải bằng bề rộng widget qua chính `--panel-w` (thừa hưởng transition 0.24s sẵn có) — KHÔNG ẩn widget, KHÔNG `display: none`.
`effectivePanelCollapsed()` tự trả `false` ở tier <992 ⇒ chọn lọc gộp không dính mobile.

## Kết luận component "không dùng" — grep class + selector, không chỉ route

- Component standalone được nhúng qua `imports: [...]` của component khác, không cần route. Trước khi ghi "code chết"/xoá: grep tên class (`AppointmentLineChart`) VÀ selector (`app-appointment-line-chart`) trong `.ts` + `.html`, không chỉ `*.routes.ts`.

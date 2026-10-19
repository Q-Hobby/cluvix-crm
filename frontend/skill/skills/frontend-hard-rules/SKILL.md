---
name: frontend-hard-rules
description: >-
  Rule bắt buộc khi viết/sửa code Angular frontend Cluvix: XSS, OnPush, subscribe
  cleanup, HTTP error/409, token, folder structure, form validation, notification
  helpers. Dùng skill này TRƯỚC KHI chạm bất kỳ file .ts/.html nào trong frontend/,
  kể cả khi chỉ sửa 1 dòng — phần lớn rule ở đây sinh ra từ sự cố thật và đọc code
  không suy ra được.
---

# Frontend hard rules (Angular)

> Ví dụ code ❌/✅ đầy đủ: `reference/examples.md`
> Rules chi tiết B.1–B.17 (bản dài, có context): `reference/fe-patterns-rules.md`
> Data fetching (Observable vs rxResource vs apiResource — khi nào dùng gì): `reference/resource-patterns.md`

## ⛔ FE Hard Rules — LUÔN áp dụng khi làm FE

- **Tạo key i18n mới**: PHẢI `npm run i18n:find -- "<value>"` trước. Có rồi → DÙNG LẠI key (chọn đúng nghĩa EN), KHÔNG tạo trùng. Chưa có → đặt theo taxonomy `common.*` / `{module}.*`, add đủ 4 locale. Chi tiết: `frontend/skill/skills/handling-fe-auth-and-i18n/SKILL.md` (skill `handling-fe-auth-and-i18n`).
- **Footer dialog**: LUÔN dùng `<app-dialog-footer [isSubmitting]="..." (cancelForm)="cancel()" (saveForm)="onSubmit()" />` — KHÔNG tự build footer bằng `p-button` riêng lẻ. `DialogFooter` đã có trong `FormShareImports`. Disable Save bằng `[isSubmitting]="isSubmitting() || !formValid"`. Chi tiết: `prompts/fe-patterns.md#footer-chuẩn` (skill `working-on-fe-modules`).
- **Design UI/UX mới hoặc tra cứu docs PrimeNG**: LUÔN dùng MCP server `primeng` (tool `component_*`/`docs_*`/`migration_*`, xem `/mcp` để list) trước — KHÔNG WebFetch từng trang primeng.dev hoặc đoán API. Cần cho: chọn component, tra props/events, xem code example, hỏi accessibility/animation/theming.



Angular v22+ · PrimeNG v21+ · Tailwind CSS · Signals · Multi-tenant ERP.

## Stack notes
- **Angular v22+**: standalone components, `@for` + `track`, `signal()`, `computed()`, `OnPush` required.
- **`@Service()`** thay `@Injectable({ providedIn: 'root' })` cho service mới — bắt buộc dùng `inject()`, KHÔNG `constructor(private x: X)` (NG2028). Constructor có body logic (không DI param) vẫn OK.
- **PrimeNG v21+**: dùng component mới — `p-datePicker` (thay `p-calendar`), `p-select` (thay `p-dropdown`), v.v. Không dùng tên component cũ.
- **Date range**: dùng 1 `p-datePicker` với `selectionMode="range"` — KHÔNG dùng 2 datepicker riêng lẻ cho from/to.
  ```html
  <p-datePicker formControlName="dateRange" selectionMode="range" [numberOfMonths]="2"
    dateFormat="dd/mm/yy" appendTo="body" [showIcon]="true" />
  ```
  Value là `[Date, Date]`. Khi chuyển sang unix: `dateRange[0]` → from, `dateRange[1]` → to.
- **Tailwind CSS**: dùng utility classes trực tiếp, không custom CSS nếu không cần.

---

## Module-specific deep dives

- `frontend/skill/skills/building-angular-signal-forms/reference/frontend-forms-ux.md` — form validation, PrimeNG imports, BaseReport rfFilter, UX/perceived-performance
- `frontend/skill/skills/building-list-and-export-views/reference/fe-print.md` — print flow (PrintLayout, openPrint, cacheKey, layoutType)
- `skill/skills/handling-fe-auth-and-i18n/SKILL.md` — branchActiveId, authBusinessProfile, RBAC, signals, token storage

---

## Rules

### B.1 XSS prevention (CRITICAL)

```typescript
// ❌ NEVER
return this.sanitized.bypassSecurityTrustHtml(value);

// ✅ ALWAYS
import DOMPurify from 'dompurify';
const clean = DOMPurify.sanitize(value);
return this.sanitized.bypassSecurityTrustHtml(clean);
```

Default: `[innerText]` thay `[innerHTML]` khi không cần markup.

Khi BE tự dựng HTML content (note/content field) cho FE render qua `| safeHtml`: tag không nằm trong `ALLOWED_TAGS` của `SafeHtmlPipe` (`share/pipes/safehtml/safehtml.pipe.ts`) bị DOMPurify strip câm lặng — giữ text, mất định dạng, không lỗi. Check allowlist trước khi BE dùng tag mới.

### B.2 `@for` track (HIGH)

```html
<!-- ❌ NEVER -->
@for (item of items; track $index) { ... }

<!-- ✅ ALWAYS -->
@for (item of items; track item.id) { ... }
```

### B.3 ChangeDetectionStrategy.OnPush (HIGH)

```typescript
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  ...
})
```

- Yêu cầu signal/computed/async pipe
- KHÔNG mutate object trực tiếp — replace reference

### B.4 Subscribe cleanup (HIGH)

```typescript
// ❌ NEVER
this.form.valueChanges.subscribe(v => ...);

// ✅ ALWAYS
private destroyRef = inject(DestroyRef);
this.form.valueChanges
  .pipe(takeUntilDestroyed(this.destroyRef))
  .subscribe(v => ...);

// ✅ Hoặc convert sang signal
formValue = toSignal(this.form.valueChanges);
```

`BaseForm`/`BaseList` đã inject sẵn `protected destroyRef` (`base-form.ts`) — subclass KHÔNG inject lại, dùng `this.destroyRef` trực tiếp (cả cho subscription tạo trong `buildForm()`).

### B.5 HTTP error handling — KHÔNG tự add error handler trong subscribe

BE errors (4xx/5xx) đã được xử lý tập trung ở interceptor (`response-handler.interceptor.ts`) — toast lỗi hiện tự động.

```typescript
// ✅ ĐÚNG — không cần error handler
call.pipe(finalize(...)).subscribe({
  next: (res) => { ... },
});

// ❌ SAI — duplicate toast, interceptor đã handle rồi
call.subscribe({
  next: ...,
  error: () => { this.noti.errorMsg(...); },
});
```

Chỉ thêm `error:` callback khi cần **local recovery logic** (reset state, fallback data) — không phải để show toast.

### B.5b HTTP 409 handling (HIGH)

```typescript
// response-handler.interceptor.ts
case 409: {
  const msg = err.error?.message ?? 'Bản ghi vừa bị sửa, vui lòng tải lại.';
  noti.warning('Xung đột dữ liệu', msg);
  break;
}
```

Form **luôn gửi `version` field** trong update request.

### B.6 Token storage
- access_token: AES-GCM encrypted trong localStorage (`SecureStorageService`)
- refresh_token: HttpOnly Secure SameSite=Lax cookie tại `/api/auth`
- Dùng `tokenService.isRefreshTokenExpired()` để kiểm tra
- ❌ NEVER `tokenService.getRefreshToken()` — đã xóa
- ❌ NEVER log token ra console hoặc đính vào URL

### B.7 i18n
Chi tiết: `frontend/skill/skills/handling-fe-auth-and-i18n/SKILL.md`

- Dùng `labelKey` cho code mới, giữ `label` làm fallback
- p-select: dùng `translateOptions()` util
- Template: `{{ (item.labelKey || item.label) | translate }}`
- Pre-commit hook: `npm run i18n:check` — block nếu thiếu key

### B.8 LocalStorage centralization

```typescript
// ❌ NEVER
localStorage.setItem('/labo/cols', JSON.stringify(cols));

// ✅ Wrapper service với enum key
this.storage.set(LSKey.LaboColumns, cols);
```

**`LayoutService.layoutConfig`** tự persist localStorage qua `effect()` mỗi khi signal đổi.
Cần set menu mode/theme tạm thời theo TRANG (không phải cài đặt user) → dùng `layoutService.menuModeOverride.set(...)`
(không persist, đọc qua `effectiveMenuMode()`), KHÔNG BAO GIỜ mutate `layoutConfig` trực tiếp rồi định "restore" ở `ngOnDestroy`
— nếu destroy không kịp chạy (đóng tab/crash), giá trị tạm bị lưu cứng, ảnh hưởng mặc định toàn bộ trang khác.

### B.9 Tenant scope FE

```typescript
// ❌ NEVER
const services = await db.services.toArray();

// ✅ Scope theo tenant
const services = await db.services
  .where('tenant').equals(currentTenantSlug)
  .toArray();
```

### B.9.1 `branchActiveId` (CRITICAL)

**FE — pre-fill được phép:**
```typescript
// ✅ OK — user vẫn đổi được trước submit
this.initialValue = {
  ...this.initialValue,
  branch_id: branchActiveId(),
};
```

**BE — KHÔNG tự derive từ JWT:**
```go
// ❌ NEVER
params["branch_id"] = middleware.GetCompanyID(c)  // GetCompanyID luôn 0, đã xoá 2026-07-04

// ✅ ALWAYS — 3 nguồn hợp lệ:
// 1. FE truyền lên | 2. Entity liên quan | 3. Validate nil → reject
if p.BranchID == nil {
    return nil, fmt.Errorf("branch_id là bắt buộc")
}
```

### (đã bỏ, D15/CRM standalone) B.9.2 Company filter fallback

Trước D15, mục này mô tả `resolveCompanyFilter`/`resolveCompanyIds` (`company-filter.util.ts`, ~22
component), và quy tắc "master data `company_id` NULL/rỗng = áp dụng chung mọi thương hiệu"
(`companyIdIncludes`, `BaseCacheService.getByCompanyIds`, F25). Verify 2026-09-28: `company-filter.util.ts`,
`resolveCompanyFilter`, `resolveCompanyIds`, `companyIdIncludes` **không còn tồn tại** trong
`frontend/src/app/` — company đã xoá hoàn toàn khỏi FE, không có pattern filter tương đương cho
`branch_id` cần ghi lại ở đây. Giữ làm lịch sử/tham khảo, KHÔNG áp dụng.

### B.10 Signal vs Observable
- State đơn giản → `signal` / `computed`
- Stream/operator (debounce, switchMap) → RxJS, rồi `toSignal()` cho template
- Không dùng BehaviorSubject + getValue() khi signal đủ

### B.11 Build verification

```bash
npm run lint:check
npx tsc --noEmit
npx ngc -p tsconfig.app.json --noEmit 2>&1 | sed 's/\x1b\[[0-9;]*m//g' | grep -cE "error (TS|NG)"   # đếm trước/sau, không tin exit code
```

⚠ **PHẢI khớp CẢ `NG`, không chỉ `TS`** (đo 2026-09-21): lỗi template của `ngc` mang mã **`NG8xxx`**
(`NG8002` binding/property không tồn tại, `NG8001` component chưa import...), KHÔNG phải `TS`. Gate cũ
`grep -c "error TS"` vì vậy trả **0 kể cả khi template hỏng thật** — tức là đúng lớp lỗi mà `tsc` không
bắt được, và cũng là lý do duy nhất người ta chạy `ngc`, thì gate lại bỏ lọt. Tự kiểm được: cấy
`[fooXYZ]="1"` vào một template rồi chạy lại — `grep "error TS"` cho 0 trong khi output thật có
`error NG8002` (nhớ gỡ cờ cấy + `grep` lại để chắc đã gỡ).

⚠ **`ngc` chết vì HẾT BỘ NHỚ cũng cho "0 lỗi"** (đo 2026-09-21, cùng phiên với bẫy `NG` ở trên): trên
cây app hiện tại, `ngc` thường xuyên bị OOM và thoát với **exit 144, KHÔNG in một dòng lỗi nào** ⇒ mọi
cách đếm (`grep -c`) đều ra **0** và trông y hệt build sạch. Luôn chạy kèm
`NODE_OPTIONS=--max-old-space-size=8192` và **kiểm exit code + độ dài output**, đừng chỉ đếm dòng khớp:
```bash
NODE_OPTIONS=--max-old-space-size=8192 npx ngc -p tsconfig.app.json --noEmit 2>&1 \
  | sed 's/\x1b\[[0-9;]*m//g' > /tmp/ngc.log; echo "exit=$?"
grep -cE "error (TS|NG)" /tmp/ngc.log
```
Quy tắc chung của cả 3 bẫy này (ANSI · mã `NG` · OOM): **"= 0" chỉ đáng tin khi chứng minh được lệnh CÓ
chạy tới nơi** — đối chứng bằng một mẫu chắc chắn có hit, hoặc bằng exit code.

⚠ **Chèn text bằng script vào file template: `str.rfind(...)` trả `-1` khi không tìm thấy** — dùng thẳng
`-1` làm vị trí cắt sẽ ghim nội dung vào **cuối file** thay vì báo lỗi. Đã dính thật 2026-09-21: chèn
comment trước thẻ mở, giả định thẻ là `<div`, nhưng 5 form dùng `<p-fluid` ⇒ comment rơi vào giữa
`<app-dialog-footer …/>` ở cuối file ⇒ `NG5002: Opening tag not terminated` ở 5 file. Luôn `assert`
kết quả tìm kiếm `>= 0` trước khi dùng (hoặc thao tác theo DÒNG rồi assert dòng đó mở bằng `<`).

`tsc --noEmit` KHÔNG type-check binding trong `.html` (strictTemplates) — chỉ `ngc`/`ng serve` bắt. Ca thật 2026-09-03: `[companyId]="currentCompanyId()"` (`number | undefined`) bind vào input `number | null` → TS2322 chỉ lộ ở dev server sau 3 pass QA "tsc xanh"; sửa `?? null`. Chi tiết + lỗi nền pre-existing: `skill/skills/using-primeng-components/reference/fe-patterns-gotchas-build.md`.

⛔ **KHÔNG dùng `ng build` để verify template.** `angular.json` trỏ `outputPath` sang `public/` ở **repo
GỐC** — thư mục đó **được git track** (bundle production đã commit). Một lần
`ng build --configuration development` để "kiểm cho chắc" đã đè bản production thành bản dev: **750
file bị xoá + 1289 file mới + 1 sửa = 2040 thay đổi** trong `public/` (đo 2026-09-03), lẫn vào
`git status` của mọi phiên đang chạy và làm hỏng đúng thứ đang được serve. Build xanh nên **không có
dấu hiệu nào báo là vừa gây ra chuyện đó** — chỉ lộ khi mở `git status`.
Muốn type-check template thì dùng `ngc -p tsconfig.app.json --noEmit` ở trên (không ghi output), muốn
thấy UI thật thì `npm start`. `ng build` chỉ chạy khi CHỦ ĐÍCH tạo artifact để deploy.

UI changes: `npm start` + test golden path browser.

### B.12 Async paradigm

```typescript
// ✅ OK — RxJS 7+ accepts async function in switchMap
.pipe(
  switchMap(async (res) => {
    res.items = await this.addLabels(res.items);
    return res;
  }),
)

// ✅ Preferred for clarity
.pipe(
  switchMap((res) => from(this.addLabels(res.items)).pipe(
    map(items => ({ ...res, items })),
  )),
)

// ❌ AVOID — subscribe(async ...) loses error channel
.subscribe(async (res) => { await something(); });

// ✅ subscribe + try/catch nếu cần async
.subscribe(async (res) => {
  try { await something(); } catch (err) { console.error(err); }
});
```

**Rule:** Inside `subscribe()`, async work MUST be wrapped in try/catch.

### B.14 Folder structure (CRITICAL)

```
frontend/src/   ← SOURCE. Mọi thay đổi đều ở đây.
public/         ← BUILD OUTPUT. Không sửa trực tiếp.
```

- Assets: `frontend/src/assets/overrides/` (source) — KHÔNG `public/assets/overrides/`
- Brand colors: `frontend/src/assets/overrides/_brand-colors.scss` — CSS vars `--cluvix-primary`, `--cluvix-accent`, `--cluvix-danger`, `--cluvix-gradient`
- Tailwind: `cluvix-primary`, `cluvix-accent`, `cluvix-danger` (config: `frontend/tailwind.config.js`)
- Sửa màu → chỉ sửa `_brand-colors.scss`
- Service tách ra từ 1 component (co-located cùng folder) mà bị module KHÁC import (cross-module) → chuyển ngay vào `core/services/`, không giữ trong folder component gốc dù có comment giải thích lý do lịch sử — vị trí sai khiến người sau tưởng private-to-component, dễ duplicate logic.
- `layout/` CHỈ chứa chính bản thân khung (sidebar/topbar/breadcrumb/menu/configurator, các file `*.layout.ts`). Widget có business logic — kể cả khi mount TOÀN CỤC từ layout (popup call, dialog xác nhận, dock chat...) — thuộc về `modules/` (vd `modules/utilities/callcenter/`, `modules/teamchat/`), không phải `layout/`, dù nó "không thuộc route nào". Tiền lệ xác nhận: `PopupCall`/`PopupCallMinimize`/`TimezoneDialog`/`UpdateDialog` đều ở `modules/utilities/` dù mount từ `app.layout.ts`/`app.topbar.ts`.

### B.18 Dark-theme-safe — PrimeNG tokens, KHÔNG hardcode màu surface

App có dark mode: `layoutConfig.darkTheme` → class `.app-dark` trên root (PrimeNG `darkModeSelector` config ở `src/app.config.ts`). Component mới KHÔNG hardcode `bg-white` / `text-gray-*` / `border-gray-*` cho surface/text — dùng token tự flip theo theme:

| Hardcode | Token |
|---|---|
| `bg-white` | `bg-[var(--p-content-background)]` |
| `bg-gray-100`, `hover:bg-gray-50` | `bg-[var(--p-content-hover-background)]` |
| `border-gray-100/200` | `border-[var(--p-content-border-color)]` |
| `text-gray-700/800` | `text-[var(--p-text-color)]` |
| `text-gray-400/500/600` | `text-[var(--p-text-muted-color)]` |
| `bg-red-50` (icon chip màu) | alpha hex `bg-[#E539351A]` |
| selected/active tint (tự `color-mix` primary) | `var(--p-highlight-background)` + `var(--p-highlight-color)` — token thật của material-compat base |
| gradient `from-*-50 to-white` | `linear-gradient(135deg, color-mix(in srgb, {color} 8%, var(--p-content-background)), var(--p-content-background))` |

Màu semantic/chart (hex trong `frontend/skill/skills/using-primeng-components/reference/ui.md`) giữ nguyên. Chuẩn tham chiếu: `modules/emr/dashboard/`.

Mặt nền BRAND (`--p-primary-*`) tối ở cả 2 theme → màu chữ trên nó phải là hằng số trắng tự khai, **KHÔNG** dùng `--p-primary-contrast-color`: preset material-compat khai nó là `light-dark(#ffffff, light-dark(#0f172a,#18181b))` nên ở dark ra **#18181B gần đen** (đo được 3.5:1, trượt AA).

**Utility ngắn gọn có sẵn — ưu tiên dùng thay cho arbitrary value dài dòng.** Plugin `tailwindcss-primeui` (đã cài, khai trong `tailwind.config.js`) ship sẵn các class ngữ nghĩa; kiểm chứng trong `node_modules/tailwindcss-primeui/v3/index.js`:

| Viết dài (vẫn đúng) | Utility có sẵn — dùng cái này |
|---|---|
| `text-[var(--p-text-color)]` | `text-color` |
| `text-[var(--p-text-muted-color)]` | `text-muted-color` |
| `border-[var(--p-content-border-color)]` | `border-surface` |
| `bg-[var(--p-content-hover-background)]` | `bg-emphasis` |
| `bg-[var(--p-highlight-background)]` + `text-[var(--p-highlight-color)]` | `bg-highlight` (set cả background lẫn color) |
| `rounded-[var(--p-content-border-radius)]` | `rounded-border` |

`bg-highlight` set CẢ nền lẫn chữ — KHÔNG override `text-*`/`text-primary` lên phần tử mang class này: dark cho **2.6:1** (trượt AA), bỏ override cho **~10:1**. Cần nhấn thêm → đổi `font-weight`, không đổi màu.

⚠ **`text-surface-*` / `bg-surface-*` KHÔNG phải token ngữ nghĩa** — chúng map thẳng tới `--p-surface-N`, tức **thang màu xám thô**, giữ nguyên bậc sáng ở cả 2 theme. Dùng `text-surface-500` cho chữ phụ là *đỡ tệ hơn* `text-gray-500` (vì ít nhất đi qua biến PrimeNG) nhưng vẫn KHÔNG theo theme như `text-muted-color`. Quy ước: vai trò **text/border/surface theo theme** → dùng utility ngữ nghĩa ở bảng trên; chỉ dùng `surface-N` khi thật sự cần một bậc xám cụ thể (vd sọc zebra trong bảng).
Ngoài ra `text-surface-400` (`#9CA3AF`) chỉ đạt **2.54:1** trên nền trắng — trượt AA rõ rệt, KHÔNG dùng cho chữ đọc được (xem bảng contrast trong `frontend/skill/skills/using-primeng-components/reference/ui.md`).

Card đơn (1 form, `max-w-*` giới hạn độ rộng) PHẢI kèm `mx-auto` trên chính div đó (hoặc wrapper `flex justify-center`) — thiếu sẽ lệch trái khi content area rộng hơn `max-w-*`. Mẫu đúng: `attendance-config.html`.

EMR dashboard là chuẩn dark-safe nhưng tile phẳng, dễ bị chê "AI làm cho có". Khi cần độ chi tiết/tinh tế cho
KPI/stat card → theo phong cách CRM dashboard (`crm/dashboard/component/quick-stats.ts`, `payment-in-month.ts`):
icon chip đặc màu riêng mỗi KPI + nền/viền tint theo màu đó + tooltip info + badge delta — nhưng dựng tint bằng
`color-mix()` (không hardcode `bg-orange-50`) để vẫn dark-safe. Dùng helper chung `tint(color,%)`/`shade(color)` từ
`@/share/utils/dashboard-tint.util.ts` (KHÔNG copy `cardBg()` per-module). Ví dụ đã áp: loyalty/sale dashboard, `care/dashboard/component/kpi-cards.ts`.

- Điều kiện hoá màu dark-safe theo signal → `[style.color]`/`[style.background]` ternary, KHÔNG `[class.text-[var(--...)]]` — arbitrary value (chứa `[ ]`) trong `[class.]` KHÔNG áp (build vẫn pass, class im lặng không apply). VD: `[style.color]="cond ? '#F59E0B' : 'var(--p-text-muted-color)'"`.

### B.19 Accessibility & content resilience (Vercel Web Interface Guidelines — lọc theo ERP)

Nguồn: `vercel-labs/web-interface-guidelines`, chỉ giữ rule liên quan ERP nội bộ (bỏ animation/touch/hydration — không áp dụng SSR/React).

- Icon-only button (nhiều trong toolbar bảng dữ liệu) bắt buộc `aria-label`:
```html
<!-- ❌ -->
<button pButton icon="pi pi-trash" (click)="onDelete()"></button>

<!-- ✅ -->
<button pButton icon="pi pi-trash" [attr.aria-label]="'common.delete' | translate" (click)="onDelete()"></button>
```
- Element có `(click)` cho hành động phải là `<button>`/`p-button` — KHÔNG `<div (click)>`, mất keyboard access. `<a>` thiếu `href` cũng vậy (không nằm trong tab order); tab bar tự dựng dùng `<button role="tab">` + `aria-selected` + roving `tabindex`.
- Không override `outline: none` mà không thay bằng `focus-visible` ring — kiểm tra lại khi custom CSS đè theme PrimeNG.
- Text dài (tên KH, ghi chú, địa chỉ) trong bảng/card PHẢI có `truncate`/`line-clamp-*`/`break-words`; flex child chứa text cần `min-w-0` để truncate hoạt động:
```html
<div class="flex items-center gap-2">
  <span class="min-w-0 truncate">{{ customer.name }}</span>
</div>
```
- Input/button trong 1 hàng `@for` push động vào `FormArray` cũng cần `min-w-0` (item co)/`shrink-0` (item cố định) — không chỉ text truncate; thiếu → hàng vừa thêm tràn ra ngoài (browser mặc định `min-width:auto` đè `flex-1`), chỉ tự sửa khi có sự kiện focus/reflow.
- Empty state: field/mảng rỗng phải render placeholder rõ ràng (`—` hoặc "Chưa có dữ liệu"), KHÔNG để trống gây vỡ layout hoặc hiểu nhầm là lỗi.
- Date/số hiển thị qua `Intl.DateTimeFormat`/`Intl.NumberFormat` (hoặc pipe `date`/`number` có sẵn) — KHÔNG tự ghép string format thủ công. Cột số trong bảng báo cáo/dashboard áp `font-variant-numeric: tabular-nums` để căn chữ số.
- Filter/tab/pagination nên phản ánh qua query params khi màn hình cần share link (dashboard EMR, report) thay vì chỉ giữ trong component state. Nhận tab từ URL PHẢI validate theo danh sách tab THỰC SỰ hiển thị (chuyên khoa/permission) — trỏ vào tab ẩn thì panel trắng trơn. Ghi lại URL bằng `Location.replaceState`, KHÔNG `router.navigate(..., {replaceUrl: true})` — navigate chạy trọn chu kỳ điều hướng, làm vùng bảng vẽ lại đè lên popover/overlay đang mở. List có filter panel → dùng `BaseListV2`, xem `skill/skills/building-list-and-export-views/SKILL.md`.
- **Content layout shift (CLS)**: block cảnh báo/banner xuất hiện có điều kiện (validation warning, alert ngoài ngưỡng...) do 1 sự kiện runtime (blur, debounce, response async) bật/tắt qua `@if` sẽ làm content bên dưới nhảy vị trí đột ngột — đặc biệt khó chịu khi user đang thao tác gần đó (đang gõ input kế bên). KHÔNG dùng `@if` trần cho các khối này. Dùng CSS `grid-template-rows` `0fr ↔ 1fr` transition (co giãn đúng theo chiều cao nội dung thật, không cần đoán `max-height` cứng) — element LUÔN mount trong DOM:
```html
<div class="grid transition-[grid-template-rows] duration-200 ease-out"
     [style.grid-template-rows]="showWarning ? '1fr' : '0fr'">
  <div class="overflow-hidden">
    <div class="pb-3"> <!-- spacing nằm TRONG vùng clip, không dùng mb-3 ở ngoài — tránh nhảy phụ theo margin -->
      <!-- nội dung banner -->
    </div>
  </div>
</div>
```
⚠ Khuôn này KHÔNG giãn khi wrapper là flex item của một flex column — đo được height đứng `0px` dù style đã `1fr`. Ca đó dùng biến thể giữ chỗ: luôn render nội dung thật + `[class.invisible]` (chiều cao là của chính nội dung, kể cả khi text xuống 2 dòng ⇒ shift = 0). **Đo lại sau khi dựng, đừng tin khuôn luôn chạy.**

Badge/tag đơn dòng đổi qua lại (có/không có giá trị) dùng cách nhẹ hơn: giữ chỗ bằng placeholder rỗng cùng kích thước (`@if (x) { <p-tag .../> } @else { <span></span> }`) thay vì để `@if` co cụm layout cha lại. Chuẩn tham chiếu: `emr/examination/form/vitals/examination-vitals-form.html` (banner cảnh báo sinh hiệu ngoài ngưỡng + badge per-field).
- **Cảnh báo theo ngưỡng số → đánh giá lúc blur/debounce, KHÔNG theo từng keystroke**: validation/warning tính từ `FormGroup.valueChanges` tức thời (vd cảnh báo BP ngoài ngưỡng) sẽ báo sai khi user gõ dở số (gõ "1"/"12" hướng tới "120" đã bị coi là bất thường). Dùng snapshot riêng (signal ghi thủ công) chỉ refresh qua `(onBlur)` mỗi input hoặc `debounceTime`, không bind computed thẳng vào `valueChanges`. `p-inputnumber` dùng `(onBlur)` (không phải `(blur)`).

#### B.19b Bổ sung WCAG 2.2 + text resilience (nguồn: `nextlevelbuilder/ui-ux-pro-max-skill`, MIT — lọc theo ERP desktop)

Nhập từ bộ 119 UX guideline của repo trên, chỉ giữ phần áp dụng cho web desktop data-dense; bỏ toàn bộ nhóm native (safe area, haptic, bottom nav, gesture) — phần đó nằm ở skill repo `mobile/`.

- **`focus-not-obscured` (WCAG 2.2 AA, 2.4.11)**: thanh sticky/overlay KHÔNG được che element đang giữ focus bàn phím. Rủi ro thật trong repo: `app-dialog-footer` là `sticky bottom-0 z-50` — form dài, tab xuống field gần cuối thì field bị footer che, user gõ mù. Khi dựng dialog form dài: đảm bảo vùng scroll có `scroll-padding-bottom` ≥ chiều cao footer (hoặc `padding-bottom` tương ứng ở cuối content) để field focus luôn tự cuộn lên trên footer.
- **`web-target-size` (WCAG 2.2 AA, 2.5.8) — 24×24 CSS px, KHÔNG phải 44pt**: 44×44 là chuẩn *mobile native* (Apple HIG), đừng bê nguyên sang web rồi kết luận toolbar icon của ERP "vi phạm". Ngưỡng web là 24×24 CSS px cho vùng bấm (hoặc có khoảng cách bù đủ). Icon-only button trong toolbar bảng phải đạt mức này — icon `pi` 16px trần không padding là KHÔNG đạt. Ngưỡng 24px là **CSS px tuyệt đối** — khai vùng bấm bằng `rem` là SAI: root font-size của app fluid (`custom.scss`: 9px <768 · 10px 768 · 12px 992 · 13px 1440 · 14px ≥1920) nên `1.5rem` chỉ ra 13.5px ở khung 375px. Đã đo: cùng một nút cho 32.5px ở viewport 1863 nhưng 22.5px ở 375. Dùng `min-width: max(<rem>, 24px)`.
- **`long-token-wrapping`**: URL / mã / ID / nội dung user nhập phải cho xuống dòng bằng `overflow-wrap: anywhere` (Tailwind `break-anywhere`, hoặc `break-all` chỉ cho token) + text child co được (`min-w-0`). KHÔNG áp `break-all` cho văn xuôi thường (cắt giữa từ tiếng Việt, đọc rất khó). Ô hiển thị URL chỉ-đọc: hoặc cho wrap, hoặc `truncate` + tooltip đầy đủ — không để tràn ngang.
- **`truncation-strategy`**: ưu tiên wrap hơn truncate; khi buộc truncate PHẢI cấp được nội dung đầy đủ qua `pTooltip` (hoặc expand). Truncate câm — không tooltip, không cách nào xem full — là mất dữ liệu về mặt trải nghiệm.
- **Khoá cả vùng form (record/phiếu đã chốt) → `[attr.inert]`, KHÔNG `pointer-events-none`** (sửa 2026-09-21, 11 form giấy tờ EMR): `pointer-events: none` chỉ chặn CHUỘT — Tab vào vẫn focus và vẫn gõ được, trong khi nút Lưu thường đã bị ẩn theo cùng cờ khoá ⇒ user gõ xong **mất trắng, không một thông báo**. `[attr.aria-disabled]` KHÔNG cứu được: nó là nhãn ARIA, không có hành vi. Dùng `[attr.inert]="locked ? '' : null"` trên chính div bao (gỡ khỏi tab order + chặn chuột + ẩn với screen reader, Chrome 102+/Safari 15.5+). Khuôn giữ-chỗ chống CLS cũng vậy: khối đang thu `0fr` vẫn nằm trong DOM ⇒ phải `inert`, nếu không tab lọt vào nút vô hình.
- **KHÔNG `opacity-70` lên vùng chỉ-đọc**: opacity nhân vào MÀU CHỮ nên kéo tương phản xuống dưới ngưỡng — đo trên nền trắng: muted `#6B7280` 4.83 → **2.74:1**, body `#4B5563` 7.56 → **3.58:1** (đều trượt AA), mà record đã khoá chủ yếu là để ĐỌC. Tín hiệu "đã khoá" dùng thẻ trạng thái + banner + ẩn nút Lưu; cần sắc thái thì đổi NỀN (`bg-emphasis`), không giảm opacity chữ.
- **`read-only-distinction`**: read-only ≠ disabled, phải khác nhau cả thị giác lẫn ngữ nghĩa. Giá trị chỉ-đọc để user copy (URL webhook/callback, mã hệ thống) KHÔNG dùng `<input readonly>` — input có con trỏ + viền focus khiến user tưởng gõ được. Dùng khối hiển thị: label → dòng giải thích → `<code>` box (`bg-[var(--p-content-background)]` + `border-[var(--p-content-border-color)]`) + icon copy. Mẫu: `config/config-omnichannel/omni-channel-settings/dialog/livechat-connect-dialog.html`, `config/config-zalo/zalo-oa-config/dialog/zalo-oa-connect-dialog.html`.
- **`chip-collection-reflow`**: cụm chip/tag/filter phải WRAP trước khi nghĩ đến thu nhỏ chữ. Cần gộp thì `+n` phải là control bấm được (mở popover xem đủ), KHÔNG phải text chết giấu luôn giá trị.
- **`aria-sort`**: cột `p-table` cho sort phải phát `aria-sort` (`ascending`/`descending`/`none`) đúng trạng thái hiện tại — screen reader không có cách nào khác biết bảng đang sort theo cột nào.
- **`toast-accessibility`**: toast KHÔNG được cướp focus; thông báo qua `aria-live="polite"` (lỗi dùng `role="alert"`). Interceptor toast hiện tại tự bắn — khi thêm toast tuỳ biến, giữ đúng nguyên tắc này.
- **`prefers-reduced-motion`**: mọi transition đã dựng (grid-template-rows CLS, dialog enter/leave, skeleton shimmer) phải tắt/giảm khi user bật giảm chuyển động:
```css
@media (prefers-reduced-motion: reduce) {
  * { animation-duration: 0.01ms !important; transition-duration: 0.01ms !important; }
}
```
Đặt 1 lần ở global style, không rải từng component.
- **`error-summary` + focus management**: form dài submit lỗi NHIỀU field → ngoài việc focus field đầu tiên (đã có ở `frontend/skill/skills/building-angular-signal-forms/reference/frontend-forms-ux.md`), nên có khối tóm tắt lỗi đầu form, mỗi mục link tới field tương ứng, và focus vào khối đó sau submit. Vẫn GIỮ nguyên message lỗi inline dưới từng field — summary là bổ sung, không thay thế.
- **`entry-point-autofocus`**: trang/dialog có DUY NHẤT một input là "điểm bắt đầu" tự nhiên (ô search chính của trang search-heavy, hoặc field đầu tiên user luôn gõ trước của form tạo mới) → tự focus lúc mount, không bắt user trỏ chuột vào trước. Pattern: `ngAfterViewInit()` + `viewChild<ElementRef<HTMLInputElement>>('x')` + `queueMicrotask(() => el()?.nativeElement?.focus())` (focus() gọi thẳng trong `ngAfterViewInit` không đợi DOM paint xong, dễ bị bỏ qua). KHÔNG áp tràn lan cho mọi input — form nhiều field ngang hàng không field nào là "điểm vào" rõ ràng, hoặc dialog confirm/table selection thì không cần; trang hay dùng trên mobile nên cân nhắc lại (autofocus bật bàn phím ảo ngay khi mở trang). Mẫu: `modules/utilities/global-search/global-search.ts`, `modules/config/config/config.ts`.

---

## Quick checklist trước commit

- [ ] Component mới có `OnPush`?
- [ ] `@for` dùng `track item.id`?
- [ ] Long-lived subscribe có `takeUntilDestroyed()`?
- [ ] `[innerHTML]` đã sanitize qua DOMPurify?
- [ ] Form update gửi `version`?
- [ ] Form validation dùng `isFieldInvalid()` — không thêm gate `touched`?
- [ ] String hiển thị qua `| translate`?
- [ ] List >100 item virtual scroll?
- [ ] Search input có `debounceTime(300)`?
- [ ] `localStorage` qua wrapper service?
- [ ] `extendedDiagnostics` config trong `tsconfig.json → angularCompilerOptions` — KHÔNG phải `angular.json` (schema error)?
- [ ] Lint + tsc clean?
- [ ] Mỗi dòng diff trace về yêu cầu user?
- [ ] Loading state mới có debounce-show + min-visible-duration?
- [ ] Show/hide state có CSS transition, không snap cứng?
- [ ] Submit button có feedback cục bộ (spinner riêng), không chỉ dựa global overlay?
- [ ] List/detail load lần đầu dùng skeleton, không đẩy layout?
- [ ] Icon-only button có `aria-label`? Vùng bấm đạt ≥24×24 CSS px?
- [ ] Text dài trong bảng/card có `truncate`/`min-w-0`? Truncate có kèm tooltip xem full?
- [ ] URL/mã dài có `overflow-wrap: anywhere` (không `break-all` cho văn xuôi)?
- [ ] Giá trị chỉ-đọc-để-copy dùng khối `<code>` + icon copy, KHÔNG `<input readonly>`?
- [ ] Dialog form dài: field focus có bị `app-dialog-footer` sticky che không?
- [ ] Trang/dialog có 1 input "điểm bắt đầu" rõ ràng → đã autofocus lúc mount (ngAfterViewInit + queueMicrotask)?
- [ ] Cột bảng cho sort có phát `aria-sort`?
- [ ] Route mới copy từ file khác — import component có trỏ đúng module của chính nó không? (đã lặp bug ở HR + Care: `dashboard` route trỏ nhầm sang `emr/dashboard`)
- [ ] Route mới/đổi route → `npm run breadcrumb:audit` xanh + đúng quy ước C1–C6 ở `frontend/skill/skills/working-on-fe-modules/reference/fe-breadcrumb.md`

---
## Tham chiếu thêm
- **Examples với code thực tế**: `skill/skills/frontend-hard-rules/reference/examples.md`
- **i18n chi tiết**: `frontend/skill/skills/handling-fe-auth-and-i18n/SKILL.md`
- **Form + UX/perceived-performance rules (B.13, B.15–B.17)**: `frontend/skill/skills/building-angular-signal-forms/reference/frontend-forms-ux.md`


## Cấu trúc
`core/` (models, services) → `modules/{module}/` (lazy-loaded) → `share/` (base classes, components, helpers, pipes). `define/` constants. `indexeddb/` Dexie sync.

## Aliases
`@/*` → `src/app/*`, `@env/*` → `src/environments/*`

## Patterns
- **KHÔNG dùng magic number/string** — mọi constant phải define trong `define/`, import khi dùng
- Standalone components only, KHÔNG NgModules
- **BaseForm**: kế thừa cho form dialog, `buildForm()`, import `FormShareImports`
- **BaseList**: kế thừa cho list, import `ListShareImports`
- Signals (`signal()`, `computed()`) cho state, `branchActiveId()` cho branch hiện tại
- Form dialog mở qua `dialogWrapperService.open()`
- API: Observable + RxJS (switchMap, catchError, finalize)
- PrimeNG 21 + Tailwind 3, i18n: ngx-translate

## Notification (i18n)
- **KHÔNG hardcode title/message** trong `noti.success/error/warn`
- `noti.successRes(res)` — khi có response từ BE, tự lấy `res.message`
- `noti.successMsg('...')` — khi có message sẵn, tự dùng i18n title
- `noti.warnMsg('...')` — warning với i18n title
- `noti.errorMsg('...')` — error với i18n title
- `noti.infoMsg('...')` — info với i18n title
- `noti.success(translate.instant('KEY'), detail)` — khi cần title custom
- KHÔNG dùng `noti.success('Thông báo', ...)` trực tiếp
- ⚠️ `noti.success(x)`/`noti.error(x)` gọi RAW 1 arg → `x` thành TITLE, thân toast RỖNG (không lỗi, chỉ sai UI) — luôn dùng `successMsg`/`errorMsg`/`successRes` cho message, KHÔNG gọi `.success()`/`.error()` trực tiếp trừ khi cố ý set title riêng (2 arg)

## Naming
File: `kebab-case.ts`. Class: `PascalCase`. Service: `{resource}.service.ts`. Model: `{resource}.model.ts`.

# Frontend code examples

## Mục lục

- 1. Think Before Coding
  - Example 1.1 — "List khách hàng load chậm"
  - Example 1.2 — "Thêm modal export khách hàng"
- 2. Simplicity First
  - Example 2.1 — Form factory anti-pattern
  - Example 2.2 — Service wrapper cho 1 HTTP call
  - Example 2.3 — BehaviorSubject khi signal đủ
- 3. Surgical Changes
  - Example 3.1 — Fix XSS đừng đụng type/structure
  - Example 3.2 — Thêm 409 case đừng đụng cases khác
  - Example 3.3 — Fix một `@for track $index` không phải tất cả 171 chỗ
- 4. Goal-Driven Execution
  - Example 4.1 — Multi-step: Implement HTTP 409 modal flow
  - Example 4.2 — Reproduce trước khi fix: subscribe leak
  - Example 4.3 — Convert component sang OnPush + signal
- Anti-patterns Summary (Cluvix FE thường gặp)
- Key Insight
- Tham chiếu trong codebase

---


Code examples từ codebase ERP Cluvix. Stack: Angular 22 + PrimeNG 21 + Tailwind 3 + RxJS 7.8 + ngx-translate. Mỗi example: lỗi LLM thường mắc + cách đúng.

Conventions chung (cấu trúc folder, aliases, notification helpers, naming): → Xem `skill/skills/frontend-hard-rules/SKILL.md`.

---

## 1. Think Before Coding

### Example 1.1 — "List khách hàng load chậm"

**❌** Fix mọi layer cùng lúc (lru-cache + virtual scroll + memoize + lazy image + debounce — 600 dòng/12 file) mà không đo bottleneck.

**✅** Hỏi user data đo trước, xác định layer rồi fix đúng chỗ:
1. **Network** — Network tab: `/api/customer` > 500ms → vấn đề BE (index, N+1, cache).
2. **Bundle** — DevTools filter `.js`: size chunk module, lazy load đúng chưa (`ng build --stats-json`).
3. **Render** — Performance tab: component có OnPush? `@for` track `item.id`? 1000+ row thiếu virtual scroll?

---

### Example 1.2 — "Thêm modal export khách hàng"

**❌** Assume FE-only: `this.api.get('/customer/export-all').subscribe(blob => saveAs(...))` — không hỏi scope/PII/volume, không loading state, hardcode filename.

**✅** Làm rõ trước khi code:
1. **Scope**: toàn tenant hay theo filter đang apply?
2. **Field**: có PII (phone, CCCD)? Field tiền cần quyền nào?
3. **Volume**: 50K row → sync export block UI 30s, hay background job + email link (cần BE support)?
4. **Format**: `.xlsx` (sheetjs ~600KB) hay `.csv`?

→ Đề xuất MVP (scope theo filter, loading state, disable button) + hỏi user chốt hướng.

---

## 2. Simplicity First

### Example 2.1 — Form factory anti-pattern

**User request:** "Form sửa khách hàng có 5 field"

**❌ Bloated**

```typescript
// 4 file, 250 dòng cho form 5 field
abstract class BaseFormBuilder<T> {
  abstract getFields(): FormFieldConfig<T>[];
  build(model: T): FormGroup { /* reflection + decorator metadata */ }
}

interface FormFieldConfig<T> {
  key: keyof T;
  label: string;
  validators: ValidatorFn[];
  asyncValidators?: AsyncValidatorFn[];
  ui: 'input' | 'select' | 'textarea';
  options?: () => Observable<{ value: any; label: string }[]>;
}

class CustomerFormBuilder extends BaseFormBuilder<Customer> {
  getFields() {
    return [
      { key: 'name', label: 'Tên', validators: [Validators.required], ui: 'input' },
      { key: 'phone', label: 'SĐT', validators: [Validators.required, phoneValidator], ui: 'input' },
      // ... 80 dòng config
    ];
  }
}

// Component
form = this.builder.build(this.customer);
```

**✅ Reactive form thuần**

```typescript
form = this.fb.group({
  name: ['', Validators.required],
  phone: ['', [Validators.required, Validators.pattern(/^\d{9,11}$/)]],
  email: ['', Validators.email],
  address: [''],
  version: [0],
});

ngOnInit() {
  if (this.customer) this.form.patchValue(this.customer);
}
```

13 dòng. Khi có 10+ form share pattern → cân nhắc abstraction. 1 form không cần.

---

### Example 2.2 — Service wrapper cho 1 HTTP call

**❌ Over-abstracted**

```typescript
// services/customer/customer-list.service.ts
@Injectable({ providedIn: 'root' })
export class CustomerListService {
  private http = inject(HttpClient);
  private cache$ = new BehaviorSubject<Customer[]>([]);

  get customers$() { return this.cache$.asObservable(); }
  get customers() { return this.cache$.getValue(); }

  load(filter?: CustomerFilter): Observable<Customer[]> {
    return this.http.get<Customer[]>('/api/customer', { params: filter as any })
      .pipe(tap(data => this.cache$.next(data)));
  }

  reset() { this.cache$.next([]); }
}

// Component
ngOnInit() {
  this.customerListService.load(this.filter).subscribe();
  this.customerListService.customers$.subscribe(data => this.list.set(data));
}
```

3 layers indirection cho 1 GET request. Cache không có invalidate strategy. Subscribe leak.

**✅ Đơn giản — subscribe trực tiếp, hoặc `apiResource()`/`rxResource()` (KHÔNG `httpResource()` raw — xem `frontend/skill/skills/frontend-hard-rules/reference/resource-patterns.md`)**

```typescript
// Component
private http = inject(HttpClient);
private destroyRef = inject(DestroyRef);

list = signal<Customer[]>([]);
loading = signal(false);

load() {
  this.loading.set(true);
  this.http.get<Customer[]>('/api/customer', { params: this.filter() })
    .pipe(takeUntilDestroyed(this.destroyRef))
    .subscribe({
      next: data => this.list.set(data),
      complete: () => this.loading.set(false),
    });
}
```

Khi có 5+ component cùng cần list customer + cache → tạo service. Không trước.

---

### Example 2.3 — BehaviorSubject khi signal đủ

**❌ Cũ kỹ**

```typescript
private currentUserSubject = new BehaviorSubject<User | null>(null);
currentUser$ = this.currentUserSubject.asObservable();

setUser(user: User) { this.currentUserSubject.next(user); }
get user() { return this.currentUserSubject.getValue(); }
isLoggedIn$ = this.currentUser$.pipe(map(u => !!u));
```

**✅ Signal-first (Angular 17+)**

```typescript
currentUser = signal<User | null>(null);
isLoggedIn = computed(() => this.currentUser() !== null);

setUser(user: User) { this.currentUser.set(user); }
```

Ngắn hơn, type-safer, sync access (không cần subscribe), tự trigger OnPush re-render.

---

## 3. Surgical Changes

### Example 3.1 — Fix XSS đừng đụng type/structure

**User request:** "SafeHtmlPipe có lỗ hổng XSS, sửa giúp dùng DOMPurify"

**❌ Drive-by**

```diff
- import { inject, Pipe, PipeTransform } from '@angular/core';
- import { DomSanitizer } from '@angular/platform-browser';
-
- @Pipe({
-   name: 'safeHtml',
-   standalone: true,
- })
- export class SafeHtmlPipe implements PipeTransform {
-   private sanitized = inject(DomSanitizer);
-
-   transform(value: any) {
-     return this.sanitized.bypassSecurityTrustHtml(value);
-   }
- }
+ /**
+  * SafeHtmlPipe
+  *
+  * Sanitizes HTML using DOMPurify before bypassing Angular's sanitizer.
+  *
+  * @example
+  * <div [innerHTML]="content | safeHtml"></div>
+  */
+ import DOMPurify from 'dompurify';
+ import { inject, Pipe, PipeTransform } from '@angular/core';
+ import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
+
+ const ALLOWED_TAGS = ['p','br','strong','em','u','ol','ul','li','a','h1','h2','h3','h4','h5','h6','blockquote','code','pre','span','div','img','table','thead','tbody','tr','td','th'];
+ const ALLOWED_ATTR = ['href','target','rel','src','alt','class','style'];
+
+ @Pipe({
+   name: 'safeHtml',
+   standalone: true,
+   pure: true,
+ })
+ export class SafeHtmlPipe implements PipeTransform {
+   private readonly sanitizer = inject(DomSanitizer);
+
+   transform(value: string | null | undefined): SafeHtml {
+     if (value == null || value === '') return '';
+     const clean = DOMPurify.sanitize(value, {
+       ALLOWED_TAGS, ALLOWED_ATTR,
+     });
+     return this.sanitizer.bypassSecurityTrustHtml(clean);
+   }
+ }
```

**Vấn đề:** Thêm doc, đổi tên var (`sanitized` → `sanitizer`), thêm `pure`, null guard, type `any` → `string`. Mỗi cái có lý do riêng nhưng KHÔNG phải yêu cầu — review khó tách "sửa XSS" và "improve types/docs".

**✅ Surgical**

```diff
+ import DOMPurify from 'dompurify';
  import { inject, Pipe, PipeTransform } from '@angular/core';
  import { DomSanitizer } from '@angular/platform-browser';

  @Pipe({
    name: 'safeHtml',
    standalone: true,
  })
  export class SafeHtmlPipe implements PipeTransform {
    private sanitized = inject(DomSanitizer);

    transform(value: any) {
-     return this.sanitized.bypassSecurityTrustHtml(value);
+     const clean = DOMPurify.sanitize(value);
+     return this.sanitized.bypassSecurityTrustHtml(clean);
    }
  }
```

2 dòng + 1 import. `any` xấu, null guard có ích. Tách ra.

---

### Example 3.2 — Thêm 409 case đừng đụng cases khác

**User request:** "Thêm `case 409` vào response handler interceptor cho optimistic lock"

**❌ Drive-by:** nhân tiện sửa luôn case 422 (thêm helper `parseValidationMessage`, đổi title) và thêm case 500 chưa được yêu cầu — review không tách được "thêm 409" khỏi "improve 422/500".

**✅ Surgical:** chỉ thêm đúng case 409 — snippet chuẩn: → Xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.5b.

422 cải thiện (parseValidationMessage) — mở issue khác. 500 fallback — đã có default branch xử lý.

---

### Example 3.3 — Fix một `@for track $index` không phải tất cả 171 chỗ

**User request:** "Fix track $index ở danh sách khách hàng"

**❌ "Helpful" làm hết**

```
[Agent quét toàn bộ codebase, fix 171 chỗ track $index trong 1 PR]
PR diff: 171 file, 342 line changed.
Reviewer: không thể review từng case có dùng id ổn định không.
```

**✅ Surgical**

```diff
-   @for (cust of customers(); track $index) {
+   @for (cust of customers(); track cust.id) {
      <app-customer-card [data]="cust" />
    }
```

Đúng 1 file. Mention trong PR description: "Còn ~170 chỗ tương tự, mở issue tracking riêng để fix incremental."

171 chỗ trong 1 PR = không review được. Mỗi PR fix 5-10 chỗ liên quan đến module đang làm.

---

## 4. Goal-Driven Execution

### Example 4.1 — Multi-step: Implement HTTP 409 modal flow

**User request:** "Khi BE trả 409, hiện modal 'Tải lại' và refresh form"

**❌ Vague**

```
"Em sẽ thêm xử lý 409, hiện modal, reload form. Done."
[Code 4 file rời rạc, không verify từng bước, không biết chỗ nào fail]
```

**✅ Plan với verify**

```
Plan 4 bước:

1. Thêm `case 409` vào `core/interceptors/response-handler.interceptor.ts`
   Verify:
   - DevTools → trigger PUT với version cũ
   - Response 409 không còn fallback "Unhandled error"
   - Toast warning hiện với message từ BE

2. Tạo `EventBus` event 'version-conflict' (hoặc dùng EventBus có sẵn)
   Verify:
   - Console log event payload có {url, recordId} đúng
   - Form chưa subscribe → không hành vi gì (OK)

3. Form component subscribe event, popup ConfirmDialog "Tải lại?"
   Verify:
   - Mở 2 tab cùng record. Sửa tab 1, sửa tab 2 → tab 2 popup
   - Bấm "Tải lại" → form refresh từ API, version cập nhật
   - Bấm "Hủy" → form giữ nguyên, user tự quyết

4. Merge dirty fields sau reload (optional)
   Verify:
   - User đang gõ field X (chưa submit). 409 → reload.
   - Field X giữ giá trị user nhập (mark dirty), các field khác fresh từ DB.

Mỗi bước test độc lập trước khi sang bước tiếp.
OK plan này không?
```

---

### Example 4.2 — Reproduce trước khi fix: subscribe leak

**User request:** "Component CustomerList rò rỉ memory, fix"

**❌ Fix mò**

```typescript
// Đoán random — thêm takeUntilDestroyed cho mọi subscribe
// Không biết subscribe nào leak, fix bừa, có thể không đụng vào subscribe đang leak thật
```

**✅ Reproduce → fix → verify**

```typescript
// Bước 1: Reproduce — DevTools Memory tab, take snapshot trước/sau
// 1. Mở Performance Monitor (Cmd+Shift+P > "Show Performance Monitor")
// 2. Vào /customer, vào component khác, vào lại /customer, lặp 10 lần
// 3. Theo dõi "JS heap size" tăng dần → confirm leak

// Bước 2: Detached element check
// DevTools Memory → Heap snapshot → filter "Detached"
// Tìm Detached HTMLDivElement có class .customer-list
// → Subscriber còn giữ reference

// Bước 3: Tìm subscribe không cleanup
// Code:
ngOnInit() {
  this.api.search$.subscribe(data => this.list.set(data)); // ← long-lived?
  this.socket.on('customer-update').subscribe(updateCust); // ← chắc chắn long-lived
  this.form.valueChanges.subscribe(/* ... */);             // ← chắc chắn long-lived
  this.api.getList().subscribe(/* ... */);                 // ← HTTP one-shot, OK
}

// Bước 4: Fix — chỉ wrap long-lived
ngOnInit() {
  const dr = this.destroyRef;
  this.api.search$.pipe(takeUntilDestroyed(dr)).subscribe(...);
  this.socket.on('customer-update').pipe(takeUntilDestroyed(dr)).subscribe(...);
  this.form.valueChanges.pipe(takeUntilDestroyed(dr)).subscribe(...);
  this.api.getList().subscribe(...); // giữ nguyên — HTTP tự complete
}

// Bước 5: Verify — chạy lại reproduce
// Heap stable, không Detached element nữa.
```

Goal-driven: leak đã reproduce + measure → fix có verifiable success.

---

### Example 4.3 — Convert component sang OnPush + signal

**User request:** "Convert CustomerListComponent sang OnPush + signal"

**✅ Plan với verify**

```
Goal: component giữ nguyên hành vi user-facing, internal đổi sang signal-based.
Verify cả trước và sau: render output, click handlers, search behavior identical.

Plan 5 bước:

1. Snapshot baseline
   Verify: chụp screenshot UI, click qua mọi action (search, filter, paginate,
   click row). Note xuống behavior hiện tại.

2. Convert state: `customers: Customer[]` → `customers = signal<Customer[]>([])`
   Update tất cả `this.customers = data` → `this.customers.set(data)`
   Update template `customers` → `customers()`
   Verify: `npx tsc --noEmit` clean. UI render giống screenshot.

3. Convert derived state: `get filteredCount()` → `computed(() => ...)`
   Verify: số filter count cập nhật đúng khi search.

4. Bật `ChangeDetectionStrategy.OnPush`
   Verify: click qua mọi action lại — không có stale UI.
   Đặc biệt: socket.io event update — phải re-render (signal → auto OK,
   nhưng async pipe / direct subscribe + assign cần markForCheck).

5. Cleanup: xóa BehaviorSubject/Observable không còn dùng
   Verify: `npm run lint:check` clean. Bundle size không tăng.

Mỗi bước có thể commit riêng + revert nếu hỏng.
OK plan này không?
```

---

## Anti-patterns Summary (Cluvix FE thường gặp)

| Nguyên tắc | Anti-pattern | Fix |
|-----------|-------------|-----|
| Think Before Coding | "Load chậm" → fix mọi layer cùng lúc | Đo trước, xác định layer (network/bundle/render), fix đúng chỗ |
| Simplicity First | BehaviorSubject + getValue + asObservable cho state đơn giản | `signal()` + `computed()` |
| Surgical Changes | Fix XSS pipe → reformat type, thêm doc, null guard | Đúng 2 dòng + DOMPurify import |
| Goal-Driven | "Memory leak" → wrap mọi subscribe bằng takeUntilDestroyed | Reproduce với Heap snapshot, fix subscribe leak thật, verify lại |

---

## Key Insight

Code "overcomplicated" trên FE thường xuất hiện ở:
- Service wrapper cho 1 HTTP call
- Form factory cho 1 form
- Custom directive làm việc Angular đã có
- BehaviorSubject + observable + getValue khi `signal` đủ

**Code tốt = signal cho state, RxJS cho stream, Angular Reactive Form thuần.**

Khi nào abstraction xứng đáng? Khi có **3+ chỗ thật sự cần share** code/pattern. Không trước.

---

## Tham chiếu trong codebase

- Interceptors: `src/app/core/interceptors/` (auth, dedupe, response-handler, request-length, loading)
- Auth flow: `src/app/core/services/auth/token.service.ts`
- SafeHtml pipe (đã sanitize DOMPurify): `src/app/share/pipes/safehtml/safehtml.pipe.ts`
- Signal patterns mẫu: tìm component có `signal()` + `computed()` đầy đủ, dùng làm template
- Header sync: `src/app/core/services/header-sync.service.ts`
- IndexedDB cache: `src/app/indexeddb/`

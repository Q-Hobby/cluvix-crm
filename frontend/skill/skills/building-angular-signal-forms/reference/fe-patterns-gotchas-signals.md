# Gotchas Signals & CD

## Mục lục

- Custom ControlValueAccessor + OnPush: writeValue() thiếu markForCheck()
- OnPush + `ngOnInit` async → rfModel set muộn, form không bao giờ render
- `@Input` chỉ set trong `ngOnInit` bị stale khi route-reuse
- `<ng-template #caption>`/`#header` trong `p-table` — embedded view CD riêng của p-table
- `@Input()` khai báo nhưng cha không bind — thực chất là state nội bộ, không phải Input thật
- `dataSelected` (checkbox chọn nhiều trong BaseList) phải reset trong `searchHandler()`
- `computed()` phụ thuộc `Date.now()`/thời gian trôi — không tự re-evaluate khi KHÔNG polling
- `effect()` chạy SAU `ngOnInit()` — không phải lúc constructor
- `model()` 2 chiều — side-effect gắn ở method bị bypass khi giá trị đổi từ ngoài vào

---


Tách từ `frontend/skill/skills/working-on-fe-modules/reference/fe-patterns-gotchas.md` (2026-07-23) — gotcha liên quan `computed()`/`effect()`/OnPush reactivity.

---

## Custom ControlValueAccessor + OnPush: writeValue() thiếu markForCheck()

Component custom form-control OnPush implement `writeValue(value)` (property mutation, vd `this.selectedItem = ...`) mà không gọi `this.cdr.markForCheck()` → khi cha gọi `formControl.patchValue()`/`setValue()` từ code (không qua thao tác UI/onSelect), giá trị FormControl đổi đúng nhưng view KHÔNG re-render tới khi có CD trigger khác. Luôn `markForCheck()` cuối `writeValue()` khi component OnPush.

---

## OnPush + `ngOnInit` async → rfModel set muộn, form không bao giờ render

BaseForm subclass OnPush override `async ngOnInit()` rồi `await` trước khi gọi `initialForm()`
(vd chờ load cache IndexedDB để tính giá trị mặc định) → `rfModel` (plain property, không phải
signal) vẫn `undefined` ở lần CD đầu tiên, `@if (rfModel)` render rỗng; khi promise resolve set
`rfModel` xong không ai gọi `markForCheck()` → OnPush không tự re-check, form biến mất vĩnh viễn.
Fix: giữ `ngOnInit` đồng bộ (build form ngay với fallback), side-effect bất đồng bộ tách riêng
method khác, patch lại field + gọi `this.cdr.markForCheck()` sau khi resolve.

**Đã fix triệt để bằng `BaseSignalForm`** (`share/base/base-signal-form.ts`, 2026-08) — `rootField`
dựng 1 lần ở field initializer, LUÔN tồn tại từ constructor, template bind thẳng `rootField.xxx()`
không cần `@if (rootField)` gate chờ giá trị set muộn như `rfModel` cũ. `model` là `WritableSignal`
thật nên `this.model.set(...)` dù gọi sau `await` (async `ngOnInit()` override) vẫn re-render đúng
qua signal-graph, không cần `markForCheck()`. Form viết bằng `BaseSignalForm` không dính gotcha này —
**trừ khi** subclass tự thêm 1 `@if` MỚI bọc ngoài nội dung form dựa trên 1 plain boolean (không phải
signal) set muộn trong async callback — lúc đó đúng bug class này tái phát ở chỗ mới. Cần gate loading
→ dùng `signal()` (xem `modules/utilities/switch-branch/switch-branch.ts`: `@if (loadingOptions()) {...}`; mẫu gốc `switch-company.ts` đã xoá, D15),
không dùng plain field.

---

## `@Input` chỉ set trong `ngOnInit` bị stale khi route-reuse

Tab khám mount qua `@defer` rồi giữ nguyên suốt session; Angular RouteReuseStrategy mặc định tái dùng component khi đổi appointment (điều hướng nội bộ "Hẹn buổi tiếp theo") — `ngOnInit` KHÔNG re-fire. Component chỉ `customer.set(appointment.customer)` trong `ngOnInit` (không có `ngOnChanges`) sẽ đứng yên ở bệnh nhân cũ. Bắt buộc thêm `ngOnChanges` re-sync khi `appointment` input đổi cho MỌI tab con của `ExaminationPage`.

Áp dụng cho MỌI tab con của ExaminationPage, và (từ 2026-07-10) cũng áp dụng cho tab con của CustomerDetail, BrokerDetail, ServiceForm, OrderPlanForm, CustomerForm — các trang này đã chuyển sang `@defer (when activeTab() === 'X')` cho tabpanel. Trước khi thêm state nội bộ copy từ @Input trong component nằm trong 1 trong các trang này → bắt buộc dùng `ngOnChanges`, không chỉ `ngOnInit`.

Cùng bug cũng xảy ra ở LẦN MOUNT ĐẦU TIÊN (không cần route-reuse) khi F5 thẳng vào URL đã có `?tab=X` — `@defer` trigger đúng lúc `model()` cha vừa resolve, 1 trong nhiều lệnh gọi phụ thuộc `@Input` trong `ngOnInit` có thể không chạy dù guard đúng. Cùng 1 fix: chuyển sang `ngOnChanges`.

---

## `<ng-template #caption>`/`#header` trong `p-table` — embedded view CD riêng của p-table

Signal đổi giá trị dùng TRONG caption/header template không đảm bảo p-table tự re-check nó, nếu
không có kèm đổi `[value]`/input khác của p-table trong cùng lúc. Phải gọi `this.cdr.markForCheck()`
tường minh ngay sau `.set()` cho state chỉ hiển thị trong caption/header (nút toolbar, badge...), dù
đã convert đúng chuẩn signal + OnPush.

---

## `@Input()` khai báo nhưng cha không bind — thực chất là state nội bộ, không phải Input thật

Trước khi coi 1 field OnPush component có `@Input()` là input thật, kiểm tra template cha có
`[fieldName]="..."` không. Không có → đó là state nội bộ bị gán nhầm dạng field thường trong
callback async (subscribe API, dialogRef.onClose) → OnPush không re-render. Fix: bỏ `@Input()`,
chuyển `signal<T>()` + `.set()`.

---

## `dataSelected` (checkbox chọn nhiều trong BaseList) phải reset trong `searchHandler()`

Pattern `dataSelected: any[]` + `p-checkbox (onChange)` thủ công (không dùng PrimeNG built-in selection) trong table `[lazy]="true" [paginator]="true"` KHÔNG tự reset khi đổi trang/filter — checkbox DOM re-render sạch (do `models.set()` tạo reference mới) nhưng field TS vẫn giữ id cũ, gây gộp nhầm id từ trang trước vào action hàng loạt (in gộp, xoá...).
Fix: `this.dataSelected = [];` ngay đầu `searchHandler()` override — nơi DUY NHẤT set `models()`, chạy mỗi lần `getList()`.

---

## `computed()` phụ thuộc `Date.now()`/thời gian trôi — không tự re-evaluate khi KHÔNG polling

`computed()` chỉ re-run khi SIGNAL dependency đổi — `Date.now()` đọc trong hàm bên trong không phải signal nên trôi thời gian một mình KHÔNG kích hoạt tính lại. Badge "stale sau N giờ" tính qua computed() sẽ đứng yên ở giá trị lúc mount nếu component/trang không có polling/reload định kỳ nào khác vô tình làm mới data.

Fix: thêm 1 signal "tick" tự cập nhật định kỳ, đưa vào computed() làm dependency thật:

```ts
private now = signal(Math.floor(Date.now() / 1000));
constructor() {
  const id = setInterval(() => this.now.set(Math.floor(Date.now() / 1000)), 5 * 60_000);
  inject(DestroyRef).onDestroy(() => clearInterval(id));
}
health = computed(() => computeHealth(this.data(), this.now()));
```

---

## `effect()` chạy SAU `ngOnInit()` — không phải lúc constructor

Thứ tự thật trong `refreshView()` (`@angular/core/fesm2022/_debug_node-chunk.mjs`):
`executeTemplate` → preOrderHooks (`ngOnInit`) → `runEffectsInView` → `detectChangesInChildComponents`.
Effect đăng ký ở constructor vẫn chạy lần đầu SAU khi `ngOnInit()` đã xong (cùng `refreshView()` đầu tiên).

Hệ quả: guard kiểu "bỏ qua echo của giá trị seed" (`if (v === seeded) { seeded = undefined; return; }`)
được viết theo giả định "effect chạy lúc constructor khi model còn `{}`" sẽ **không bao giờ được tiêu
thụ** — `effectOnChange` skip-lần-đầu đã nuốt lần chạy đó, nên guard "lên đạn" suốt vòng đời component
và nuốt oan 1 lần side-effect hợp lệ về sau.

Fix: disarm guard ở lần action ĐẦU TIÊN bất kể giá trị có khớp seed hay không — đúng cho cả 2 thứ tự:

```ts
if (this.seeded !== undefined) {
  const wasSeed = value === this.seeded;
  this.seeded = undefined;      // disarm bất kể nhánh nào
  if (wasSeed) return;
}
```

Seed model bất đồng bộ (`GET(id)` rồi `model.set()`) thì lần chạy đầu của effect THẬT SỰ thấy model rỗng
— vẫn phải giữ stamp ghi đồng bộ, không dựa vào thứ tự trên.

---

## `model()` 2 chiều — side-effect gắn ở method bị bypass khi giá trị đổi từ ngoài vào

Method có side-effect (vd `togglePillUnread() { this.pillUnread.set(...); this.clearSelection(); }`) chỉ chạy khi CHÍNH component gọi qua template click. Nếu 1 component khác đổi cùng signal qua `[(pillUnread)]` 2 chiều (model() input được cha ghi trực tiếp), Angular chỉ set giá trị — side-effect trong method KHÔNG chạy.

Fix: chuyển side-effect vào `effect()` theo dõi TRỰC TIẾP giá trị signal (bắt được mọi nguồn thay đổi):

```ts
let prev = this.pillUnread();
effect(() => {
  const v = this.pillUnread();
  if (v !== prev) { prev = v; this.clearSelection(); }
});
```

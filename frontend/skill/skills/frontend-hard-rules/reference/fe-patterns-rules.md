# Frontend patterns rules chi tiết

## Mục lục

- 1. Subscribe cleanup — bắt buộc
- 1b. Load-on-change — dùng `Subject + switchMap` khi có HTTP
- 2. OnPush + Signal — bắt buộc cho component mới
- 3. Label attachment trong list — IndexedDB vs BE JOIN (CRITICAL)
  - Decision tree
  - ✅ Case 1 — IndexedDB helper có getter → attach trong `addLabel()`
  - ❌ Case 2 — Không có helper getter → BE JOIN
  - Tra helper có getter — tự grep, KHÔNG dựa danh sách tĩnh
- 4. IndexedDB Autocomplete vs network search (CRITICAL)
  - Decision tree
  - ✅ Cách dùng IndexedDB autocomplete trong form
  - Điểm khác biệt so với pattern cũ (p-autoComplete)
  - Validators dynamic
  - Tra AutocompleteFormControl đã có — tự tra, KHÔNG dựa danh sách tĩnh
- 5. `@for` track — bắt buộc dùng `item.id`
- 6. Keyword search — Subject + debounce, không setTimeout
- 7. switchMap — không mutate response object

---


Subscribe cleanup, OnPush+Signal, Label attachment (IndexedDB vs BE JOIN), IndexedDB Autocomplete vs network search, @for track.


---

## 1. Subscribe cleanup — bắt buộc

→ Xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.4 (`takeUntilDestroyed`; `BaseForm`/`BaseList` đã có sẵn `protected destroyRef` — không inject lại).

---

## 1b. Load-on-change — dùng `Subject + switchMap` khi có HTTP

Khi component load data mỗi lần một control thay đổi (vd chọn chi nhánh → load danh sách template; ca gốc là chọn company — đã bỏ, D15), KHÔNG subscribe HTTP request trực tiếp trong `valueChanges.subscribe`. Request cũ không bị cancel → race condition ghi đè kết quả theo thứ tự response.

```typescript
// ✅ Subject + switchMap — cancel request cũ tự động
private readonly _loadTemplates$ = new Subject<number>();

ngOnInit() {
  // Setup pipeline TRƯỚC super.ngOnInit() (vì super gọi buildForm)
  this._loadTemplates$.pipe(
    takeUntilDestroyed(this.destroyRef),
    switchMap(cid => {
      this.isLoading.set(true);
      return this.service.getList(cid).pipe(
        catchError(err => { ... return of([]); }),
        finalize(() => this.isLoading.set(false)),
      );
    }),
  ).subscribe(list => this.items.set(list));

  super.ngOnInit();
}

buildForm(model) {
  ...
  this.rfModel.get('branch_id')!.valueChanges
    .pipe(takeUntilDestroyed(this.destroyRef))
    .subscribe(cid => {
      if (cid) this._loadTemplates$.next(cid);  // ← push vào Subject
    });

  // Pre-load nếu đã có giá trị ban đầu
  if (initialId) this._loadTemplates$.next(initialId);
}

// ❌ KHÔNG làm — request cũ vẫn chạy nền
this.rfModel.get('branch_id')!.valueChanges.subscribe(cid => {
  this.service.getList(cid).subscribe(list => this.items.set(list)); // race condition
});
```

**Khi nào cần Subject + switchMap:**
- HTTP request triggered bởi form control thay đổi
- User có thể thay đổi liên tục (branch picker, filter dropdown, search input)
- Response time không ổn định (API call, external service)

**Khi KHÔNG cần (subscribe trực tiếp OK):**
- Observable complete sau 1 emit (HTTP từ `ngOnInit` như `getTriggers`, `getAvailableFields`)
- Chỉ đọc giá trị form, không call HTTP

---

## 2. OnPush + Signal — bắt buộc cho component mới

→ Xem `frontend/skill/skills/frontend-hard-rules/SKILL.md` B.3 (OnPush + signal state; KHÔNG mutate object — replace reference).

---

## 3. Label attachment trong list — IndexedDB vs BE JOIN (CRITICAL)

Khi list template cần hiển thị tên/label từ ID (vd `branch_id` → tên chi nhánh, `groupLocal_id` → tên khu vực), có 2 nguồn:

### Decision tree

```
Field cần label?
  → IndexedDB helper có getter method?
      ✅ YES → attach trong addLabel() của Service (FE, không cần BE)
      ❌ NO  → BE JOIN + trả về nested object
```

### ✅ Case 1 — IndexedDB helper có getter → attach trong `addLabel()`

```typescript
// service.ts
addLabel(item: XxxModel) {
  return {
    ...item,
    branchLabel:     this.branchHelper.getBranchName(item.branch_id),
    campaignLabel:   this.campaignHelper.getCampaignName(item.campain_event_id),
    serviceTypeLabel: this.serviceTypeHelper.getLabel(item.serviceType_id),
    pageFacebook:    this.pageFacebookHelper.getPageFacebookInfo(item.page_facebook_id),
    // createdByInfo, updatedByInfo, marketingInfo → userHelper.getUserInfo(id)
  };
}

async ensureCacheLoaded() {
  await Promise.all([
    this.branchHelper.ensureBranchesLoaded(),
    // ... mọi helper dùng trong addLabel phải ensureLoaded ở đây
  ]);
}
```

Template đọc `model.branchLabel`, `model.serviceTypeLabel`, etc. (lịch sử: `companyInfo`/`companyHelper` đã xoá cùng company — D15; `campaignHelper`/`pageFacebookHelper` cũng không còn trong `indexeddb/config/components/`)

**Cách check helper có getter không:**
- Mở `src/app/indexeddb/config/components/<name>/<name>-helper.service.ts`
- Nếu chỉ có `ensureXxxLoaded()` → **KHÔNG có getter → cần BE**
- Nếu có `getXxxName()` / `getXxxInfo()` / `getLabel()` → **dùng FE**

### ❌ Case 2 — Không có helper getter → BE JOIN

Handler phải LEFT JOIN bảng liên quan và trả về nested object:

```go
// handler.go — BuildXxxQuery
q := tenant.ScopedDBNullBranch(c, h.DB, "xxx_table").
    Model(&dao.XxxEntity{}).
    Joins("LEFT JOIN group_local gl ON gl.id = xxx_table.groupLocal_id").
    Select("xxx_table.*, gl.name AS groupLocal_name")

// Sau Paginate — nest thành object để template đọc model.groupLocal?.name
for i, item := range items {
    name := item["groupLocal_name"]
    delete(item, "groupLocal_name")
    if name != nil && name != "" {
        item["groupLocal"] = map[string]interface{}{"name": name}
    }
    items[i] = item
}
```

Template: `{{ model.groupLocal?.name || '—' }}`

**Lưu ý khi có JOIN:**
- Qualify tất cả filter column với table prefix (`digital_facebook.branch_id` thay vì `branch_id`) để tránh "ambiguous column"
- `Order("main_table.id DESC")` — cũng cần prefix
- SUM/COUNT query cũng phải dùng cùng `BuildXxxQuery` (đã có JOIN)

### Tra helper có getter — tự grep, KHÔNG dựa danh sách tĩnh

```bash
# Getter của 1 helper (getXxxName/getXxxInfo/getLabel...) — không match gì → cần BE JOIN
grep -nE "get[A-Z][A-Za-z]*\(" frontend/src/app/indexeddb/config/components/<name>/<name>-helper.service.ts

# Liệt kê toàn bộ helper ĐÃ có getter
grep -rlE "get[A-Z][A-Za-z]*\(" frontend/src/app/indexeddb/config/components --include="*-helper.service.ts"
```

---

## 4. IndexedDB Autocomplete vs network search (CRITICAL)

Khi form có field chọn resource (material, service, unit, v.v.), **KHÔNG** dùng `p-autoComplete` + gọi API `/load-by-keyword`. Kiểm tra trước:

### Decision tree

```
Resource cần autocomplete?
  → Có XxxAutocompleteFormControl trong indexeddb/config/components/<name>/?
      ✅ YES → dùng IndexedDB autocomplete (không gọi mạng)
      ❌ NO  → dùng p-autoComplete + loadXxxByKeyword API
```

### ✅ Cách dùng IndexedDB autocomplete trong form

```html
<!-- Template: dùng formControlName trực tiếp vào material_id (không cần material_data) -->
<!-- Lịch sử: ca gốc (ERP, kho vật tư — D2) còn input [companyId]; đã bỏ, D15 -->
<app-material-autocomplete-form-control
  formControlName="material_id"
  (valueChange)="onSelectMaterial($event, index)"
/>
```

```typescript
// TS: inject cache service để lấy related fields khi user chọn
private materialCache = inject(MaterialCacheService);
// Handler: nhận id (không phải object), lookup cache để lấy related fields
onSelectMaterial(id: number | null, key: number) {
    const groups = (this.rfModel.controls['groupServiceMaterial'] as FormArray).controls as FormGroup[];
    if (id) {
        const m = this.materialCache.getById(id);
        groups[key].patchValue({ unit_id: m?.unit_id ?? null });
    } else {
        groups[key].patchValue({ unit_id: null, quantity_min: 0, quantity_max: 0, note: null });
    }
}

// FormGroup: KHÔNG cần material_data — chỉ cần material_id
const fb = this.fb.group({
    material_id: [serviceMaterial?.material_id ?? null],
    unit_id: [serviceMaterial?.unit_id],
    // ...
});
```

### Điểm khác biệt so với pattern cũ (p-autoComplete)

| | Cũ (p-autoComplete + API) | Mới (IndexedDB) |
|---|---|---|
| Form control | `material_data` lưu `{label, value}` | `material_id` lưu `number` |
| Khởi tạo từ edit | Build `{label, value}` từ `material.name` | `writeValue(id)` tự lookup cache |
| Gọi mạng | Có — mỗi lần gõ | Không — filter local |
| Filter scope | Không | Theo input của control (ca gốc `[companyId]` đã bỏ, D15; hiện không control nào có input scope) |
| Event | `(onSelect)`, `(onClear)` | `(valueChange)` — cả select lẫn clear |
| Related fields | Từ `$event.value.unit_id` (object) | `cache.getById(id).unit_id` |

### Validators dynamic

Chuyển validator từ `material_data` sang `material_id`:

```typescript
fb.controls['material_id'].valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((value) => {
    if (value) {
        fb.controls['quantity_min'].setValidators([Validators.required]);
        fb.controls['quantity_max'].setValidators([Validators.required]);
    } else {
        fb.controls['quantity_min'].clearValidators();
        fb.controls['quantity_max'].clearValidators();
    }
    fb.controls['quantity_min'].updateValueAndValidity();
    fb.controls['quantity_max'].updateValueAndValidity();
});
```

### Tra AutocompleteFormControl đã có — tự tra, KHÔNG dựa danh sách tĩnh

```bash
find frontend/src/app/indexeddb/config/components -name "*autocomplete-form-control*"
# Check control có input lọc (scope) không — ca gốc grep companyId, đã bỏ D15:
grep -n "@Input\|input(" <path>/<name>-autocomplete-form-control.ts
```

Cascading theo field khác qua **live BE call** (không phải IndexedDB cache) — chưa có base class, tự viết: debounce trong component, disable nút cho tới khi đủ input, lỗi/provider không hỗ trợ vẫn cho gõ tay (forceSelection=false). Ví dụ: `ai-config-form.ts` (`fetchModels`, model phụ thuộc provider_id+api_key).

---

## 5. `@for` track — bắt buộc dùng `item.id`

```html
<!-- ✅ ALWAYS -->
@for (item of models(); track item.id) { ... }

<!-- ❌ NEVER -->
@for (item of models(); track $index) { ... }
```

---

## 6. Keyword search — Subject + debounce, không setTimeout

```typescript
// ✅ CORRECT
private keywordSubject = new Subject<string>();

initFilter() {
  // ...other filters...
  this.keywordSubject
    .pipe(debounceTime(500), takeUntilDestroyed(this.destroyRef))
    .subscribe((value) => this.dt.filter(value, '_keyword', 'contains'));
}

searchKeyword(event: any) {
  this.keywordSubject.next(event.target.value);
}

// ❌ AVOID — timer không bị cancel khi component destroy
searchKeyword(event: any) {
  if (this.timer) clearTimeout(this.timer);
  this.timer = setTimeout(() => this.dt.filter(event.target.value, '_keyword', 'contains'), 500);
}
```

---

## 7. switchMap — không mutate response object

```typescript
// ✅ CORRECT — spread tạo object mới
switchMap(async (response: any) => ({
  ...response,
  items: response.items ? await this.modelService.addLabels(response.items) : [],
})),

// ❌ AVOID — mutate trực tiếp
switchMap(async (response: any) => {
  response.items = await this.modelService.addLabels(response.items); // mutation
  return response;
}),
```

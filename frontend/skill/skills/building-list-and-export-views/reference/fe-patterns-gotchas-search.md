# Gotchas Search/AutoComplete/List display

## Mục lục

- PrimeNG AutoComplete single-select — value phải là object, không phải array
- `[forceSelection]="true"` trên p-autocomplete xoá ký tự đang gõ (đặc biệt "Đ")
- `searchHandler` override với async `addLabels` — thứ tự operators
- AutoComplete search — dùng `Subject + switchMap` (tham khảo `frontend/skill/skills/frontend-hard-rules/reference/fe-patterns-rules.md` §1b)
- clearFilter() phải restore defaultSearch
- Cắt kết quả search phải theo độ liên quan, không theo thứ tự sort gốc
- Safety-net "item đã chọn dù inactive vẫn hiện" xung đột với cache chỉ-active
- XxxDisplayModel — typed display fields từ addLabel()

---


Tách từ `frontend/skill/skills/working-on-fe-modules/reference/fe-patterns-gotchas.md` (2026-07-23, file gốc vượt 350 dòng) — gotcha liên quan search/autocomplete/list display fields.

---

## PrimeNG AutoComplete single-select — value phải là object, không phải array

```typescript
// ❌ NEVER — array → AutoComplete hiển thị "[object Object]", không resolve được
material_data: model.material
  ? [{ label: model.material.name, value: model.material.id }]
  : []

// ✅ ALWAYS — single-select nhận object hoặc null
material_data: model.material
  ? { label: model.material.name, value: model.material.id }
  : null
```

Multi-select mới dùng array. Single-select (`[multiple]="false"` hoặc default) → object.

---

## `[forceSelection]="true"` trên p-autocomplete xoá ký tự đang gõ (đặc biệt "Đ")

`onInput()` của PrimeNG bỏ qua `updateModel(query)` khi `forceSelection=true` → `inputValue()` (bind `[attr.value]`) không theo kịp text đang gõ. CD trigger giữa lúc gõ (vd `filteredItems.set()` trong `completeMethod`) → input DOM bị reset về giá trị cũ, mất ký tự vừa gõ — "Đ" hay dính nhất vì thường là ký tự đầu tên có dấu, đúng lúc debounce đầu fire.

Fix: bỏ `forceSelection`, thêm `onBlur()` tự revert về giá trị hợp lệ cuối nếu user gõ tự do không chọn item (`typeof selected === 'string'` → revert).

KHÔNG phải giải pháp cho lag search dataset lớn (16k+ record) — cái đó xử lý bằng memoize + `MAX_SEARCH_SUGGESTIONS` + `virtualScroll` (xem gotcha "Cắt kết quả search..." dưới).

---

## `searchHandler` override với async `addLabels` — thứ tự operators

Áp cho `BaseList` (V1). List dùng `BaseListV2` override `fetchList()` thay vì `searchHandler` — xem `skill/skills/building-list-and-export-views/SKILL.md`.

Convention: `takeUntilDestroyed` đặt **CUỐI pipe** (thống nhất toàn codebase). Trước hay sau `switchMap` đều cancel HTTP in-flight khi destroy (unsubscribe propagate upstream) — điều bắt buộc là PHẢI có nó trong pipe.

```typescript
override searchHandler() {
  this.modelService.list({ params: this.searchParams() })
    .pipe(
      switchMap(async (response) => ({
        ...response,
        items: response.items
          ? await this.modelService.addLabels(response.items)
          : [],
      })),
      finalize(() => this.loading.set(false)),
      takeUntilDestroyed(this.destroyRef),        // ← cuối pipe (convention)
    )
    .subscribe({
      next: (response) => {
        this.models.set(response.items ?? []);
        this.paginationInfo.set(response._meta ?? null);
      },
      error: () => this.models.set([]),
    });
}
```

---

## AutoComplete search — dùng `Subject + switchMap` (tham khảo `frontend/skill/skills/frontend-hard-rules/reference/fe-patterns-rules.md` §1b)

PrimeNG AutoComplete `(completeMethod)` event → cần `Subject + switchMap` để cancel race condition, tương tự form control `valueChanges`. Pattern đầy đủ ở `frontend/skill/skills/frontend-hard-rules/reference/fe-patterns-rules.md` §1b "Load-on-change".

```typescript
private readonly searchSubject$ = new Subject<string>();

ngOnInit() {
  this.searchSubject$.pipe(
    debounceTime(300),
    switchMap((query) => this.myService.search(query)),
    takeUntilDestroyed(this.destroyRef),
  ).subscribe((items) => this.autoCompleteList.set(items));
}

onSearch(event: { query: string }) {
  this.searchSubject$.next(event.query);   // chỉ push, không gọi service trực tiếp
}
```

---

## clearFilter() phải restore defaultSearch

Áp cho `BaseList` (V1). `BaseListV2` không dùng `defaultSearch` (default nằm trong `filterFields()`) và có sẵn `clearFilter()` — KHÔNG tự viết lại.

Khi `ngOnInit` set default date range vào `defaultSearch`, `clearFilter()` KHÔNG được `set({})` — phải restore đúng giá trị đó.

```typescript
// ❌ NEVER — xóa mất defaultSearch → API gọi không có date range → timeout/data lớn
clearFilter() {
  this.defaultSearch.set({});
  ...
}

// ✅ ALWAYS — restore đúng default
clearFilter() {
  this.defaultSearch.set({
    'filter[created_at][gte]': this.timezoneService.monthStartUnix(),
    'filter[created_at][lte]': this.timezoneService.todayEndUnix(),
  });
  ...
}
```

---

## Cắt kết quả search phải theo độ liên quan, không theo thứ tự sort gốc

Mảng nguồn đã `.sort()` theo field A (vd `code`), search cả field A lẫn field B (vd `name`), rồi `.slice(0, N)` → kết quả thiên vị theo field A, ẩn mất match đúng ở field B nằm ngoài N phần tử đầu.

Fix: rank theo tier TRƯỚC khi cắt N — `startsWith` field A → `startsWith` field B → `includes` field A → `includes` field B — không cắt thô theo thứ tự mảng gốc.

---

## Safety-net "item đã chọn dù inactive vẫn hiện" xung đột với cache chỉ-active

Component có logic prepend item inactive đã chọn (`includeItemId`/tương tự) vào danh sách hiển thị — khi optimize filter/search, PHẢI filter trực tiếp trên danh sách đã có safety-net đó. KHÔNG dùng 1 danh sách cache chỉ-active làm allowlist/cross-reference — sẽ làm mất item inactive đã chọn khỏi kết quả search.

---

## XxxDisplayModel — typed display fields từ addLabel()

Khi `addLabel()` inject các fields hiển thị không có trong `XxxModel` (`statusClass`, `createdByInfo`, `branchLabel`, ...) → tạo `XxxDisplayModel extends XxxModel`.

```typescript
// model.ts
export interface NoteDisplayModel extends NoteModel {
  customerDisplayName?: string;
  created_atFormatted?: string;
  statusLabel?: string;
  statusKey?: string;
  statusClass?: string;
  createdByInfo?: UserProfileModel;
  branchLabel?: string;
}

// service.ts
addLabel(item: NoteModel) { ... }   // return type inferred

async addLabels(items: NoteModel[]): Promise<NoteDisplayModel[]> {
  await this.ensureCacheLoaded();
  return items.map((item) => this.addLabel(item)) as NoteDisplayModel[];
}

// component.ts
export class NoteList extends BaseList<NoteDisplayModel> { ... }
```

Return type của `addLabel` để inferred (không annotate) để tránh conflict với helper service return types. Cast `as NoteDisplayModel[]` chỉ ở `addLabels`.

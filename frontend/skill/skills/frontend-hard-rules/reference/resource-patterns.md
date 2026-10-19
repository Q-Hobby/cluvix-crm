# Resource patterns

## Mục lục

- Quyết định: Observable vs rxResource vs apiResource
- `rxResource()` — wrap service có sẵn
- `apiResource()` — GET đơn giản, không transform
- Tại sao KHÔNG dùng `httpResource()` raw
- Signal debounce — `.debounced()`

---


## Quyết định: Observable vs rxResource vs apiResource

| Trường hợp | Dùng gì |
|---|---|
| Service có `afterLoad()` / `addLabel()` (AppointmentService, v.v.) | `rxResource()` wrap Observable |
| GET endpoint đơn giản, raw JSON không cần transform | `apiResource()` — `share/resource/` |
| Mutations: create / update / delete | Observable + `.subscribe()` như cũ |
| FormControl.valueChanges cần debounce | `toSignal(obs.pipe(debounceTime(...)))` — KHÔNG `.debounced()` |

---

## `rxResource()` — wrap service có sẵn

```typescript
import { rxResource } from '@angular/core/rxjs-interop';

readonly id = input.required<number>();
private svc = inject(AppointmentService);

// afterLoad() chạy bên trong svc.get() → unix → Date ✅
readonly appt = rxResource({
  request: () => this.id(),
  loader: ({request: id}) => this.svc.get(id),
});
// Template: appt.value() | appt.isLoading() | appt.error() | appt.reload()
// Tự re-fetch khi id() thay đổi — không cần ngOnInit, takeUntilDestroyed
```

---

## `apiResource()` — GET đơn giản, không transform

Wrapper tại `share/resource/` — đã handle `baseUrl` + `withCredentials: true`.

```typescript
import { apiResource } from '@/share/resource';

// Stat/chart endpoint — raw number/string, không cần afterLoad
readonly stats = apiResource<{ total: number }>(
  () => ({ path: `crm/appointment/overview?year=${this.year()}` }),
);

// Với params
readonly list = apiResource<ItemResponse>(
  () => ({ path: 'crm/patient', params: { page: this.page(), limit: 20 } }),
);

// Suspend khi chưa có id (trả undefined → không fetch)
readonly detail = apiResource<DetailResponse>(
  () => this.id() ? { path: `crm/patient/${this.id()}` } : undefined,
  { parse: raw => (raw as { data: DetailResponse }).data },
);
```

---

## Tại sao KHÔNG dùng `httpResource()` raw

- Không đi qua `ApiService` → thiếu `withCredentials: true` → API 401
- Bypass `afterLoad()` → unix timestamp vẫn là `number`, date fields vẫn là `string`
- Phải hardcode full URL thay vì path relative

Dùng `apiResource()` thay — wrapper đã xử lý các vấn đề trên.

---

## Signal debounce — `.debounced()`

Chỉ dùng khi debounce **Signal** (search input). KHÔNG thay `toObservable+debounceTime` khi source là Observable.

```typescript
// ✅ Signal search — dùng .debounced()
searchValue = signal('');
debouncedSearch = this.searchValue.debounced(300);

// ✅ FormControl.valueChanges — giữ toSignal+debounceTime
filterValues = toSignal(this.form.valueChanges.pipe(debounceTime(500), startWith(this.form.value)));
```

# IndexedDB sync

## Mục lục

- Flow FE
- Table registry
- Sync layers (3 lớp)
  - Lớp 1: APP_INITIALIZER preload
  - Lớp 2: Trang `/syncing` (sau login)
  - Lớp 3: Header-based per-table sync
  - Auto-reload cho tenant-defining tables
  - Reload cho self-user edit (riêng biệt)
- Race safety (FE)
- Thêm master data table mới (FE steps)
  - Index hỗ trợ query nhanh
- 1 DB duy nhất `cluvixData` (v50, 2026-05)
- Status filter — quy tắc bất biến
  - Cache service — status filter pattern (CRITICAL)
- UserHelperService — display name convention (CRITICAL)
- APP_INITIALIZER — preloadIndexedDb race
- Common bugs
- Files chính (FE)

---


Master data cấu hình (companies, branches, units, services, ...) cache ở FE qua IndexedDB. BE thông báo thay đổi qua header `X-Dexie-Versions` → FE chỉ sync table lệch version, không full sync. Cơ chế phía BE (BumpDexieTable, Redis cache, sentinel `__all__`, endpoint `/api/crm/indexeddb/sync`): doc repo cha `../skill/skills/handling-auth-and-workspace/reference/indexeddb.md`.

## Flow FE

```
FE response interceptor → headerSync.checkVersionFromHeaders
        → handleTableVersionsHeader
          ├─ localGlobalVersion=0 → runFullSync (fresh install / chưa từng sync)
          ├─ header empty + localGV>0 → no-op (BE chưa có per-table data)
          ├─ "__all__" mismatch → runFullSync (legacy/generic handler bumped)
          └─ per-table mismatch → syncTableOnce(table) parallel cho từng table
        → setTableVersions (re-read latest, race-safe merge)
```

## Table registry

Tên table trong `src/app/indexeddb/config/table-registry.ts` phải khớp **chính xác** registry BE (`configTables` trong `internal/modules/webapp/config/indexeddb/handler/sync.go`, repo cha) — sai tên = FE invalidate sai table → silent stale.

## Sync layers (3 lớp)

### Lớp 1: APP_INITIALIZER preload
`HeaderSyncService.ensureSyncedIfEmpty()` chạy trong `app.config.ts` sau `initializeAuth`. Nếu localVersion=0 → full sync trước khi component mount.

### Lớp 2: Trang `/syncing` (sau login)
`login.ts` navigate sang `/syncing?redirect=<url>` ngay sau khi có token — KHÔNG tự sync. `syncing.ts` gọi `ensureSyncedIfEmpty()` + `refreshCompanyActive()` trong `ngOnInit`, xong mới navigate tới đích thật. Tách riêng để tránh block nút Login (UX).

### Lớp 3: Header-based per-table sync
Mỗi response → `responseHandlerInterceptor` → `headerSync.checkVersionFromHeaders(headers)`: parse header, compare in-memory `tableVersions` (lazy load từ IndexedDB metadata), sync mismatched tables parallel qua `registry.invalidateAndSync(name)`, persist new versions.

### Auto-reload cho tenant-defining tables
`companies`, `branches` đổi → ảnh hưởng toàn UI. Khi mismatch: per-table sync xong → lưu message `sessionStorage['post_reload_toast']` → `setTimeout(() => location.reload(), 0)` (defer 1 tick để form subscriber kịp close dialog) → reload xong `app.component.ts` đọc sessionStorage → toast. Chỉ fire qua per-table path, KHÔNG qua `runFullSync`/`ensureSyncedIfEmpty` (boot flow không reload). Mở rộng: thêm entry `RELOAD_ON_CHANGE_TABLES` trong `header-sync.service.ts`.

### Reload cho self-user edit (riêng biệt)
Sửa user KHÔNG trigger global reload. Logic trong `user-form.ts`: `model().id === userId()` (sửa chính mình) → sessionStorage toast + reload; sửa user khác → close bình thường, `users` table sync qua header. Cross-tab self-edit (hiếm): authUser signal tab khác stale — user tự refresh.

## Race safety (FE)

- `tableSyncPromises` Map dedup per-table sync (nhiều response cùng trigger 1 table → share Promise)
- `syncPromise` dedup full sync trong cùng tab
- Cross-tab lock fixed key `sync-lock-full` TTL 30s — tab thứ 2 skip + refresh in-memory từ IndexedDB. Edge: tab crash giữa sync → lock stuck 30s, chấp nhận.
- Re-read `this.tableVersions` sau awaits trước khi merge → tránh lost update khi 2 response concurrent mang version khác nhau

## Thêm master data table mới (FE steps)

> BE steps (handler CRUD + BumpDexieTable + configTables): doc repo cha.

1. Add `{ name: 'tableName', keyPath: 'id' }` vào `src/app/indexeddb/config/table-registry.ts`
2. **⚠ CRITICAL — bump `INDEXEDDB_DB_VERSION`** trong `define/indexeddb.define.ts`. Quên bump: `onupgradeneeded` không fire với user có IDB cũ → store mới không được tạo → `putItem()`/`getItem()` throw `'store not found'` → silently caught → `load()` luôn trả null → `restoreAuthUser()` miss session → permission guards redirect `/auth/accessdenied` khi F5 trên sub-routes.
3. Tạo cache service kế thừa `BaseCacheService<T>`
4. Tạo form-control wrapper cho dropdown/multiselect

### Index hỗ trợ query nhanh
`TableConfig.indexes?` khai báo IDB index khi cần `getByIndex(storeName, indexName, value)`:
```ts
{ name: 'customerLeads', keyPath: 'id',
  indexes: [{ name: 'status', field: 'status' }, { name: 'assigned_user_id', field: 'assigned_user_id' }] }
```
`BaseIndexedDBService.onupgradeneeded` tự tạo index nếu chưa tồn tại. Sau khi thêm → bump `INDEXEDDB_DB_VERSION`.

## 1 DB duy nhất `cluvixData` (v50, 2026-05)

Trước v50 có 3 IndexedDB (`cluvixData`, `EmrConfigDB`, `CustomerLeadDB`) — đã gộp về 1. EMR stores + `customerLeads` nằm trong `TABLE_REGISTRY` chính; `cleanupLegacyDBs()` tự deleteDatabase 2 DB cũ (fire-and-forget, idempotent). Endpoint sync hợp nhất: `/api/crm/indexeddb/sync?tables=...` cover cả config + EMR (`sync-emr` đã xóa).

## Status filter — quy tắc bất biến

| Endpoint | Mục đích | Status filter |
|---|---|---|
| Selector/lookup API (`load-*`, `get-all-*`) | FE form chọn khi tạo mới | Bắt buộc `status = 1` (user: `status = 10`) |
| IndexedDB sync (`/sync?tables=...`) | Cache toàn bộ để hiển thị label | **KHÔNG filter status** — sync cả record disabled |

Lý do: đơn cũ chọn dịch vụ A, admin disable A → đơn vẫn phải hiện đúng tên A (FE tra label trong IndexedDB). Sync filter status = list trắng/thiếu tên.

### Cache service — status filter pattern (CRITICAL)

`BaseCacheService.getByCompanyIds()` trả toàn bộ items KHÔNG filter status → dropdown leak item disabled khi form control đi path này.

**Rule:** Mọi `*CacheService` dùng trong selector/dropdown phải override:
```typescript
public override getByCompanyIds(companyIds: number[]): XxxModel[] {
  return super.getByCompanyIds(companyIds).filter(item => item.status === 1);
}
```

**Rule:** Form control phải có getter unfiltered riêng để preserve selected item bị disabled khi update:
```typescript
// cache service
getAllXxxUnfiltered(): XxxModel[] { return this.getAllItems(); }
// form control loadItems(): nếu selectedValue không nằm trong filtered → prepend item disabled đang chọn
```
Đã fix (2026-06-03): `ServiceComboCacheService`, `ServicesCacheService`, `DiagnosticCacheService`. Cascade disable service_type → service: `ServicesCacheService` inject `ServiceTypeCacheService`, getter chỉ trả service thuộc type `status === 1`.

## UserHelperService — display name convention (CRITICAL)

`indexeddb/config/components/user/user-helper.service.ts`:

| Method | Returns | Nickname logic |
|---|---|---|
| `getLabel(id)` | `string` | ✅ `nickname \|\| fullname` |
| `getUserInfo(id)` | object | ✅ `fullname` đã pre-compute = `nickname \|\| fullname` |

Template dùng `.fullname` thẳng — KHÔNG tự xử lý `nickname || fullname` lại (thừa).

## APP_INITIALIZER — preloadIndexedDb race

Sau sync xong phải reload **cả company lẫn call center** — thiếu một là signal đó null suốt session:
```typescript
await Promise.all([
  authService.refreshCompanyActive(),
  authService.refreshCallCenterActive(), // ← PHẢI có; thiếu → popup-call không init SDK → click SĐT chỉ copy
]);
```

## Common bugs

- **FE không refresh sau CRUD**: BE bump sai tên table — verify khớp `table-registry.ts`.
- **Infinite full sync loop**: BE chưa có dexieVersions JSON + FE localVersion=0 — `setLocalVersion(Date.now())` sau mọi sync (đã có trong code).
- **2 tab cùng full sync**: lock `sync-lock-full` 30s xử lý — tab 2 skip + refresh in-memory.
- **`computed()` đọc cache qua hàm thường sẽ ĐÓNG BĂNG**: `BaseCacheService.filterUsers()`/`getAllItems()` đọc `cachedItems` — thuộc tính THƯỜNG, không phải signal ⇒ `computed()` nào chỉ gọi chúng sẽ chạy đúng 1 lần rồi đóng băng; bảng nạp async nên lần chạy đầu gần như luôn rơi vào lúc cache rỗng → danh sách rỗng vĩnh viễn, không lỗi nào (`tsc`/AOT đều xanh). Phải đọc signal của registry trong CÙNG computed để tạo phụ thuộc: `this.registry.getTableService('users')?.getAll()()` (khuôn `base-multiselect-indexed.ts`), rồi mới gọi hàm lọc.
- **Dropdown lọc theo department rỗng sau login dù IDB có data (fix 2026-07-27)**: race trong `BaseCacheService` — `getAllItems()` (caller B) populate `cachedItems` + build index dẫn xuất (`departmentIndex`) TRONG LÚC `loadItems()` (caller A) đang await `loadTableOnDemand`; A resume gọi `onCacheCleared()` xoá index nhưng cachedItems vẫn ấm → `getAllItems()` skip rebuild → index null vĩnh viễn, filter theo department trả 0 (users total vẫn đủ). Reload hết vì preload đổi thứ tự init. Fix 2 lớp: `loadItems()` gọi `invalidateMemoryCache()` (xoá đồng bộ cache + index + bump dataVersion) và `customUserFilter` rebuild `departmentIndex` lazily khi null. **Rule: `invalidateMemoryCache()` là entry point invalidation DUY NHẤT (đã tự gọi `onCacheCleared()` bên trong) — không gọi riêng `onCacheCleared()`; subclass có index dẫn xuất PHẢI rebuild được lazily, không giả định index sống cùng đời với cachedItems.**

**`getOrBuildDerived(key, builder)`**: helper generic trong `BaseCacheService` để memoize 1 giá trị derived (vd danh sách đã precompute search field cho autocomplete) — tự clear qua `derivedCache` bên trong `invalidateMemoryCache()`. Dùng cái này thay vì tự viết `onCacheCleared()` override thủ công để reset field cache riêng của subclass.


## Files chính (FE)

- `core/services/header-sync.service.ts` — main sync logic
- `indexeddb/config/service/base-indexed-db.service.ts` — IndexedDB primitives
- `indexeddb/config/service/master-data-registry.service.ts` — table registry + load
- `indexeddb/config/table-registry.ts` — table list + keyPath
- `core/interceptors/response-handler.interceptor.ts` — trigger checkVersionFromHeaders
- `app.config.ts` — APP_INITIALIZER preloadIndexedDb
- `modules/auth/login/login.ts` — login flow sync

## Control con lọc theo `companyId` — reset lựa chọn theo TÍNH HỢP LỆ (2026-09-03)

9 control `indexeddb/config/components/{branch,source,diagnostic,customer-category,campaign,group-source,material,customer-care-type,service-combo}` khi `companyId`/`companyIds` đổi (`!firstChange`):
- KHÔNG reset vô điều kiện (khuôn cũ `selectedItem = null; onChange(null)` xoá cả dữ liệu record vừa load khi `undefined → số`, vd record dùng chung được gán thương hiệu lần đầu).
- KHÔNG gate bằng `previousValue != null` (giữ luôn lựa chọn thuộc thương hiệu khác → lưu sai âm thầm).
- ĐÚNG: `if (!this.cache.hasLoaded) return;` rồi `items = getFilteredItems(); if (!items.some(i => i.id === selected.id)) reset` — khuôn sẵn có ở `user-form-control`/`services-autocomplete-form-control`.
- ⚠ "Danh sách LỌC rỗng" KHÔNG đồng nghĩa "cache chưa load": thương hiệu mới có thể thật sự chưa có item → phải đo trên cache NGUỒN (`BaseCacheService.hasLoaded`), không dùng `items.length > 0`.
- Lưới: `company-filter-reset.spec.ts` (9 control × còn/không còn/chưa load/đã load+rỗng/firstChange) gọi `prototype.ngOnChanges` với host giả — mutation bỏ 1 vế phải đỏ.

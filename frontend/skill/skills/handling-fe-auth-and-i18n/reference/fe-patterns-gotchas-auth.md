# Auth gotchas

## Mục lục

- Auth: `restoreAuthUser` phải là async + await trong APP_INITIALIZER
- Auth: `restoreAuthUser` — case IDB bị xóa (CRITICAL)
- Auth: `clearIndexedDB()` xóa toàn bộ DB kể cả `userAuthSession`
- Auth: Đọc permissions phải dùng `userPermissions()` signal, KHÔNG từ localStorage
- Auth: `permission = PERMISSION_SELECT` chỉ có sẵn ở `BaseList`, không ở `BaseDetail`/`BaseForm`
- Auth: `_cachedAuthVersion` — không async IDB read trong Angular effect
- Auth: router.navigate hardcode path trong interceptor phải khớp route thật
- Auth: refresh fail ≠ logout — phân biệt transient (status 0/5xx) vs invalid (401/403)
- `LOGOUT_KEYS` chỉ chứa dữ liệu auth theo tài khoản, KHÔNG chứa preference thiết bị
- `response-handler.interceptor.ts` tự toast 401 khi route đang là `/login`
- Permission matrix (user-role.ts) hardcode đúng 5 cột — KHÔNG thêm permission thứ 6/module
- 1 hành động dùng chung service/component từ nhiều entry point — gate quyền phải khớp Ở MỌI NƠI
- Deep-link từ thông báo/URL: đọc query param phải REACTIVE, không phải `snapshot`

---


## Auth: `restoreAuthUser` phải là async + await trong APP_INITIALIZER

```typescript
// app.config.ts — initializeAuth factory
return async () => {
  await tokenService.initialize();
  await authService.restoreAuthUser(); // ← PHẢI await
};
```

Nếu fire-and-forget void: `preloadIndexedDb` chạy ngay sau khi `initializeAuth` resolve → `authUser` signal còn null → `refreshCompanyActive()` trả null → `authCompanyActive` null cả session.

---

## Auth: `restoreAuthUser` — case IDB bị xóa (CRITICAL)

`hasCached=false` xảy ra khi `userAuthSession` store không tồn tại (chưa bump `INDEXEDDB_DB_VERSION` khi thêm store) hoặc `clearIndexedDB()` đã xóa toàn bộ IDB.

Nếu không handle: function return mà không set `authUser`/`userPermissions` → `authGuard` pass → `permissionGuard` fail (empty Set) → `/auth/accessdenied`.

Case 1a hiện tại cover CẢ access token còn hạn lẫn đã hết hạn — điều kiện KHÔNG được require `hasToken` (F5 sau >15 phút với IDB rỗng sẽ lọt xuống không case nào):

```typescript
// ✅ Case 1a (2026-07-04)
if (!hasCached && hasRefreshToken) {
  const tokenOk = hasToken || (await this.tokenService.trySilentRefresh());
  if (tokenOk) {
    await this.fetchAndSyncMe(); // populate authUser + userPermissions từ BE
  }
  // tokenOk=false: 401/403 → doRefresh đã clearToken → authGuard redirect /login;
  // lỗi mạng → KHÔNG clear, authGuard retry khi điều hướng
  return;
}
```

Đi kèm: `authGuard` silent-refresh path phải `return waitInitialized$` (không `of(true)`) — refresh xong nhưng `fetchAndSyncMe` chưa xong thì `requirePerm` vẫn thấy Set rỗng. Xem `authGuard.ts`.

---

## Auth: `clearIndexedDB()` xóa toàn bộ DB kể cả `userAuthSession`

`clearAllAuthData()` (gọi khi logout) → `cacheManager.clearIndexedDB()` → `indexedDB.deleteDatabase('cluvixData')` — xóa ENTIRE database bao gồm `userAuthSession` store.

Sau login `userAuthSession.save()` rebuild lại. Nếu save fail silently (vd store chưa tồn tại do quên bump `INDEXEDDB_DB_VERSION`) → boot tiếp theo: `load()` trả null → `hasCached=false` → hit missing case trên.

---

## Auth: Đọc permissions phải dùng `userPermissions()` signal, KHÔNG từ localStorage

```typescript
// ❌ NEVER — getPermissionStrings() đọc localStorage (non-reactive)
// → effect/computed chỉ chạy 1 lần, không cập nhật khi admin đổi role
const permStrings = this.rbacService.getPermissionStrings();

// ✅ ALWAYS — signal luôn đúng dù load từ IDB hay login
const permStrings = Array.from(userPermissions());
```

**Đặc biệt với `computed()`:** Angular chỉ track signal dependencies — `getPermissionStrings()` đọc localStorage nên computed không bao giờ re-evaluate:

```typescript
// ❌ computed() không reactive — evaluate 1 lần lúc khởi tạo, không cập nhật khi role đổi
readonly modules = computed(() => {
  const perms = new Set(this.rbacService.getPermissionStrings()); // localStorage, non-signal
  ...
});

// ✅ computed() reactive
readonly modules = computed(() => {
  const perms = userPermissions(); // signal → computed tự re-run khi thay đổi
  ...
});
```

`setPermissionsFromStrings(arr)` (IDB restore) KHÔNG ghi localStorage — chỉ set signal `userPermissions`.
`setPermissions(arr)` (login/refresh) có ghi localStorage.

---

## Auth: `permission = PERMISSION_SELECT` chỉ có sẵn ở `BaseList`, không ở `BaseDetail`/`BaseForm`

`*hasPerm="permission.xxx"` trong template dựa vào property `permission` — `BaseList` đã có sẵn (`permission = PERMISSION_SELECT`), nhưng `BaseDetail`/`BaseForm` KHÔNG kế thừa property này. Component detail/form nào cần gate nút bằng `*hasPerm` phải tự khai báo:

```typescript
import { PERMISSION_SELECT } from '@/define';

export class MyDetail extends BaseDetail<MyModel> {
  protected readonly permission = PERMISSION_SELECT;
}
```

---

## Auth: `_cachedAuthVersion` — không async IDB read trong Angular effect

```typescript
// ❌ NEVER — async chain trong effect: nếu signal update 2 lần nhanh → race, chain sau đọc existing.authVersion cũ → reset về 0
effect(() => {
  const user = authUser();
  this.userAuthSession.load(user.id).then(existing => {
    this.userAuthSession.save({ ...existing, authVersion: existing?.authVersion ?? 0 });
  });
});

// ✅ ALWAYS — giữ _cachedAuthVersion in-memory, update synchronously khi restore/version header thay đổi
_cachedAuthVersion = 0;  // field trên AuthService

effect(() => {
  const user = authUser();
  const currentAuthVersion = this._cachedAuthVersion;  // sync read
  void this.userAuthSession.save({ userId: user.id, user, permissions, authVersion: currentAuthVersion });
});
```

Cập nhật `_cachedAuthVersion`:
- Khi `restoreAuthUser()`: `this._cachedAuthVersion = session.authVersion`
- Khi `X-User-Auth-Version` header lệch (trong interceptor): `authService._cachedAuthVersion = newVer` trước khi gọi `fetchAndSyncMe()`

---

## Auth: router.navigate hardcode path trong interceptor phải khớp route thật

`response-handler.interceptor.ts` (case 403/410/...) hardcode string path — không có compile-time check với `app.routes.ts`/`auth.routes.ts`. Path sai (`/auth/access` thay vì `/auth/accessdenied`, `/sys/error` thay vì `/auth/error`) rơi vào wildcard `path: '**'` → hiện nhầm trang 404 thay vì trang lỗi đúng nghĩa. Trước khi thêm/sửa `router.navigate([...])` trong interceptor → grep path đích trong `*.routes.ts` xác nhận tồn tại.

**Lỗi này KHÔNG chỉ ở interceptor — mọi `navigateByUrl('/...')` hardcode trong component đều dính.** Bug thật 2026-08-14: `customer-detail.ts` có 2 chỗ `navigateByUrl('/customer')` ở nhánh lỗi, nhưng route thật là `/crm/customer` (module CRM lồng dưới `path: 'crm'` ở `app.routes.ts`) ⇒ user gặp lỗi tải khách hàng bị đẩy vào wildcard → **trang 404**, đúng cái mà nhánh lỗi đang cố tránh. Đường điều hướng "về lại danh sách" hay bị viết thiếu prefix module vì tác giả nhớ theo tên thư mục (`modules/crm/customer`) chứ không theo path route. Quy tắc: viết xong `navigateByUrl`/`navigate` với path tuyệt đối → grep chuỗi đó trong `*.routes.ts`, hoặc thử điều hướng thật; build và `tsc` KHÔNG bắt được lỗi này.

**Case 404 của interceptor là redirect MÙ toàn trang** — `router.navigate(['/auth/notfound'])` cho *bất kỳ* API nào trả 404 (chỉ cần URL khớp `/\/(api|crm)\//i`). Trang gọi nhiều API, 1 API phụ 404 là cả trang bị ném đi. Màn hình nào có 404 *hợp lệ về nghiệp vụ* (record ngoài phạm vi scope, dữ liệu bị xoá) phải tự tắt nó bằng context token rồi xử lý ở `error` của mình:

```typescript
import { HttpContext } from '@angular/common/http';
import { SKIP_GLOBAL_404 } from '@/core/interceptors/response-handler.interceptor';

this.modelService.getDetail(id, {context: new HttpContext().set(SKIP_GLOBAL_404, true)})
```

Chọn token theo nhu cầu — KHÔNG dùng lẫn:
- `SKIP_GLOBAL_404` — chỉ tắt `navigate('/auth/notfound')`, giữ nguyên toast 401/403/409. Dùng cho request PHỤ của màn đang mở (panel chi tiết, popup, lazy block).
- `SKIP_GLOBAL_403` (thêm 2026-09-07, cùng khuôn `SKIP_GLOBAL_404`, đọc TRƯỚC `navigate('/auth/accessdenied')` trong nhánh 403 của interceptor) — dùng khi bấm vào một bản ghi qua deep-link/thông báo có thể đã hết quyền (uỷ quyền hết hạn, đổi người duyệt) mà màn vẫn đang mở hợp lệ ở phần còn lại; bắt buộc tự báo lỗi tại chỗ như `SKIP_GLOBAL_404`, không thì cũng thành hỏng câm.
- `SKIP_ERROR_NOTIFICATION` — tắt cả khối xử lý lỗi, chỉ dùng khi thật sự muốn im mọi lỗi.

Bật `SKIP_GLOBAL_404` thì **bắt buộc** hiển thị lỗi tại chỗ (`SlideOver` có sẵn `[errorMessage]` + `(retry)`), nếu không thành hỏng câm: spinner tắt, panel trắng, không ai biết vì sao.

Không mâu thuẫn với rule "không tự thêm error toast trong subscribe": rule đó áp dụng khi interceptor CÒN xử lý; tắt interceptor rồi thì bắt buộc phải tự báo, nếu không lỗi im lặng hoàn toàn.

`SKIP_ERROR_NOTIFICATION` tắt CẢ KHỐI `handleAuthError` (`response-handler.interceptor.ts:108-141`), không chỉ toast — 404 cũng có nhánh `router.navigate(['/auth/notfound'])` riêng (case 404, dòng ~196) chạy TRƯỚC bất kỳ `.subscribe({error})` cục bộ nào của caller. Service nào cố ý coi 404 là kết quả hợp lệ cần tự xử lý im lặng (vd `TeamchatService.loadChannelDetail()` — "404 vì không còn là thành viên") mà KHÔNG set context này thì cả tab vẫn bị điều hướng sang `/auth/notfound`, bất kể `error: () => {}` cục bộ viết gì — bug thật gặp phải (2026-08-30, teamchat).

---

## Deep-link từ thông báo/URL: đọc query param phải REACTIVE, không phải `snapshot`

Container đọc `route.snapshot.queryParamMap` trong `ngOnInit`/hàm khởi tạo chỉ chạy **đúng 1 lần
trong đời component**. Bấm một thông báo khác trỏ tới CÙNG route nhưng khác `?id=` khi component đã
tồn tại (Angular router tái dùng instance, không tạo lại) ⇒ deep-link lần 2 bị bỏ qua hoàn toàn, im
lặng — không lỗi, không log, chỉ đơn giản là không có gì xảy ra. Phải subscribe reactive
(`toSignal(route.queryParamMap)` + `effect`, hoặc `route.queryParamMap.pipe(...)`) để đọc lại mỗi khi
URL đổi, không chỉ lúc khởi tạo.

Biến thể liên quan: router phát `NavigationSkipped` (không phải `NavigationEnd`) khi URL trùng hệt
lần trước ⇒ bấm LẠI đúng thông báo vừa xem (đang đứng sẵn ở màn đó, cùng id) cũng không mở lại được
dù đã sửa reactive — cần thêm nonce vào URL hoặc xoá query param khi đóng dialog để bấm lại lần sau
mới có tín hiệu URL đổi.

## Auth: refresh fail ≠ logout — phân biệt transient (status 0/5xx) vs invalid (401/403)

BE dev restart bởi `air` → `/auth/refresh-token` dính `ERR_CONNECTION_REFUSED` (status 0) đúng lúc guard/interceptor cần refresh → nếu coi mọi `trySilentRefresh() === false` là "session chết" thì user bị logout oan liên tục (bug thật 2026-07-18).

Quy ước đã cài (giữ nguyên khi sửa auth flow):
- `TokenService.doRefresh`: status 0 / ≥500 → retry backoff 1s/2s/3s (re-stamp `refresh_lock` mỗi lần — tổng phải < ngưỡng stale lock 10s); CHỈ `clearToken()` khi 401/403 hoặc response body invalid.
- Caller (`authGuard`, `auth.interceptor`, `restoreAuthUser`): sau khi refresh fail, check `tokenService.hasValidRefreshSession()` — còn `true` = fail transient (doRefresh không clear) → KHÔNG clearToken/redirect login, cho đi tiếp/propagate error; interceptor sẽ tự refresh lại ở 401 kế tiếp khi BE sống dậy.

---

## `LOGOUT_KEYS` chỉ chứa dữ liệu auth theo tài khoản, KHÔNG chứa preference thiết bị

`storage.service.ts` — `clearOnLogout()` xóa toàn bộ key trong mảng `LOGOUT_KEYS`. Preference thiết bị/trình duyệt (ngôn ngữ `STORAGE_KEY_SELECT.Lang`, theme...) KHÔNG được đưa vào mảng này — nó phải sống sót qua logout. Bug thật (2026-07-23): key `lng` từng nằm trong `LOGOUT_KEYS` → chọn tiếng Anh ở màn login xong logout là bị xóa về mặc định `vi-VN`.

---

## `response-handler.interceptor.ts` tự toast 401 khi route đang là `/login`

Case 401 trong interceptor có nhánh riêng: `router.routerState.snapshot.url.startsWith('/login')` → tự `noti.error('Unauthorized', ...)`. Component nào tự xử lý lỗi login riêng (vd rate-limit countdown, custom message) mà gọi API qua interceptor này PHẢI gắn `SKIP_ERROR_NOTIFICATION` (context) cho request đó, nếu không sẽ bị toast trùng (1 cái từ interceptor, 1 cái tự code).

---

## Permission matrix (user-role.ts) hardcode đúng 5 cột — KHÔNG thêm permission thứ 6/module

`user-role.ts`/`.html` (trang gán quyền theo role) hardcode CỨNG 5 cột hành động (List/Create/Update/Extend/Delete = `type` 1-5): header viết tay 5 `<th>`, `aggChecked` computed `Record<number,boolean>={1..5}` + `for(let t=1;t<=5;t++)`. Thêm permission `type:6` vào 1 module trong `permission.define.ts` → row đó lệch cột với header + mọi module khác. Permission mới → tái dùng permission có sẵn (vd `updatePlanOrder`) nếu hợp lý, không tự tạo quyền mới type 6+ trừ khi sửa cả `user-role.ts` hỗ trợ cột động.

---

## 1 hành động dùng chung service/component từ nhiều entry point — gate quyền phải khớp Ở MỌI NƠI

Component/service dùng chung (dialog, modal) gọi API sensitive không tự mang quyền theo — quyền nằm ở nơi TRIGGER nó (route guard hoặc `*hasPerm` quanh nút mở). Nếu cùng 1 hành động (vd connect kênh omnichat) mở được từ ≥2 nơi (route riêng CÓ gate + nút tắt ở trang khác KHÔNG gate) → nơi thiếu gate là lỗ hổng thật, không phải lý thuyết.

Trước khi thêm nút/link mở dialog dùng chung → `grep` toàn bộ FE tìm mọi nơi khác đang mở component đó, đối chiếu quyền yêu cầu ở TỪNG nơi phải bằng hoặc nghiêm hơn route "chính thức" của hành động đó.

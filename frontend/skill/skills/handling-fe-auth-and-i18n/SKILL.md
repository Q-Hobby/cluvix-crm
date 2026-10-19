---
name: handling-fe-auth-and-i18n
description: >-
  Auth signals (branchActiveId, authBusinessProfile — D15: KHÔNG còn companyActiveId),
  tenant scope FE, permission check, TimezoneService, i18n key taxonomy và i18n:find.
  Dùng skill này KHI làm feature đụng permission/quyền, branch scope, thông tin thương
  hiệu, timezone/DatePicker, hoặc trước khi thêm bất kỳ i18n key mới nào.
---

# Handling FE auth and i18n

> Timezone chi tiết (DatePicker → unix timestamp, dateHmsToUnix, toTz/toUnix): `reference/fe-timezone.md`
> Auth gotchas (restoreAuthUser async, IDB missing case, permission gate): `reference/fe-patterns-gotchas-auth.md`



Tài liệu cho AI agents hiểu auth signals, tenant scope FE, và cách dùng đúng.
Đọc file này trước khi làm việc với branch selection, permissions, hoặc print.

> **(đã bỏ, D15/CRM standalone)**: hệ thống là 1 thương hiệu/DB — KHÔNG còn `companyActiveId`,
> `companyListId`, hay khái niệm "chọn công ty". Thông tin thương hiệu (tên, logo, timezone, tiền
> tệ, 2FA, IP allowlist) đọc qua signal `authBusinessProfile` (`src/app/core/services/auth/auth.state.ts`),
> nguồn BE là `pkg/brand`/bảng `business_profile` (D22, story-62/63). Trục scope duy nhất còn lại là
> `branchActiveId`/`branchListId`.

---

## 1. Auth signals — import từ `@/core/services/auth`

**File:** `src/app/core/services/auth/auth.selectors.ts`

```typescript
import {
    branchActiveId,    // computed: userProfile()?.branchActive_id
    branchListId,      // computed: authUser()?.branchId ?? []    — mảng tất cả branch
    userId,            // computed: authUser()?.id
    userFullname,      // computed: userProfile()?.fullname
    isLoggedIn,        // computed: !!authUser()
    isSuperUser,       // computed: role in SUPER_ROLES whitelist
    isDevelop,         // computed: role === 'develop'
    isAdmin,           // computed: role === 'admin'
    hasPermission,     // fn(name): boolean — check permission string
    hasAnyPermission,  // fn(...names): boolean
    hasAllPermissions, // fn(...names): boolean
} from '@/core/services/auth';
```

**State signals (low-level, ít dùng trực tiếp):**
```typescript
import { authUser, authBusinessProfile, userPermissions } from '@/core/services/auth';
```

---

## 2. `authBusinessProfile` — hồ sơ thương hiệu (thay `companyActiveId`/`companyListId` cũ)

| Signal | Trả về | Ý nghĩa |
|---|---|---|
| `authBusinessProfile()` | `BusinessProfileModel \| null` | Hồ sơ thương hiệu duy nhất của DB (tên, logo, timezone, tiền tệ...) |
| `branchActiveId()` | `number \| undefined` | Branch user đang chọn (active context) — có thể chưa chọn |
| `branchListId()` | `number[]` | Tất cả branch user có quyền — mảng rỗng nếu chưa login |

### Dùng đúng

```typescript
// ✅ Scope SELECT theo active context
tenant.ScopedDBNullBranch(c, h.DB, "table")  // BE lo (đọc từ context, AuthResponseHeaders set từ user_profile)

// ✅ FE: pre-fill form mặc định
this.initialValue = {
    branch_id: branchActiveId(),
};

// ❌ NEVER — dùng để auto-fill khi INSERT (xem B.1.1 CLAUDE.md)
// branch_id của record phải từ FE picker, không tự derive từ active context
```

---

## 3. Tenant scope trong component FE

### Pre-fill form — OK

FE pre-fill `branch_id` từ active context để tiện UX. User vẫn có thể đổi qua picker.

```typescript
// ✅ OK — pre-fill mặc định, user đổi được
this.initialValue = {
    ...this.initialValue,
    branch_id: branchActiveId(),
};
```

### (đã bỏ, D15/CRM standalone) Company picker trong form — toàn bộ pattern đã xoá

Trước D15, mục này còn 3 pattern lớn cho `company_id` trong form: "Root entity không picker, seed theo
thương hiệu active" (`BaseSignalForm.seedCompanyId`, `CompanyFormControl`, gate BE `company_active_required`),
"Record con derive company từ record cha" (`FormAppointment` seed `company_id` từ `customer.company_id`),
và "IndexedDB cache key theo `companyActiveId()-branchActiveId()`". Verify 2026-09-28: `seedCompanyId`,
`CompanyFormControl`, và field `company_id` trong các form trên **không còn tồn tại** trong
`frontend/src/app/` — company đã xoá hoàn toàn khỏi FE (`tenantKey` kiểu `companyActiveId()-branchActiveId()`
cũng không còn xuất hiện trong code — verify trước khi viết lại pattern IndexedDB key mới, đừng suy
từ đoạn lịch sử này). Giữ đoạn trên làm lịch sử/tham khảo nếu cần đọc code cũ, KHÔNG áp dụng cho code mới.

---

## 4. RBAC / Permissions

**KHÔNG dùng `ngx-permissions`/`NgxPermissionsService`/`NgxPermissionsModule`** — đã gỡ khỏi package.json (2026-07-05).
Cơ chế permission DUY NHẤT là signal-based (`userPermissions()` + `isSuperUser()`), lộ ra qua `hasPerm` directive / `hasPermission()` fn.
`BaseApp.permissionsService` vẫn còn (nhiều component cũ gọi `this.permissionsService.getPermission(x)`) nhưng giờ là `PermissionsService` (`share/services/permissions.service.ts`) — shim mỏng gọi `hasPermission()`, KHÔNG phải ngx-permissions.

**Permission tính năng (`viewX`/`createX`...) CHỈ enforce ở FE** — BE (`pkg/define/rbac.go`) chỉ chặn super-role + tenant scope, KHÔNG có middleware kiểm quyền per-endpoint. ⇒ guard FE bypass được qua gọi API trực tiếp; kiểm soát dữ liệu thật vẫn ở BE (`ScopedDBNullBranch`).

Quyền MỚI tạo: **gán cho role TRƯỚC, thêm `*hasPerm`/`requirePerm` SAU**. Gate FE là chặn thật, mà quyền mới chỉ super role tự có ⇒ thêm gate trước khi gán = khoá tính năng với 100% user thường.

```typescript
// Template — directive tên `hasPerm` (KHÔNG phải `hasPermission`)
<button *hasPerm="permission.permissionCreateCustomer">Tạo KH</button>

// TS — check trước khi action
if (!hasPermission('deleteCustomer')) {
    this.noti.warnMsg('Bạn không có quyền xóa');
    return;
}

// Super user bypass tất cả
if (isSuperUser()) { /* admin toàn quyền */ }
```

**Format permission name:** camelCase phẳng theo `PERMISSION_SELECT` (`@/define`) — vd `viewCheckUp`, `createCheckUp`, `deleteCustomer`. KHÔNG phải dạng `module.resource.action`.

**Tên hằng ≠ raw value** (vd `permissionViewLabo` = `'viewLaboOrder'`). Mọi chỗ ghi quyền dạng chuỗi (`route_perm_map.json`, migration SQL, mobile `kPerm*`, Dev Notes story) lấy raw value từ `permission.define.ts`, KHÔNG suy từ tên hằng. `TestRoutePermMapPermsInCatalog` chỉ bắt key không tồn tại, không bắt nhầm sang key có thật.

`PERMISSION_MATRIX` (catalog cho màn phân quyền role) là const top-level, evaluate 1 LẦN lúc module import — KHÔNG thể gate theo signal runtime (`branchActiveId()`...) vì sẽ đóng băng ở giá trị lúc app khởi động (thường null/0, trước khi auth xong). Cần ẩn/hiện theo runtime → đổi thành function gọi lại mỗi lần dùng, hoặc bỏ gate (luôn hiện đủ catalog).

`PERMISSION_SELECT` = tập quyền ĐẦY ĐỦ dùng gating; `PERMISSION_MATRIX` là **subset** hiển thị ở màn gán role + sync xuống `rbac_auth_item` (BE catalog). Sync/seed catalog phải lấy nguồn từ **SELECT**, KHÔNG chỉ MATRIX — nếu không sẽ prune/thiếu quyền defined-nhưng-không-ở-matrix (vd `viewTool`, `configDevelop`).

**CRITICAL — `*hasPerm` constant: verify qua `route_perm_map.json`, đừng copy sibling module.** Nhiều module `config-emr/*` từng gate nút bằng `permissionCreateBroker`/`UpdateBroker` (copy-paste từ 1 scaffold không liên quan) trong khi BE khai đúng (nay là `manageConfig` cho mọi route module Cấu hình) (`backend/internal/modules/webapp/auth/rbac/seed/route_perm_map.json`). Trước khi wire `*hasPerm`, grep route đích trong file đó lấy đúng field `perm` — không suy ra từ module cùng cây thư mục.

**Route `canActivate: [requirePerm(...)]` khi di dời UI giữa section (vd CRM → Config) phải dùng đúng permission family của resource, KHÔNG dùng permission chung của section chứa nó.** Di dời `psych-quiz` từ CRM sang Config từng gán nhầm guard `permissionViewConfigClinic` (quyền chung Config-Clinic) thay vì `permissionViewPsychQuiz` (quyền thật của resource). ⚠ Cập nhật 2026-09-02: riêng `psych-quiz` **dưới `/config`** nay CỐ Ý dùng `manageConfig` vì BE `/api/config/psych-quiz*` đã gộp về key đó — `*PsychQuiz` chỉ còn cho `/api/crm/psych-session*`. Nguyên tắc không đổi: lấy permission từ `route_perm_map.json` của **endpoint màn đó gọi**, không suy từ section chứa nó — user có quyền Clinic nhưng không có quyền PsychQuiz vẫn vào được trang vì list bên trong không tự chặn theo permission riêng khi load (chỉ `*hasPerm` ẩn nút CRUD).

**`CustomerDetail` — CHỦ ĐÍCH chỉ gate `viewCustomer` ở route cha, KHÔNG gate view theo tab (user chốt 2026-08-14).** `/crm/customer/detail/:id` chỉ có `requirePerm(permissionViewCustomer)` trên path `customer` (`crm.routes.ts:27`); route `detail/:id` (`customer.routes.ts`) không guard riêng, thừa hưởng cha. Vào được trang = xem hết 13 tab + data. **KHÔNG thêm `*hasPerm` view cho tab, KHÔNG thêm permission check vào GET API con** — quyết định nghiệp vụ, không phải nợ kỹ thuật; đừng "dọn cho đồng nhất". Quyền CUD trong tab (`createPayment`, `deletePayment`, `createTreatmentCourse`...) giữ nguyên: `viewCustomer` chỉ quy định quyền XEM. **2 ngoại lệ cố ý giữ**: tab 7 "Hình ảnh" gate `permissionViewImage` ở CẢ template LẪN `visibleTabValues()` (`customer-detail.ts:284` — sửa 1 chỗ thôi thì vào bằng `?tab=7` vẫn lọt, đây là pattern chung cho mọi tab gate bằng quyền); nút xem chi tiết phiên trắc nghiệm tab 13 gate `permissionViewPsychQuiz` (gate điều hướng sang trang khác, không phải gate list). Tab 3/9/12/13 ẩn/hiện theo **chuyên khoa** (`isDental`/`isCheckUpGeneralVisible`/`isPsychiatryVisible`), KHÔNG phải permission — đừng nhầm. Bối cảnh đầy đủ + các trang detail khác vẫn ở trạng thái gap: repo cha `../skill/skills/backend-hard-rules/reference/security-patterns.md` (mục "Field lock / permission").

---

## 5. Token storage — rules

```typescript
// ✅ access_token: AES-GCM encrypted trong localStorage (SecureStorageService)
// ✅ refresh_token: HttpOnly Secure SameSite=Lax cookie — FE không đọc trực tiếp

// ❌ NEVER
tokenService.getRefreshToken()  // method đã xóa
console.log(token)              // NEVER log token
url + '?token=' + token        // NEVER attach vào URL

// Kiểm tra refresh session còn hạn
tokenService.isRefreshTokenExpired()  // dùng cái này
```

**Device fingerprint** (`core/services/auth/fingerprint.ts`): `getDeviceFingerprint()` — SHA-256 base64url các thuộc tính browser ổn định (KHÔNG gồm screen/devicePixelRatio → tránh sai khi cắm màn ngoài), cache in-memory. Gửi ở login (`login.ts`) + refresh (`token.service.doRefresh`) cho device binding BE. Quản lý thiết bị: dialog `UserDevices` trong menu profile (`GET/POST /auth/devices`).

---

## 6. `authGuard` — protect routes

```typescript
// app.routes.ts
{
    path: 'crm',
    component: AppLayout,
    canActivate: [authGuard],
    loadChildren: () => import('@/modules/crm/crm.routes'),
}

// Print routes cũng có authGuard
{
    path: 'print',
    component: PrintLayout,
    canActivate: [authGuard],
    loadChildren: () => import('@/prints/prints.routes'),
}
```

`authGuard`: check token valid → nếu không → navigate `/login?reason=...`  
`loginGuard`: nếu đã login → redirect về `/`

Route trung gian sau login (như `/syncing`) PHẢI loại khỏi `redirect` query param trong `toLogin()` — giống `isLoginRoute` đã làm cho `/login`. Thiếu → nếu token hết hạn đúng lúc user ở route trung gian, redirect-back tạo bounce loop qua chính route đó trước khi tới đích thật. Pattern: `isSyncingRoute()` trong `authGuard.ts`.

---

## 7. `AuthService` — methods chính

```typescript
const authService = inject(AuthService);

authService.login(params)                // → set authUser signal + save IndexedDB + auth_user_id localStorage
authService.logout()                     // → clear tokens + authUser + IndexedDB
authService.restoreAuthUser()            // → async, await trong APP_INITIALIZER — restore từ IndexedDB (fallback localStorage)
authService.fetchAndSyncMe()             // → gọi /api/auth/me, set signals, fire-and-forget safe
authService._cachedAuthVersion           // → in-memory authVersion — dùng trong effect, KHÔNG đọc IDB async
```

---

## 8. Auth state lifecycle

```
App bootstrap (APP_INITIALIZER — chạy SONG SONG, KHÔNG sequential):
    initializeAuth:      tokenService.initialize() → restoreAuthUser() → authInitialized.set(true)
    initializeTranslations: load i18n
    preloadIndexedDb:    ensureSyncedIfEmpty() → refreshCompanyActive() + refreshCallCenterActive()
    router (internal):   withEnabledBlockingInitialNavigation() — navigation bắt đầu NGAY khi bootstrap
                         → authGuard đợi authInitialized trước khi pass

Login flow
    → POST /auth/login → tokens + user + permissions
    → userAuthSession.save(IDB)
    → storageService.saveStorage('auth_user_id', userId)
    → navigate('/')

Logout flow
    → clearAllAuthData() — clear IDB + localStorage
    → window.location.href = '/login'

Tab sync (StorageEvent 'auth_user_id')
    → other tab login (oldValue null → newValue) → reload
    → other tab logout (oldValue → newValue null) → reload

Token revoke → 401 → silent refresh → fetchAndSyncMe() → authUser signal update
X-User-Auth-Version lệch → fetchAndSyncMe() → authUser signal update
```

**CRITICAL — APP_INITIALIZER chạy song song:** `preloadIndexedDb` bắt đầu cùng lúc với `initializeAuth`. `restoreAuthUser()` phải `await` — → Xem `frontend/skill/skills/handling-fe-auth-and-i18n/reference/fe-patterns-gotchas-auth.md`. `preloadIndexedDb` dùng `hasValidRefreshSession()` (đọc localStorage) thay vì `hasValidToken` vì `initialize()` chưa chạy xong khi check.

**CRITICAL — withEnabledBlockingInitialNavigation race:** Router navigation chạy song song với APP_INITIALIZER. `requirePerm` guard có thể chạy trước khi `userPermissions` được populate. `authGuard` đợi `authInitialized` signal để chặn race này. Xem `skills/handling-auth-and-workspace/SKILL.md §authInitialized`.

---

## 9. Signals trong component — pattern chuẩn

```typescript
// ✅ Đọc signal trong template
{{ branchActiveId() }}
@if (isLoggedIn()) { ... }

// ✅ Đọc signal trong TS (không cần async)
const bid = branchActiveId();
if (!bid) return;

// ✅ Reactive với effect
effect(() => {
    const bid = branchActiveId();
    if (bid) this.loadDataForBranch(bid);
});

// ✅ Computed từ auth signal
readonly canDelete = computed(() =>
    hasPermission('crm.customer.delete') || isSuperUser()
);
```

---

## 10. Menu component reactive với permission changes

Tất cả `*menu.ts` extend `BaseApp`. Pattern chuẩn để sidebar tự rebuild khi admin đổi role:

```typescript
import { effect } from '@angular/core';
import { userPermissions } from '@/core/services/auth/auth.state';

export class XxxMenu extends BaseApp {
  model: any[] = [];

  constructor() {
    super();
    effect(() => this.buildMenu()); // re-run mỗi khi userPermissions signal thay đổi
  }

  // KHÔNG dùng ngOnInit() + implements OnInit

  private buildMenu(): void {
    const per = Array.from(userPermissions()); // ✅ SIGNAL
    this.model = filterMenuByPermission([...], per);
  }
}
```

Nếu menu cần đợi thêm signal khác (vd translations):

```typescript
constructor() {
  super();
  effect(() => {
    if (!this.translationsReady()) return; // guard signal trước
    this.buildMenu();
  });
}
// ngOnInit chỉ set translationsReady.set(true), không gọi buildMenu()
```


## Define files

```typescript
// ❌ NEVER (legacy)
{ value: 1, label: 'Chưa xử lý' }

// ✅ Giữ `label` để backwards-compat, ADD `labelKey` cho code mới
{ value: 1, label: 'Chưa xử lý', labelKey: 'define.customer_status.pending' }
```

## Gotcha: `_MAP` reduce ẩn `labelKey`

`*_MAP = *_OPTIONS.reduce(..., {} as Record<number, {value,label}>)` — nếu type annotation thiếu `labelKey?: string`,
TS chặn truy cập dù object runtime (từ OPTIONS) có sẵn field này → dễ bị hardcode label tiếng Việt thay vì fix type.
Gặp ở `appointment/partner/payment/reminder/customer-care.define.ts`. Fix: thêm `labelKey?: string` vào type annotation của reduce.

## PrimeNG p-select — dùng `translateOptions()`

```typescript
import { translateOptions } from '@/share/utils/translate-options.util';
import { CUSTOMER_STATUS_OPTIONS } from '@/define';

protected readonly customerStatusOptions = translateOptions(CUSTOMER_STATUS_OPTIONS);
```

```html
<p-select [options]="customerStatusOptions()" optionLabel="label" optionValue="value"/>
```

`translateOptions()` trả `Signal<T[]>`. Lang change → signal re-emits → re-render. Fallback: nếu chưa có `labelKey`, giữ nguyên `label`.

## Direct template rendering

```html
<!-- ✅ CHUẨN — smart fallback -->
<span>{{ (item.labelKey || item.label) | translate }}</span>
<span>{{ (model.nameKey || model.name) | translate }}</span>

<!-- Constant key -->
<span>{{ 'define.permission.action.list' | translate }}</span>

<!-- ❌ DEPRECATED -->
<span>{{ item.labelKey ? (item.labelKey | translate) : item.label }}</span>
```

TS code: `this.translate.instant(item.labelKey || item.label)`.

`translate.instant(key)` thiếu bản dịch ⇒ trả NGUYÊN chuỗi key (không `''`/`undefined`) ⇒ `instant(k) || fallback`
KHÔNG BAO GIỜ fallback. Cần fallback: `const t = instant(k); return t === k ? fallback : t`.

`TranslatePipe` có trong `ListShareImports` — không cần wire thêm.

- KHÔNG ghép i18n key từ giá trị dữ liệu runtime (`'prefix.' + code`) — dùng key do BE khai sẵn trong payload, fallback về chính giá trị đó khi bảng tra chưa tải xong. Ghép chuỗi vỡ **câm** ngay khi quy ước 2 bên lệch nhau một ký tự (`-` vs `_`).

## displayCols / data structures

```typescript
// ❌ DEPRECATED
{ label: 'Họ tên', value: 'fullname' }

// ❌ AVOID
{ label: 'CUSTOMER_CODE', value: 'code' }

// ✅ CHUẨN
{ label: 'Họ tên', labelKey: 'common.label.fullname', value: 'fullname' }
```

## Layout menu / dialog header

```typescript
// ❌ NEVER
{ label: 'Khách hàng', icon: 'pi pi-users', routerLink: ['/crm/customer'] }
this.dialog.open(Form, { header: 'Cập nhật khách hàng' });

// ✅ ALWAYS
{ label: 'Khách hàng', labelKey: 'layout.menu.customer', icon: 'pi pi-users', routerLink: ['/crm/customer'] }
this.dialog.open(Form, { header: this.translate.instant('crm.customer.update_dialog_title') });
```

## ⛔ Tạo key dịch mới — BẮT BUỘC check value trước (chống trùng)

**Quy trình bắt buộc TRƯỚC khi thêm bất kỳ key dịch nào:**

1. **Tra value đã tồn tại chưa:**
   ```bash
   npm run i18n:find -- "Thanh toán"          # khớp chính xác
   npm run i18n:find -- --contains "thanh"     # khớp chứa
   ```
2. **Nếu CÓ key trùng value (exit 3):**
   - Cùng nghĩa → **DÙNG LẠI key đó**, KHÔNG tạo mới.
   - Tool nhóm theo nghĩa EN: nếu có nhiều nghĩa (vd Payment vs Billing) → chọn key ĐÚNG ngữ cảnh.
3. **Nếu CHƯA có (exit 0)** → tạo key mới theo taxonomy:
   - Dùng chung ≥2 module → `common.<label|action|status|message|tooltip|placeholder|validation>.<slug>` (vd `common.action.save`, `common.label.customer`, `common.status.paid`)
   - Riêng 1 module → `{module}.{feature}.<slug>` (vd `crm.customer.fullname`)
   - Menu items → `layout.menu.*`
   - Add ĐỦ 4 locale cùng path: `vi-VN` + `en-US` + `ja-JP` + `fil-PH`.

**1 nghĩa = 1 key**: VI trùng nhưng khác nghĩa EN (vd "Thanh toán" = Payment vs Billing) → key RIÊNG theo nghĩa, KHÔNG gộp.

**KHÔNG được:** tạo key mới khi value đã tồn tại · đặt key trùng nghĩa ở module khác nhau · hardcode string trong template/TS.

### Tooling
- `npm run i18n:find -- "<value>"` → tra ngược theo value TRƯỚC khi tạo key (exit 3 = đã có, reuse)
- `node scripts/i18n-dedup.mjs` → `i18n-dedup/report.json` (group trùng + usage)
- `node scripts/i18n-build-mapping.mjs` → `i18n-dedup/mapping.json` (oldKey→canonical)
- `node scripts/i18n-apply.mjs <safe|risky|all> [--step=add|rewrite|prune|all]` — áp dụng
- Protected dynamic prefix (codemod KHÔNG được đụng): `define.emr.record_type.*`

## i18n key audit gate

Pre-commit hook (`.husky/pre-commit`) chạy 2 gate:
1. `npm run i18n:check -- --fail-on-missing` — **BLOCK** nếu thiếu key.
2. `npm run i18n:dup` — **CẢNH BÁO** khi commit thêm key MỚI trùng value cùng nghĩa (so staged vs HEAD; đồng âm khác nghĩa EN được bỏ qua). Mặc định không chặn; `I18N_DUP_STRICT=1` để chặn cứng.

```bash
npm run i18n:check          # human-readable summary
npm run i18n:check:verbose  # full lists
npm run i18n:check:json     # JSON cho CI
```

Khi PR fail i18n gate:
1. Xem MISSING keys (file path + key name)
2. Trước khi add key mới hàng loạt: nếu số lượng MISSING gần bằng số ORPHAN cùng module → nghi ngờ đây là rename sót (prefix đổi ở JSON nhưng code chưa cập nhật, hoặc ngược lại). So sánh suffix-set (bỏ prefix) giữa 2 danh sách — khớp 1:1 → sửa code dùng đúng prefix JSON hiện có, KHÔNG tạo key trùng mới.
3. Add đủ 4 locale cùng nested path (gate chỉ check `vi-VN`/`en-US`)
4. Re-run `npm run i18n:check` → confirm 0 missing

Key trả về GIÁN TIẾP qua hàm thuần (vd `resolveXxxKey()` return string key, không phải literal `'key' | translate` trong template) → `i18n:check` báo ORPHAN SAI (false-positive) vì tool chỉ static-scan literal — đừng xoá key chỉ vì nằm trong danh sách ORPHAN, grep thử usage gián tiếp trước.

`git commit --no-verify` chỉ dùng emergency — fix keys luôn là đúng.

## Nguồn file

- **Key i18n mới HOẶC sửa value key có sẵn: ghi CẢ 2 nơi × 4 locale** — `frontend/src/assets/i18n/*.json` (nguồn `ng build`, repo `frontend/` riêng, gitignored khỏi repo chính) VÀ `public/assets/i18n/*.json` (artifact đang serve, được git track ở repo chính). Chỉ sửa `src` → runtime thiếu key tới lần build kế; chỉ sửa `public` → key MẤT khi `ng build` regenerate. hr-completion + hr-2026-compliance đều dính lệch 2 dir (mỗi key chỉ nằm 1 nơi) — verify bằng grep key ở CẢ 2 dir. ⚠ Hai gốc là `frontend/src/assets/i18n/` và `public/assets/i18n/` ở **repo GỐC** — `frontend/public/assets/i18n/` **KHÔNG tồn tại**; grep nhầm ở đó trả rỗng rồi kết luận "chỉ có 1 nơi" (đã xảy ra 2026-08-31, dẫn tới một đính chính SAI ghi vào sổ nợ). Đối chứng: grep từ gốc repo với CẢ 2 đường dẫn + một key đã biết chắc có.
- **Sửa bằng script: KHÔNG `json.load()` rồi `json.dump()` lại** — dump không giữ thứ tự key gốc, diff nhảy lên hàng nghìn dòng (re-sort toàn file thay vì chỉ thêm key mới). Chèn text-based: regex tìm dòng key láng giềng đã biết, insert ngay sau, giữ nguyên phần còn lại của file.

## Loader
- Prod: `StaticTranslateLoader` (vi-VN inline main.js, en-US lazy chunk)
- Dev: `CachedTranslateLoader` — stale-while-revalidate qua localStorage (`i18n_<lang>`), sửa JSON KHÔNG hiện ngay lập tức. Cần reload 2 lần (lần 1 kích hoạt fetch nền update cache, lần 2 mới đọc cache mới) hoặc `localStorage.removeItem('i18n_<lang>')`. `LS_VERSION` (auto-invalidate cache) chỉ đổi khi `npm run build` (hook `prebuild`), KHÔNG đổi khi `ng serve`/hot-reload.
- `fallbackLang: 'vi-VN'` → missing en key fallback VN thay vì raw key

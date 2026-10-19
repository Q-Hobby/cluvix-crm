---
name: handling-auth-and-workspace
description: >-
  Dual authentication CRM/Client (JWT chỉ chứa uid, 2FA TOTP, device limit qua rts:{uid}
  zset) và các module config liên quan (IndexedDB master-data sync). Dùng skill này KHI đụng login/logout/refresh-token/2FA,
  session/device management, trước khi tái tạo helper đọc company/username từ JWT (đã
  xoá 2026-07-04 — PHẢI dùng GetBranchActiveID; company đã xoá hoàn toàn D15/story-41),
  hoặc khi thêm bảng IndexedDB mới ở FE.
---

# Auth & Workspace

> reference/indexeddb.md: IndexedDB master-data sync phía BE.


## Tổng quan
2 hệ thống auth hoàn toàn tách biệt: CRM (nhân viên) và Client (đối tác bên ngoài).

## CRM Auth (Nhân viên)

### Flow
```
POST /api/crm/auth/login   { username, password, device_fingerprint }
  → BE: validates credentials
  → BE: upsert user_device (device binding — xem §Device binding); đóng fingerprint vào refresh token
  → BE: Set-Cookie: refresh_token=<opaque>; HttpOnly; Secure; SameSite=None; Path=/api/auth; Max-Age=...
       (HTTP dev: SameSite=Lax + Secure=false — None yêu cầu Secure=true)
  → Response body: { accessToken, expiresAt, refreshExpiresAt }   ← NO refreshToken in body
  → FE: TokenService.setTokens(accessToken, expiresAt, refreshExpiresAt)
       stores encrypted access_token (AES-GCM) + refresh_token_expires_at in localStorage
       refresh_token cookie managed entirely by browser

POST /api/crm/auth/refresh-token   { device_fingerprint }
  → Browser auto-sends refresh_token cookie (withCredentials: true, default in ApiService)
  → BE: reads c.Cookie("refresh_token"), verify device fingerprint theo DEVICE_BINDING_MODE, rotates, sets new cookie
  → Response body: { accessToken, expiresAt, refreshExpiresAt }   ← NO refreshToken in body

POST /api/crm/auth/logout   ← PUBLIC route (không qua JWTAuth)
  → Hoạt động cả khi access token đã hết hạn — bắt buộc để consume refresh cookie
  → Prod: reject Sec-Fetch-Site: cross-site (403) — chống CSRF forced-logout; dev skip (FE/BE khác site)
  → BE: revoke access jti (best-effort nếu có Bearer, truyền c.ClientIP() cho IP binding) + consume refresh + clear cookie (Max-Age=-1)
  → FE: TokenService.clearToken() wipes localStorage timestamps

POST /api/crm/auth/logout-all (authed)
  → RevokeAllSessions + RevokeAllRefreshTokens — PHẢI revoke cả 2 (xem §Security)

Request → middleware.JWTAuth(tokenSvc) → decode JWT + check revoked:{jti} (Redis) → set userID
       → middleware.AuthResponseHeaders() → loadUserScope() → set branchActiveId, branchIds, locale
```

### JWT Claims
```go
type Claims struct {
    UID uint `json:"uid"`
}
// KHÔNG có branch_id/username trong JWT — branch scope lấy từ UserScope
// (GetBranchActiveID). GetCompanyID đã xóa 2026-07-04, KHÔNG tái tạo. D15/story-41:
// company đã xoá hoàn toàn khỏi hệ thống (1 thương hiệu/DB) — không còn company_id/
// companyActiveId/companyIds ở bất kỳ tầng nào.
```

### Context getters
- `GetUserID(c)` → uint
- `GetBranchActiveID(c)` → uint
- `GetBranchIDs(c)` → []uint

### UserScope (cached Redis 5 phút)
- Load từ `user` (driving) LEFT JOIN `user_profile` → branchActive_id, branchIds (JSON), locale, **status**
- `InvalidateUserScope(userID)` — gọi khi user đổi branch/status
- `AuthResponseHeaders` abort 401 nếu `scope.Status != define.UserStatusActive` — enforce per-request, không chỉ lúc login
- Cache cleared khi admin update user → request ngay sau re-read DB → enforce ngay không cần logout

### Security
- **Device limit thật = giới hạn refresh token** (2026-07-04): zset `rts:{uid}` (rid → expiry) index refresh token/user; login vượt `TOKEN_MAX_SESSIONS` (default 3) → evict refresh của thiết bị ít-refresh-nhất (LRU) → thiết bị đó chết trong ≤ AccessTTL. Giới hạn access-session một mình VÔ NGHĨA (bị revoke thì silent-refresh sống lại)
- **Logout dùng `ConsumeRefresh` KHÔNG `VerifyAndRotateRefresh`** — rotate lúc logout tạo orphan token chiếm 1 slot device đến hết RefreshTTL
- Token revocation (jti blacklist), IP binding (optional)
- **Revocation enforce tại `JWTAuth(tokenSvc)`** (2026-07-04): check `revoked:{jti}` mỗi request — fail-open khi Redis lỗi
- **TTL blacklist = phần đời còn lại của token** (lấy từ `sess:{uid}` zset score) — TTL cố định ngắn hơn AccessTTL sẽ mở resurrection window (token revoke xong sống lại)
- **Revoke access PHẢI kèm revoke refresh (CRITICAL)**: chỉ `RevokeAllSessions` → thiết bị khác 401 thoáng qua rồi silent-refresh lấy token mới → revoke vô dụng. LUÔN gọi cặp `RevokeAllSessions(uid)` + `RevokeAllRefreshTokens(uid)` (logout-all, đổi mật khẩu, đổi role/status)
- refresh_token: HttpOnly Secure SameSite=None cookie (Lax trên HTTP dev), Path=/api/auth (Cấp 2, implemented 2026-04-26)
- access_token: AES-GCM encrypted at rest in localStorage (Cấp 1)
- Login rate limit: 10 req/IP/5min via Redis INCR
- **Trusted proxies (CRITICAL)**: `r.SetTrustedProxies(cfg.TrustedProxies)` — gin default tin MỌI proxy → `ClientIP()` spoof được qua X-Forwarded-For. Set `TRUSTED_PROXIES` đúng khi sau reverse proxy, nếu không rate-limit/IP-binding/seclog đều dựa IP giả. Chiều ngược lại cũng chết: sau nginx mà `.env` TRỐNG ⇒ `ClientIP()` = 127.0.0.1 cho MỌI request ⇒ rate limit key theo IP thành 1 bucket toàn cục (livechat 2026-09-03). Rate limit mới theo IP phải key thêm tenant/site và ghi tiền điều kiện này
- **IP binding** (`TOKEN_IP_BINDING=true`): `buildAccessJWT` nhét `ipHash=sha256(ip)` vào access token; `JWTAuth` so với `sha256(ClientIP)` mỗi request → 401 khi lệch. User di động đổi IP → 401 → interceptor silent-refresh → token rebind IP mới (không logout). Opt-in, default off
- **Token khác access (challenge 2FA, one-time...) PHẢI ký key riêng** (vd `sha256(Secret+":purpose")`) — `JWTAuth` verify chữ ký + expiry + `revoked:{jti}` + issuer/audience (`token.ClaimsMatchTenant`, issuer/aud chứa DB_NAME từ 2026-10-18 — chặn token chéo khách) nhưng KHÔNG phân biệt mục đích token; ký chung `TOKEN_SECRET` → token đó dùng được làm Bearer access token (challenge không có jti → bỏ qua cả revocation check) = bypass. Test guard: `pkg/token/challenge_isolation_test.go`

### IP allowlist nhân viên (Hướng 2A — khác IP binding)
- **`IP_ALLOWLIST_MODE=off|log|enforce`** (`SetIPAllowlistMode` ở boot). Chặn nhân viên theo `allowed_ips` (IP/CIDR) — KHÁC IP binding (bind token vào 1 IP): allowlist là danh sách IP/dải được phép
- **Cột `allowed_ips` JSON** trên `business_profile` (D15/story-62, qua `pkg/brand`; trước đây trên `company`) + `user`. **Effective = user override > hồ sơ thương hiệu** (user non-empty thắng; rỗng cả hai = không giới hạn). Load vào `UserScope.AllowedIPs` (cache 5 phút → đổi allowed_ips propagate ≤5 phút hoặc `InvalidateUserScope`)
- **Enforce trong `AuthResponseHeaders`** (sau khi set branchActiveId): `helpers.IPInAllowlist(ClientIP, list)` (exact IP + CIDR). enforce → 403 `ip_not_allowed`; log → chỉ seclog (throttle 1 lần/(user,ip)/5phút chống flood)
- **Super-user LUÔN bypass** (`UserScope.IsSuper` = EXISTS role trong `define.SuperRoles`) — cấu hình sai không tự khóa admin
- **PHỤ THUỘC trusted proxies** — ClientIP phải thật
- FE: field `allowed_ips` (textarea IP/CIDR mỗi dòng) trong form hồ sơ doanh nghiệp (`authBusinessProfile`) + user; cảnh báo lockout. `helpers.IPInAllowlist` cũng nâng `checkPartnerIP` (partner_auth) từ exact match lên CIDR
- Test: `pkg/helpers/ipmatch_test.go` (exact/CIDR/IPv6/edge)

### Device binding (fingerprint)
- **Bảng `user_device`** (auth-level, KHÔNG company_id): `(user_id, fingerprint)` UNIQUE, status trusted/pending/revoked. Nền cho 2FA/WebAuthn per-device sau này
- FE gửi `device_fingerprint` (SHA-256 base64url, `fingerprint.ts`, KHÔNG gồm screen — đổi khi cắm màn ngoài) ở login + refresh
- Fingerprint đóng vào refresh token value: `rt:{uid}:{rid}` = `<hash>` (legacy) hoặc `<hash>:fp:<fingerprint>`
- **`DEVICE_BINDING_MODE=off|log|enforce`**: enforce → refresh với fp lệch storedFP = 401; log → chỉ seclog `refresh_fp_mismatch` (đo tỷ lệ trước khi bật enforce); token legacy (không fp) luôn bỏ qua, tự nâng cấp ở rotation kế
- API self-service: `GET /auth/devices`, `POST /auth/devices/:id/revoke` — CHỈ thiết bị của chính user (`GetOwned` scope theo user_id); FE ở menu profile
- **Upsert throttle**: `Dao.Upsert(..., minTouchInterval)` — login truyền 0 (luôn ghi), refresh truyền 1h (bỏ qua write nếu last_seen còn mới) → giảm tải hot-path refresh
- **Dọn nền**: `user_device_cleanup` (`helpers.RunBackground` ở bootstrap, 24h) xoá device `last_seen` quá `define.UserDeviceStaleDays` (90) — chống phình bảng
- **`pkg/seclog`**: structured security events (`sec_event` field) — login_success/failed, new_device, refresh_fp_mismatch, access_ip_mismatch, device_evicted/revoked, revoked_token_reused, ip_not_allowed. **`Event` no-op khi `logger.L` nil** (test/boot sớm)
- **Test**: `pkg/token/token_device_test.go` (miniredis) — fingerprint binding enforce/log, legacy upgrade, device limit cap, ConsumeRefresh, RevokeRefreshByFingerprint, ActiveFingerprints, revocation blacklist

### Audit log (Hướng 2B — persist seclog vào DB)
- **`seclog.InitSink(buf, flush)`** (wire ở bootstrap): `Event` đẩy `Entry` vào buffered channel → goroutine batch-INSERT (`CreateInBatches`). **Non-blocking**: channel đầy → drop + `seclog.Dropped()` đếm, KHÔNG chặn request. Flush do module audit cấp (seclog không import DB/DAO)
- `Event` promote key `user_id`/`username` sang cột, còn lại vào `detail` JSON.
- **Bảng `security_audit_log`** (auth-level, TOÀN DB — D15/story-58 đã bỏ hẳn cột `company_id`, 1 thương hiệu/DB nên không cần scope company nữa): index `(user_id,created_at)`, `(event,created_at)`
- **API `GET /auth/audit-log`**: super-user only (`RbacSvc.IsSuperUser` — config module không có perm middleware), KHÔNG scope gì thêm — xem hết log của DB. paginate + filter `user_id / event (1) / events (nhóm, phẩy) / from / to`.
- **Dọn nền** `security_audit_cleanup` (`helpers.RunBackground`, 24h) xoá quá `define.AuditRetentionDays` (180)
- **FE**: trang `config/security-audit` (audit tổng) + `log/login` (lịch sử đăng nhập, module history) — cả 2 super-user, p-table lazy. `authService.getAuditLog()`
- **Test**: `pkg/seclog/seclog_test.go` — sink batch flush + field promotion + nil-logger no-panic

### Login history (chi tiết kiểm soát đăng nhập)
- `login_failed` mang `reason` trong detail: `user_not_found | bad_password | inactive | rate_limited`. **Distinction chỉ ở audit (super-user), response luôn generic `login_failed`** — chống user enumeration
- Login query bỏ `WHERE status=Active` → query theo username rồi check status (phân biệt not_found vs inactive). Rate-limit (429) giờ cũng ghi `login_failed reason=rate_limited`
- `login_failed` (user thật) + `logout` resolve `companyActive_id` sớm → hiện trong view theo company (trước đây NULL → vô hình)
- `logout` event ghi khi decode được bearer (user_id + company). Login là public route → resolve company qua query user_profile
- FE `log/login` (`modules/history/login`): gộp login_success/failed/logout/new_device/login_no_fingerprint qua `?events=`; cột reason + device. Menu gate `isSuperUser()` (không gắn permission để non-super khỏi thấy rồi 403)

### FE TokenService API (post Cấp-2 refactor)
```typescript
// ✅ Current API
tokenService.setTokens(accessToken: string, expiresAt?: number, refreshExpiresAt?: number)
tokenService.getAccessToken(): string | null   // decrypts AES-GCM from localStorage
tokenService.hasValidToken: boolean
tokenService.hasValidRefreshSession(): boolean  // checks refresh_token_expires_at timestamp only
tokenService.isRefreshTokenExpired(): boolean
tokenService.clearToken()                       // clears localStorage, cookie cleared by BE on logout

// ❌ REMOVED — do NOT use
// tokenService.getRefreshToken()     → no longer exists (cookie is HttpOnly, JS cannot read)
// tokenService.setTokens(a, b, c, d) → 4-arg form removed
```

### doRefresh — clearToken chỉ khi 401/403 (CRITICAL)

`doRefresh()` catch block **không được gọi `clearToken()` cho mọi lỗi**. Chỉ clear khi BE xác nhận token thực sự invalid:

```typescript
// ✅ CORRECT (hiện tại)
} catch (err: any) {
    const status = (err as any)?.status;
    if (status === 401 || status === 403) {
        this.clearToken();  // token thực sự hết hạn ở Redis
    }
    return false;  // network/5xx: không xóa refresh_token_expires_at
}

// ❌ NEVER — clearToken với mọi lỗi
} catch (err: any) {
    this.clearToken();  // lỗi mạng thoáng qua xóa refresh_token_expires_at
    return false;       // → isRefreshTokenExpired() = true → "session expired" giả
}
```

**Lý do:** `clearToken()` xóa `refresh_token_expires_at` khỏi localStorage. `isRefreshTokenExpired()` trả `true` khi key bị thiếu. Nếu auto-refresh timer gặp lỗi mạng và clear session, điều hướng kế tiếp sẽ báo "Refresh session expired" dù refresh token trong Redis vẫn còn hiệu lực.

**Phân biệt 3 path:**
- **auto-refresh timer** → network error: `doRefresh()` retry backoff (status 0/5xx) rồi mới trả false, không clearToken → state giữ nguyên
- **API request → 401 → refresh fail (interceptor)**: chỉ clearToken khi `hasValidRefreshSession()` = false; transient → propagate error, không logout
- **authGuard → trySilentRefresh fail**: transient (session còn) → cho điều hướng tiếp, interceptor refresh lại ở 401 kế tiếp; chỉ redirect login khi session hết

Chi tiết retry/discriminator: `frontend/skill/skills/handling-fe-auth-and-i18n/reference/fe-patterns-gotchas-auth.md` §"refresh fail ≠ logout"

### authGuard (post Cấp-2)
FE cannot read the refresh_token cookie. Guard checks expiry via timestamp only:
```typescript
// ✅ Correct check
if (tokenService.isRefreshTokenExpired()) { redirect to /login }

// ❌ Old pattern — removed
// const rt = tokenService.getRefreshToken(); if (!rt) { redirect }
```

### authInitialized — race với withEnabledBlockingInitialNavigation (CRITICAL)

`withEnabledBlockingInitialNavigation()` đăng ký APP_INITIALIZER riêng chạy **song song** với `initializeAuth` — không phải sau. Khi access token còn hạn: `initialize()` (crypto ~1ms) xong trước → `hasValidToken=true` → `authGuard` pass ngay → `requirePerm` chạy synchronous — nhưng IDB read trong `restoreAuthUser()` (~10–50ms) chưa xong → `userPermissions = Set()` rỗng → redirect `/auth/accessdenied` trên mọi deep route khi F5.

Xảy ra khi: auth storage dùng IndexedDB (async) + SW cache chunk (fast import) — cả hai cộng lại làm race đủ lớn để xảy ra deterministic.

**Fix pattern (đã implement):**
```typescript
// auth.state.ts
export const authInitialized = signal(false);

// auth.service.ts — restoreAuthUser()
try { /* ... restore logic ... */ } finally { authInitialized.set(true); }

// authGuard.ts — waitInitialized$ tạo trong injection context (toObservable không gọi được trong switchMap)
const waitInitialized$ = authInitialized()
  ? of(true as const)
  : toObservable(authInitialized).pipe(filter(Boolean), take(1), map(() => true as const));
// Dùng ở CẢ 2 path: hasValidToken = true VÀ silent-refresh thành công
// (refresh xong nhưng restoreAuthUser Case 1a còn đang fetch /me → phải đợi)
```

Overhead: chỉ xảy ra lần đầu tiên (F5 với token còn hạn). Mọi navigation sau đó `authInitialized()` đã `true` → zero overhead.

### withCredentials
`ApiService.createHttpOptions()` sets `withCredentials: true` by default — all requests
automatically include cookies. No need to pass it explicitly at call sites.

### Cấp 3 (NOT implemented — deferred)
Moving access_token to HttpOnly cookie would trade XSS risk for CSRF risk (no CSRF protection in place).
Triggers to re-evaluate: SOC2 Type II audit, PCI-DSS scope expansion, or pen-test finding.

## (đã bỏ, D15/CRM standalone) Client Auth (Đối tác — Zalo Mini App, website)

Module `internal/modules/client/` (PartnerAuth, `partners.company_id`/`companyActiveId`,
`GetPartnerCompanyID`) **KHÔNG còn tồn tại trong repo CRM standalone này** — verify 2026-09-28:
`internal/modules/` chỉ còn `webapp/` (không có `client/`/`mobile/`/`public/`), không có
`internal/middleware/partner_auth.go`. Nội dung dưới đây (flow OAuth partner, JWT `PartnerClaims`,
`GetPartnerID`/`GetPartnerCompanyID`) mô tả kiến trúc ERP cũ trước khi tách CRM standalone — giữ
làm lịch sử, KHÔNG áp dụng. Skill `building-client-and-sync-apis` đã xoá khỏi CRM standalone.

## Response format khác nhau

### CRM — `pkg/response/response.go`
- `Data(c, data)`, `DataWithMessage(c, data, msg)`, `Validate(c, msg)`, `NotFound(c)`, `ServerError(c, err)`

### Client — `pkg/response/client.go`
- `ClientSuccess(c, data, msg)` → `{success: true, code: 200, message, data, timestamp}`
- `ClientError(c, msg, code)` → `{success: false, code, message, data: null, timestamp}`
- `ClientValidate(c, msg)` → `{success: false, code: 422, message, data: null, timestamp}`

## Router
```go
api := r.Group("/api")
api.POST("/client/oauth/token", h.OAuthToken)                          // public
// CRM public: /auth/login, /auth/refresh-token, /auth/logout (xem Flow)
authed := api.Group("", middleware.JWTAuth(d.TokenSvc), middleware.AuthResponseHeaders(db, d.Cache)) // CRM
clientGroup := api.Group("/client", middleware.PartnerAuth())           // Client
```

### 2FA — TOTP (Hướng 3)
- **Login 2 bước**: `POST /auth/login` → nếu user bật 2FA + thiết bị chưa trusted → trả `{requires_2fa, challenge_token}` (CHƯA cấp token). `POST /auth/login/2fa {challenge_token, code, remember_device}` → verify TOTP/backup → cấp token. `challenge_token` = JWT 5 phút aud=`2fa-challenge` (proof đã qua password).
- **Device-aware skip**: `user_device.totp_trusted_until > now` → bỏ qua 2FA. `remember_device` set = now + `company.totp_trust_days` (0 = luôn hỏi, cho máy dùng chung).
- **Secret mã hóa** AES-GCM bằng `TOTP_ENC_KEY` (env riêng); backup codes hash sha256, 1 lần. Rate-limit verify 5 lần/5 phút.
- **Endpoints** (authed): `/auth/2fa/status|setup|verify-setup|disable`, `/auth/2fa/reset/:userId` (super-user).
- **Policy per-company + per-role**: `company.require_2fa=1` HOẶC role ∈ `company.require_2fa_roles`. Bắt buộc mà chưa enroll → `UserScope.MustSetup2FA=true` → **AuthResponseHeaders chặn mọi API (trừ `/auth/2fa/*`, `/auth/me`, `/auth/permission`) với 403 code=`2fa_setup_required`** → FE interceptor redirect `/2fa-setup`.
- **`AuthResponseHeaders` chạy các gate nối tiếp sau khi set `branchActiveId`/`branchIds`**: 2FA (`code=2fa_setup_required`, chặn mọi API trừ endpoint setup 2FA + `/me`) rồi IP allowlist (`ip_not_allowed`, mode `enforce`). ⚠ **(đã bỏ, D15)** Trước đây còn gate thứ 3 chặn khi `GetCompanyActiveID(c)==0` (`code=company_active_required`, FE redirect `/choose-company`) — đã xoá cùng company (1 thương hiệu/DB không cần chọn thương hiệu active).
- **Hai gate loại trừ nhau về cấu trúc**: `require_2fa`/`require_2fa_roles` lấy qua `LEFT JOIN company c ON c.id = p.companyActive_id` ⇒ `companyActive_id` NULL thì join rỗng ⇒ `MustSetup2FA` **luôn false**. Nghĩa là chính sách 2FA theo thương hiệu KHÔNG đọc được khi chưa chọn thương hiệu, và thứ tự thật là **chọn thương hiệu trước → 2FA sau** (không phải ngược lại). Không có lỗ hổng: giữa hai bước, mọi API nghiệp vụ đều bị một trong hai gate chặn.
- **Prefix `/api/config/onboarding/` được MIỄN gate** (DB trắng chưa có thương hiệu nào thì `switch-profile` vô dụng). Cửa này KHÔNG tự bảo vệ — handler tự kiểm super-user + bảng `company` rỗng **toàn bảng** (không `ScopedDB*`), tự đóng vĩnh viễn khi đã có company. `bootstrap` tạo company/branch + gán active trong 1 transaction, tuần tự hoá bằng named lock kèm tên DB.
- **Module**: `internal/modules/webapp/auth/twofa` (service `pquerna/otp` + handler). FE: `login.ts` (challenge), `twofa-setup/` (enroll/disable + QR `ng-qrcode`), menu profile "Bảo mật 2 lớp".
- seclog: `2fa_challenge`, `2fa_failed`, `2fa_enrolled`, `2fa_disabled`, `2fa_reset`.

## Env
```
TOKEN_SECRET=<secret>           # CRM JWT key
PARTNER_JWT_KEY=<different>     # Client JWT key — PHẢI khác nhau
TOTP_ENC_KEY=<hex 64 chars>    # mã hóa TOTP secret 2FA (openssl rand -hex 32) — prod BẮT BUỘC
```

---
name: working-on-hr
description: >-
  Quy tắc và pattern nghiệp vụ HR Cluvix trong CRM standalone: user nội bộ, role/RBAC,
  đổi/đặt lại mật khẩu, import user. Dùng skill này KHI làm feature/sửa bug đụng module
  hr (chỉ còn `hr/user`) — trước khi đọc code.
---

# Working on HR




Module quản lý user nội bộ: tài khoản, profile, role RBAC, EMR departments, import/export.

---

## File map

### Backend
```
internal/modules/webapp/hr/user/
├── route.go              # RegisterRoutes: gắn /hr/user/* + nested /user/:id/departments
├── handler/handler.go    # Tất cả handlers + BuildUserQuery + ImportUsers
├── service/service.go    # CRUD logic, cache, token revocation
└── dao/entity.go         # User, UserProfile structs + TableName
```

### Frontend
```
src/app/modules/hr/user/
├── user-list/user-list.ts                              # Danh sách user, import, export, open form
├── user-list/user-import-result-dialog/                # Dialog kết quả import (danh sách tài khoản + password mới)
├── user-list/user-reset-password-dialog/               # Dialog đặt lại mật khẩu (FE chọn/generate pass, gửi lên BE)
├── form/user-form/user-form.ts                         # Admin tạo/sửa user (có EMR dept assignment)
├── form/user-update-profile/                           # User tự sửa profile cá nhân
└── form/change-password/                               # User tự đổi mật khẩu
```

---

## Tenant scope — ĐẶCBIỆT cho bảng `user` và `user_profile`

`user` và `user_profile` **KHÔNG nằm trong `branchTables`** → `ScopedDBNullBranch(c, h.DB, "user")` / `ScopedDBNullBranch(c, h.DB, "user_profile")` đều là **silent no-op** (không thêm WHERE nào).

⚠ **(đã bỏ, D15/story-58)** Trước 1 thương hiệu/DB, `BuildUserQuery` còn tự thêm `JSON_CONTAINS`
thủ công trên `u.companyId` (fallback `companyActiveID` → `companyIDs`). Đã bỏ hoàn toàn (D15) — mọi nhân
viên trong DB thuộc thương hiệu duy nhất, KHÔNG cần lọc company nữa:

```go
// ✅ BuildUserQuery hiện tại — không còn company filter, chỉ còn guard id <> 1
q := h.DB.Table("user u").
    Select(`u.id, u.id_old, u.username, u.status, u.branchId, ...`).
    Joins("LEFT JOIN user_profile p ON p.user_id = u.id").
    Where("u.id <> 1")
// filter branch (nếu cần) qua query.ApplyJSONContainsFilter(c, q, "branch_id", "u.branchId")
// — đây là filter THEO QUERY PARAM tường minh của FE, không phải auto-scope theo session.
```

Guard IDOR của Update/Delete/đổi mật khẩu (đi qua chính `BuildUserQuery`) nay là: tồn tại +
`u.id <> 1` + `targetIsProtectedSuper` — KHÔNG còn lớp company nào để verify thêm.

**Single-record ops (GetUser, UpdateUser, DeleteUser) đều phải qua BuildUserQuery** để prevent IDOR:
```go
// ✅ Scope check trước khi sửa/xóa
var count int64
h.BuildUserQuery(c).Where("u.id = ?", uint(id)).Count(&count)
if count == 0 { response.NotFound(c); return }
```

---

## Response structure — nested

FE nhận response dạng lồng `userProfile` + `userRole` (Yii2 compat):
```json
{
  "id": 42,
  "username": "abc",
  "status": 10,
  "branchId": [1,2],
  "updated_at": 1760000000,
  "userProfile": {
    "user_id": 42,
    "fullname": "Nguyen Van A",
    "phone": "0901234567",
    "departmentList": [1, 2],
    ...
  },
  "userRole": { "name": "manager", "description": "Quản lý" }
}
```

**parseGinH / parseJSONInItems** — sau khi GORM trả `[]map[string]interface{}`, các JSON column về dạng string. Phải gọi helpers để parse thành `json.RawMessage`:
```go
helpers.ParseJSONFieldsForTables(items, "user")  // dùng cho GET /:id
parseJSONInItems(result.Items)                    // dùng cho list (sau khi nest)
```

---

## Service — key patterns

### Optimistic locking (UpdateUser) — theo `updated_at` (D6, `pkg/optlock`)
```go
// 1. Pre-check trước transaction (fast fail)
if input.UpdatedAt != user.UpdatedAt {
    return nil, errs.ErrVersionConflict
}
// 2. Ghi có khoá: WHERE updated_at = ? + bump; 0 dòng ⇒ ErrVersionConflict
optlock.Updates(tx.Model(&hrUserDao.User{}).Where("id = ?", userID), input.UpdatedAt, updates)
```

### Token revocation
Khi đổi `status` (và khi ChangePassword, DeleteUser) → revoke CẢ access token lẫn refresh token (đổi branch/role KHÔNG revoke — xem §Forced logout; vế `companyId` đã bỏ, D15):
```go
if needRevoke && s.TokenSvc != nil {
    s.TokenSvc.RevokeAllSessions(userID)      // JTI blacklist
    s.TokenSvc.RevokeAllRefreshTokens(userID) // prevent silent re-auth
}
```
Tương tự khi ChangePassword, DeleteUser.

### Role update (trong transaction)
```go
// Dùng tx (không phải s.DB) khi query role bên trong transaction
tx.Table("rbac_auth_assignment").Select("item_name").Where("user_id = ?", userID).Scan(&currentRole)
if currentRole != *input.Role {
    tx.Exec("DELETE FROM rbac_auth_assignment WHERE user_id = ?", userID)
    tx.Exec("INSERT INTO rbac_auth_assignment (item_name, user_id, created_at) VALUES (?, ?, ?)", ...)
    needRevoke = true
}
```

### GetAllUsers — cache scope
Luôn 1 khoá cache chung (story-58: bỏ lọc JSON `u.companyId`, 1 thương hiệu/DB — D15); `invalidateCache` xoá đúng khoá đó.
(Lịch sử: trước đây `useCache := len(companyIDs) == 0` — chỉ cache nhánh không lọc company; đã bỏ, D15.)

### Forced logout — chỉ khi `status` thay đổi
- `RevokeAllSessions` + `RevokeAllRefreshTokens` CHỈ khi `status` thay đổi (user bị vô hiệu hóa không được tiếp tục dùng app)
- branch/role/dept: BE enforce per-request (UserScope + RBAC cache cleared) → KHÔNG cần revoke → user không bị logout
- `BumpUserAuthVersion` chạy sau MỌI update (kể cả status) làm safety net

### BumpUserAuthVersion — khi nào gọi
Gọi `middleware.BumpUserAuthVersion(s.Cache, userID)` sau khi commit profile update khi `!needRevoke && input.UserProfile != nil`.
- KHÔNG gọi khi `needRevoke=true` (token refresh flow đã lo sync FE)
- KHÔNG gọi khi đổi password (token bị revoke → FE tự fetchAndSyncMe sau refresh)
- Hàm nằm trong `internal/middleware/response_headers.go`, cần import middleware package vào service

---

## Handler — ChangePassword vs ResetPassword vs UpdatePassword

| Endpoint | Dùng khi | Xác thực |
|---|---|---|
| `PUT /hr/user/reset-password` | Admin đặt lại pass cho user — **FE chọn/generate password** | Không cần old password; BE validate tenant scope qua `BuildUserQuery` (prevent IDOR); min 6 ký tự |
| `POST /hr/user/change-password` | Admin reset pass (legacy endpoint, vẫn còn) | Không cần old password, cần `id` target |
| `POST /hr/user/update-password` | User tự đổi pass chính mình | Phải nhập `oldpassword` + `newpassword` + `newpasswordrepeat` |

Cả 3 đều: hash bcrypt cost 12, call `ChangePassword(userID, newPass)` trong service → auto revoke sessions.

**IDOR check bắt buộc cho reset-password:**
```go
// Dùng BuildUserQuery (KHÔNG ScopedDBNullBranch trực tiếp) vì user table dùng JSON array scope
var exists int64
h.BuildUserQuery(c).Where("u.id = ?", body.ID).Count(&exists)
if exists == 0 { response.NotFound(c); return }
```

---

## ImportUsers — N+1 fix

**Trước khi bắt đầu transaction**: pre-scan toàn bộ rows để batch load:
```go
// 1. Batch load phone → userId (tránh per-row query "ai đang dùng phone này")
var phonePreRows []struct{ UserID uint; Phone string }
h.DB.Table("user_profile").Select("user_id, phone").Where("phone IN ?", phonesInFile).Find(&phonePreRows)
phoneOwner := make(map[string]uint)

// 2. Batch load profileExists (tránh per-row COUNT)
var pidPreRows []struct{ UserID uint }
h.DB.Table("user_profile").Select("user_id").Where("user_id IN ?", updateIDsInFile).Find(&pidPreRows)
profileExists := make(map[uint]bool)

// 3. Trong loop: sau khi CREATE profile mới → cập nhật map
profileExists[currentUserID] = true  // tránh duplicate key nếu cùng ID xuất hiện 2 lần
```

`importEmrDepartments` — tương tự: pre-load existing assignments trước khi vòng lặp insert.

---

## Frontend — UserForm (admin)

`user-form.ts` — admin tạo/sửa user.

### EMR department flow — ĐÃ BỎ (lịch sử)
Gán khoa EMR cho user (`emrDeptRows`, `saveDepartments`, "1 primary per `company_id`" — lịch sử, D15) đã xoá cùng
module `hr/user_emr_department` (story-12) và company (D15). Form nay submit 1 lệnh `updateUser`/`create`.

### onSubmit pattern
```typescript
const payload = this.getPayload();
const saveOp = this.isEditMode()
  ? this.modelService.updateUser(modelId!, payload, {})
  : this.modelService.create(payload, {});
saveOp.pipe(finalize(() => this.isSubmitting.set(false))).subscribe({ ... });
```

### Self-edit (admin sửa chính mình)
```typescript
if (this.isEditMode() && modelId === userId()) {
  this.syncAuthUserIfSelf();  // sync branchId vào authUser signal + localStorage (companyId/departments đã bỏ, D15)
  sessionStorage.setItem('post_reload_toast', translate.instant('...'));
  location.reload();  // phải reload để JWT context refresh
  return;
}
```

---

## Frontend — UserChangePassword

Validator cross-field mismatch cần kích hoạt lại khi `newpassword` thay đổi:
```typescript
this.rfModel.get('newpassword')!.valueChanges
  .pipe(takeUntilDestroyed(this.destroyRef))
  .subscribe(() => this.rfModel.get('newpasswordrepeat')!.updateValueAndValidity());
```

`newpassword` phải có `Validators.minLength(6)` (match BE validation ≥6 chars).

Sau khi đổi password thành công → BE revoke all tokens → FE phải logout ngay:
```typescript
setTimeout(() => this.authService.handleRefreshTokenExpired(), 1500);  // delay để user đọc toast
```

---

## Frontend — UserUpdateProfile

`birthday` timezone trap:
```typescript
// ❌ NEVER — new Date("2000-01-15") → UTC midnight → hiển thị 14/01 ở GMT+7
birthday: new Date(res.birthday)

// ✅ ALWAYS — append T00:00:00 để parse local time
birthday: new Date(res.birthday + 'T00:00:00')
```

`srcImageUpload` type — phải là `string | null` không phải `!: any`:
```typescript
srcImageUpload: string | null = null;
```

---

## Toast duplicates — KHÔNG dùng `<p-toast>` local

`app.component.ts` đã có 1 global `<p-toast position="top-right" />`.  
PrimeNG `MessageService` broadcast tới **tất cả** `<p-toast>` instance trong DOM.  
→ KHÔNG thêm `<p-toast />` vào từng page/component con (attendance.html, schedule.html, ...).

---

## i18n keys liên quan

| Key | Nội dung |
|---|---|
| `user_updated` | Cập nhật người dùng thành công |
| `password_changed` | Đổi mật khẩu thành công |
| `password_min` | Mật khẩu tối thiểu 6 ký tự |
| `password_confirm_mismatch` | Mật khẩu xác nhận không khớp |
| `password_old_wrong` | Mật khẩu cũ không đúng |
| `import_success` | Import thành công |

---

## Per-user IndexedDB auth session

Store `userAuthSession` trong IndexedDB `cluvixData` — lưu user data + permissions + authVersion **riêng từng user**.

**Đặc điểm:**
- keyPath: `userId` (KHÔNG phải `id`) — isolated by user, multi-user cùng browser không xung đột
- KHÔNG add vào Dexie global sync (`X-Dexie-Versions`) — chỉ sync khi BE báo version lệch
- Migration: fallback localStorage `auth_user` key cho session cũ, tự migrate sau lần save kế tiếp
- `auth_user_id` (localStorage, number) — lightweight boot marker để biết load IDB entry nào

**2 sync triggers:**
1. Token revocation → FE nhận 401 → silent refresh → `fetchAndSyncMe()` (gọi `/api/auth/me`, cập nhật signals + IDB)
2. Header `X-User-Auth-Version: {userId}:{timestamp}` lệch → `isStale()` → `fetchAndSyncMe()`

**`X-User-Auth-Version` header flow:**
- BE set header này trong mọi response (từ `AuthResponseHeaders` middleware), giá trị = Redis key `auth:user_auth_ver:{userID}`
- `BumpUserAuthVersion(cache, userID)` ghi Redis key với TTL 30 ngày — gọi khi profile update không revoke token
- FE kiểm tra trong `response-handler.interceptor.ts` → chỉ trigger sync khi `headerUserId === currentUserId`
- Cập nhật `_cachedAuthVersion` + `userAuthSession.updateAuthVersion()` **trước** khi gọi `fetchAndSyncMe()` để tránh loop

---

## Endpoints summary

```
GET    /hr/user                     # list + filter + export
GET    /hr/user/get-all-users       # dropdown/sync (JSON array, no pagination)
GET    /hr/user/:id                 # single record
POST   /hr/user                     # create
PUT    /hr/user/:id                 # update (optimistic lock)
DELETE /hr/user/:id                 # soft delete (status = define.UserStatusDeleted = 1)
POST   /hr/user/change-password     # admin reset pass của user khác
POST   /hr/user/update-password     # user tự đổi pass (verify old pass)
PUT    /hr/user/reset-password      # admin đặt lại pass (FE gửi password, BE validate + apply)
POST   /hr/user/import-users        # import Excel
GET    /hr/user/get-template        # download import template
```

**Status thật của `user`:** 10=Active, 9=Inactive (vô hiệu hóa tạm thời, KHÔNG phải nghỉ việc), 1=Deleted (soft-delete qua DeleteUser) — comment GORM cũ trong `entity.go` ghi sai ("0=Inactive, 9=Deleted"), đừng tin theo.

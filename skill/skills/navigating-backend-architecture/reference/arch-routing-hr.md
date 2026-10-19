# Routes Auth + HR

## Mục lục

  - Auth (internal/modules/webapp/auth/router.go) — 23 endpoints
  - HR (internal/modules/webapp/hr/router.go) (authed)

---


> ⚠ Path viết trước restructure 2026-07-28: `internal/modules/<domain>` đọc là `internal/modules/webapp/<domain>` (CRM standalone chỉ còn nhóm `webapp/`).

> Index tổng + auth groups: `arch-routing.md`. Nguồn: code tại 2026-06-10.

### Auth (internal/modules/webapp/auth/router.go) — 23 endpoints

**Path prefix:** `/api/auth`. Public sub-group (`login`, `refresh-token`) trên group `api` + phần còn lại trên `authed` (JWT).

#### Public — `/api/auth`
- POST `/api/auth/login` → `Login` (internal/modules/webapp/auth/core/handler/handler.go:94) — login email/password, rate-limit 10/IP/5min.
- POST `/api/auth/refresh-token` → `RefreshToken` (internal/modules/webapp/auth/core/handler/handler.go:162) — đổi refresh-token lấy access-token mới.

#### Authed — `/api/auth` (JWT)
- POST `/api/auth/logout` → `Logout` (internal/modules/webapp/auth/core/handler/handler.go:209).
- POST `/api/auth/logout-all` → `LogoutAll` (internal/modules/webapp/auth/core/handler/handler.go:438) — revoke mọi session + refresh token.
- POST `/api/auth/clear-cache` → `ClearCache` (internal/modules/webapp/auth/core/handler/handler.go:427) — xóa cache user.
- GET  `/api/auth/me` → `Me` (internal/modules/webapp/auth/core/handler/handler.go:235).
- GET  `/api/auth/permission` → `Permission` (internal/modules/webapp/auth/core/handler/handler.go:248).

#### RBAC — `/api/auth/rbac` (handler: internal/modules/webapp/auth/rbac/handler/handler.go)
- GET    `/api/auth/rbac/child-role-for-dropdown` → `ChildRoleForDropdown` (:65).
- GET    `/api/auth/rbac/list-permission` → `ListPermissionByRole` (:218).
- POST   `/api/auth/rbac/update-permission-with-role` → `UpdatePermissionWithRole` (:29).
- POST   `/api/auth/rbac/sync` → `SyncPermissions` (:233).
- GET    `/api/auth/rbac/role` → `ListRole` (:70) — alias plural `/roles`.
- GET    `/api/auth/rbac/role/get-name` → `GetRoleName` (:77).
- POST   `/api/auth/rbac/role/create-name` → `CreateRoleName` (:92).
- PUT    `/api/auth/rbac/role/update-name` → `UpdateRoleName` (:109).
- POST   `/api/auth/rbac/role/update-permission-with-role` → `UpdatePermissionWithRole` (:29) — alias.
- DELETE `/api/auth/rbac/role/delete-name` → `DeleteRoleName` (:130).
- GET    `/api/auth/rbac/permission` → `ListPermission` (:146) — alias plural `/permissions`.
- GET    `/api/auth/rbac/permission/get-name` → `GetPermissionName` (:151).
- POST   `/api/auth/rbac/permission/create-name` → `CreatePermissionName` (:166).
- PUT    `/api/auth/rbac/permission/update-name` → `UpdatePermissionName` (:183).
- DELETE `/api/auth/rbac/permission/delete-name` → `DeletePermissionName` (:204).

---

### HR (internal/modules/webapp/hr/router.go) (authed)

**Path prefix:** `/api/hr`.

#### User (`/user`)
- GET    `/api/hr/user` → `ListUser` (internal/modules/webapp/hr/user/handler/handler.go:72).
- GET    `/api/hr/user/get-all-users` → `GetAllUsers` (internal/modules/webapp/hr/user/handler/handler.go:341) — không phân trang.
- GET    `/api/hr/user/:id` → `GetUser` (internal/modules/webapp/hr/user/handler/handler.go:131).
- POST   `/api/hr/user` → `CreateUser` (internal/modules/webapp/hr/user/handler/handler.go:190).
- PUT    `/api/hr/user/:id` → `UpdateUser` (internal/modules/webapp/hr/user/handler/handler.go:206).
- POST   `/api/hr/user/change-password` → `ChangePassword` (internal/modules/webapp/hr/user/handler/handler.go:251) — user tự đổi.
- POST   `/api/hr/user/update-password` → `UpdatePassword` (internal/modules/webapp/hr/user/handler/handler.go:272).
- PUT    `/api/hr/user/reset-password` → `ResetPassword` (internal/modules/webapp/hr/user/handler/handler.go:313) — admin reset.
- POST   `/api/hr/user/import-users` → `ImportUsers` (internal/modules/webapp/hr/user/handler/handler.go:390) — import Excel.
- GET    `/api/hr/user/get-template` → `GetTemplate` (internal/modules/webapp/hr/user/handler/handler.go:680) — Excel template.
- DELETE `/api/hr/user/:id` → `DeleteUser` (internal/modules/webapp/hr/user/handler/handler.go:232).

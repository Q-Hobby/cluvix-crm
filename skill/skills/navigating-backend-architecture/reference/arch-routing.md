# Routing Index

## Mục lục

- Auth groups
  - CORS — thêm method/header mới (CRITICAL)
- Static & infrastructure
- Route theo module — xem file con
- Notes về cross-module routing
- Auth coverage tóm tắt

---


Tổng số endpoint: đếm bằng `grep -rE '\.(GET|POST|PUT|DELETE|PATCH)\(' backend/internal/modules/` — KHÔNG hardcode số (drift nhanh).
Source entry: `backend/internal/router/router.go` → `Setup()` — gọi `<module>.Register(authed, d)` cho từng module tại `internal/modules/<module>/router.go` (danh sách module: xem `router.go`).

> **Layout mới (Plan 07/08):** KHÔNG còn `internal/router/<module>.go` và `internal/handlers/` — mỗi module tự đăng ký route trong `internal/modules/<module>/router.go`, handler tại `internal/modules/<module>/<resource>/handler/`.

## Auth groups

- **`api`**: `r.Group("/api")` — public. Dùng cho login, refresh-token, `/api/client/oauth/token`.
- **`authed`**: `api.Group("", middleware.JWTAuth(), middleware.AuthResponseHeaders(db, cache))` — Bearer JWT user nội bộ + response headers (Cluvix-Customer-Lead, Cluvix-Reminder, Cluvix-Auth).
- Global middleware (router.go:60-72): `gin.Logger`, custom recovery (log panic + stack), `RequestID`, `RequestSizeLimit`, `CORS`, `SecurityHeaders`, `RequestPath` (bake path vào SQL error log), `ResponseHeaders(db, cache)` (Dexie/EMR version headers).

### CORS — thêm method/header mới (CRITICAL)

- Dùng HTTP method mới cho route (`r.PATCH`...) → BẮT BUỘC thêm vào `Access-Control-Allow-Methods` trong `internal/middleware/cors.go` + case trong `cors_test.go`.
- Thiếu method: preflight vẫn trả 204, browser chặn request thật → **không có dòng log nào ở BE**, DevTools không hiện status code. Dấu hiệu trong `backend/tmp/run.log`: có `OPTIONS "/api/x"` mà KHÔNG có dòng method thật ngay sau.
- Thiếu header trong `Access-Control-Allow-Headers` gây triệu chứng câm y hệt.
- FE prod chạy khác origin BE (`corsAllowedOrigins`) → lỗi loại này ảnh hưởng cả production, không riêng dev.

## Static & infrastructure

- `GET /health` → inline handler (router.go:74) — public.
- `Static /uploads` → `./storage/uploads` | `/download/export` → `./storage/export` | `/download/templates` → `./storage/templates`.

---

## Route theo module — xem file con

| File | Modules | Prefix |
|------|---------|--------|
| `arch-routing-crm.md` | CRM | `/api/crm` |
| `arch-routing-config.md` | Config (số resource: `ls internal/modules/webapp/config/`) | `/api/config` |
| `arch-routing-hr.md` | Auth, HR | `/api/auth`, `/api/hr` |
| `arch-routing-ops.md` | Print, History, Import, Export | `/api/print`, `/api/history`, `/api/import` |

Số endpoint từng module: đếm trực tiếp trong `internal/modules/<module>/` — không ghi số cứng ở đây.

---

## Notes về cross-module routing

- **IndexedDB sync**: handler thuộc module Config, mount tại `/api/crm/indexeddb/sync` (legacy path).
- **Export module KHÔNG có route riêng** — utility package gọi cross-module (Print, report).

## Auth coverage tóm tắt

| Group | Auth |
|---|---|
| `/health`, static `/uploads`, `/download/*` | none |
| `/api/auth/login`, `/api/auth/refresh-token` | none |
| Toàn bộ `authed` (CRM/Config/HR/...) | JWTAuth + AuthResponseHeaders |

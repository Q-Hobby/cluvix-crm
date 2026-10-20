# Cluvix CRM

CRM đa chi nhánh cho doanh nghiệp dịch vụ: quản lý khách hàng (hồ sơ, ghi chú, tệp đính kèm,
hạng khách), lịch hẹn, đơn hàng/kế hoạch dịch vụ, thanh toán và công nợ, nhắc việc, dashboard +
báo cáo doanh thu (theo ngày, theo nhân viên telesale), import/export Excel, mẫu in (phiếu
thu, lịch hẹn, đơn hàng), nhân viên + phân quyền (RBAC, 2FA) và nhật ký thao tác. Backend Go (Gin + GORM + MySQL + Redis), frontend Angular (PrimeNG). Mô hình
**1 database = 1 thương hiệu** (DB-per-customer) — mọi `branch_id` trong DB chỉ phân tách các
chi nhánh của CÙNG MỘT thương hiệu.

*Multi-branch CRM for service businesses: customers, appointments, orders/service plans,
payments & debt, reminders, dashboard & revenue reports, Excel import/export, print
templates, staff & role-based permissions (with 2FA) and audit log.*

> 🇻🇳 **Tóm tắt nhanh**: clone repo → đăng ký license PrimeUI + lấy key (§2: Community miễn phí nếu đủ
> điều kiện, còn lại / cần template Ultima thì mua Commercial) → đặt gói layout dẫn xuất từ Ultima
> (gói private, chỉ cấp cho người có license Commercial — liên hệ chủ dự án; KHÔNG có trong repo này) → tạo cert local bằng `mkcert` → copy
> `backend/.env.example` → `backend/.env` và điền giá trị → tạo database MySQL rỗng → chạy
> migration → chạy backend (cổng 8011) + frontend (cổng 4211) → đăng nhập `admin` /
> `ChangeMe@2026` — hệ thống bắt đổi mật khẩu ngay ở lần đăng nhập đầu. Chi tiết từng bước ở dưới (tiếng Anh).

## Tech stack / Requirements

| Component | Version | Note |
|---|---|---|
| Go | ≥ 1.26 (see `backend/go.mod`) | |
| Node.js | `^20.19.0 \|\| ^22.12.0 \|\| >=24.0.0` (Angular 22 peer requirement, see `frontend/package-lock.json`) | |
| Angular CLI | 22.x (installed via `npm install`, no global install needed) | |
| MySQL | ≥ 8.4 | Project runs MySQL 9.7 LTS in dev/prod, but the shipped migrations do not use 9.x-only syntax (verified: no `VECTOR`/`JSON_TABLE`/system-versioning usage) |
| Redis | 6.x or newer | session/refresh-token store, cache, rate limit |
| [mkcert](https://github.com/FiloSottile/mkcert) | any recent version | to generate a locally-trusted TLS certificate — both the backend API and the Angular dev server run over HTTPS in this project |

## 1. Clone

```bash
git clone <this-repo-url> cluvix-crm
cd cluvix-crm
```

## 2. PrimeUI licensing, the Ultima layout package and your license key

The frontend depends on two commercial products from **PrimeTek Informatics**. Neither is
included in this repository, and **each developer who builds or modifies the frontend needs
their own license** — the Cluvix license and key are **not** transferable to you.

| What | Why this project needs it | License you need |
|---|---|---|
| **PrimeNG 22** (`primeng`, `@primeuix/*`, `@primeicons/*` from npm) | All UI components. Since v22 these packages are distributed under the *PrimeUI License* (not open source — see `frontend/node_modules/primeng/LICENSE.md`) and expect a **license key**. | **Community** (free, if eligible) **or Commercial** |
| **Ultima** application template | The admin layout (sidebar, topbar, menu, layout service). It is part of PrimeUI's **Template Catalog**, which is included only in the Commercial license. | **Commercial** |

> Terms below are summarised from https://primeui.dev/licenses/community,
> https://primeui.dev/licenses/commercial and https://primeui.dev/pricing as of 2026-10-19.
> PrimeTek can change them at any time — **the binding terms are always the ones on their site**.

### 2.1 Choose your license

**Community (free, renewed every 12 months)** — available to:
- individual developers and students working on their own projects, or
- organizations that meet **all** of: fewer than 5 developers in total (counting every developer,
  not only those using PrimeUI), fewer than 10 employees, under 1M USD annual revenue (or budget),
  never more than 3M USD outside funding, and **not** a public-sector body, government-funded
  organization, university or other educational institution. Non-profit status alone does not
  qualify, and there is **no separate open-source program**.

Community covers PrimeNG/PrimeReact/PrimeVue and PrimeIcons (up to 4 developer seats) but **not**
templates such as Ultima, PrimeBlocks or the Theme Designer. You may still build and sell
commercial apps with it.

**Commercial (paid, per developer, perpetual)** — required for everyone else, and for anyone who
needs the Ultima template. One seat per developer who writes or maintains frontend code (backend
developers, QA, designers and CI servers do not need a seat); seats can be reassigned when people
leave. The license is perpetual and includes 1 year of updates; renewal is optional and only
needed for newer versions. Larger teams can ask for a **Site License** (organization-wide, annual).
Current prices: https://primeui.dev/pricing.

### 2.2 Register and get your key

1. Open the **PrimeStore**: https://primeui.store/primeui and create an account (*Sign Up*).
2. Pick a plan:
   - **Community** → click **Apply**, confirm you meet the eligibility criteria (self-service;
     PrimeTek may ask for proof later). Registering while knowingly ineligible voids the license.
   - **Commercial** → choose the number of developer seats → **Buy Now** and pay. For a Site
     License (or volume pricing from 25 seats) use *Get a quote* on the pricing page.
3. After registration/purchase, copy your **PrimeUI license key** from your PrimeStore account.
   The key is a long signed string; it is verified **offline** (no network call, no telemetry).
4. **Community keys expire after 12 months** (30-day grace period). Renew for free in the
   PrimeStore by re-confirming eligibility, then replace the key (section 2.4). For Commercial,
   apps built while your license was valid keep running without a notice even after the 1-year
   update period ends.

The key is not a secret (it ends up in the JS bundle), but the license forbids **publishing it
for others to use** — never commit it to this public repository.

### 2.3 Get the private Ultima-derived layout package

This project's layout files are **not** the stock Ultima template: they were adapted from Ultima
(extra methods/fields), so an unmodified Ultima download will not build. Because of Ultima's
license they are excluded from this repository via `.gitignore` (section "Template Ultima") and
distributed separately as a **private Ultima-derived layout package**, only to Ultima licensees.

1. Hold a **Commercial** PrimeUI license (section 2.1) — that is what gives you Ultima rights.
2. Request the package from the project maintainer: **<contact the project maintainer — channel to
   be announced>**, with proof of your license (PrimeStore order / license owner).
3. Extract it into `frontend/src/` so every file lands on the exact paths listed under
   "Template Ultima" in the root `.gitignore` (`assets/layout/**`, `assets/demo/**`,
   `assets/overrides/…`, `app/layout/components/*.ts`, `app/layout/service/layout.service.ts`,
   and a few files under `app/modules/auth/` and `app/modules/utilities/`). Those paths stay
   git-ignored, so the package never gets committed back.

Without the package the frontend does not build (missing `assets/layout/**`, `app/layout/**`);
the backend runs fine on its own.

### 2.4 Put the key into the app

The key is passed to PrimeNG through `providePrimeNG({ license })` in `frontend/src/app.config.ts`
(read via `frontend/src/app/share/utils/primeui-license.util.ts`). Every tracked
`frontend/src/environments/environment*.ts` ships with `primeuiLicense: ''` — set it only in a
**git-ignored** copy:

- **Local development** (`ng serve -c local`):
  ```bash
  cp frontend/src/environments/environment.local.example.ts frontend/src/environments/environment.local.ts
  ```
  set `primeuiLicense: '<your key>'`, then run
  `npx ng serve -c local --port 4211 --ssl true --ssl-cert ./localhost.pem --ssl-key ./localhost-key.pem`.
- **Docker dev** (`docker compose`): put the key in `primeuiLicense` of
  `frontend/src/environments/environment.docker.ts` on your machine only (this file is tracked —
  do not commit the change; `git update-index --skip-worktree <file>` helps). See [DOCKER.md](DOCKER.md).
- **Production build**: copy `environment.cluvix.example.ts` → `environment.cluvix.ts`
  (git-ignored), set `primeuiLicense` (and leave `primeuiLicenseCdnUrl: ''`), then
  `npm run build:cluvix`. The plain `npm run build` uses the tracked `environment.prod.ts`, which
  has no key.
- **Optional: rotate the key without rebuilding.** Host a JSON file with the key on your own
  CDN, set `primeuiLicenseCdnUrl` to its URL in your git-ignored environment file, and build with
  `SW_S3_ORIGIN=https://<your-cdn> npm run build:cluvix` so the service worker lets that request
  through (`scripts/inject-sw-origin.js`). The app keeps using the cached/build-time key and picks
  up a new one on the next page load — handy for yearly Community renewals. The file must follow
  the schema read by `primeui-license.util.ts`; the URL Cluvix uses for its own key is not
  licensed to third parties.

**Check:** with a valid key there is no license notice. With a missing, invalid or expired key,
PrimeNG shows a red **"Invalid PrimeUI License"** banner — the app keeps working, but running
without a valid license is a breach of PrimeTek's terms, not just a cosmetic issue.

> 🇻🇳 **Tóm tắt đăng ký PrimeUI**: (1) Vào https://primeui.store/primeui → *Sign Up*. (2) Đủ điều
> kiện Community (cá nhân, hoặc tổ chức < 5 dev, < 10 nhân sự, doanh thu < 1M USD, gọi vốn < 3M USD,
> không phải cơ quan nhà nước/trường học) → **Apply** (miễn phí, gia hạn mỗi 12 tháng). Không đủ,
> hoặc cần template **Ultima** → mua **Commercial** theo số dev (**Buy Now**, vĩnh viễn + 1 năm cập
> nhật). (3) Copy license key trong tài khoản PrimeStore. (4) Dán vào `primeuiLicense` của file
> environment **không theo dõi git** (`environment.local.ts` khi dev, `environment.cluvix.ts` khi
> build prod) — tuyệt đối không commit key. (5) Có license Commercial thì liên hệ chủ dự án xin gói
> layout dẫn xuất Ultima (mục 2.3). Mỗi dev tự có license riêng, không dùng chung key của Cluvix.

## Chạy bằng Docker / Run with Docker (dev)

A one-command Docker dev environment (MySQL 9.7, Redis, Go backend, Angular dev server) is
available — see [DOCKER.md](DOCKER.md) (Vietnamese). It still needs the private Ultima layout
package from this section. Host ports: frontend 14211, backend 18011, MySQL 13306, Redis 16379.

## 3. Generate a local TLS certificate (mkcert)

The frontend dev server (`npm start`) and the backend API both run over HTTPS in this
project's default configuration (`frontend/src/environments/environment.ts` points to
`https://localhost:8011/api`).

```bash
# once per machine
mkcert -install

# from the repo root
mkdir -p backend/certs
mkcert -cert-file backend/certs/localhost.pem -key-file backend/certs/localhost-key.pem localhost 127.0.0.1 ::1
cp backend/certs/localhost.pem backend/certs/localhost-key.pem frontend/
```

`backend/certs/` and `*.pem` are already gitignored — these files never get committed.

## 4. Configure the backend `.env`

```bash
cd backend
cp .env.example .env
```

Edit `backend/.env`:

- `APP_ENV=dev` — for local development (`prod` is for real deployments). `dev` also relaxes
  CORS (any Origin is allowed) so you don't need to touch `CORS_ALLOWED_ORIGINS`. With `APP_ENV=prod` you
  MUST set `CORS_ALLOWED_ORIGINS` (comma-separated frontend origins) — left empty, every
  cross-origin request is rejected.
- `DB_NAME=cluvix_crm` (or any name you'll create in step 5).
- `DB_USER` / `DB_PASS` — a MySQL user with privileges on that database.
- `TOKEN_SECRET` — required even in dev. Generate one: `openssl rand -hex 32`.
- `TOTP_ENC_KEY` — encrypts users' 2FA (TOTP) secrets at rest; recommended even for local dev
  if you want to try 2FA. Generate with `openssl rand -hex 32`.
- `TLS_CERT=./certs/localhost.pem` and `TLS_KEY=./certs/localhost-key.pem` — paths are
  relative to the `backend/` working directory (where you'll run `go run .` from step 7),
  pointing at the cert generated in step 3.
- Everything else can be left blank for a local run. Telegram error alerts and the
  service-type/unit suggestion URLs simply stay off when empty. The `S3_*` keys (S3/MinIO)
  are only needed for features that store files — attachments and uploads won't work until
  you point them at a bucket.

## 5. Create an empty database

```sql
CREATE DATABASE cluvix_crm CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

Must be a **brand-new, empty** database — the seed migration (step 6) creates the default
branch and admin account only when the relevant tables are still empty.

## 6. Run migrations

From `backend/`:

```bash
go run ./cmd/migrate -cmd=up
```

This applies every file under `backend/migrations/` in order, including the seed migration
that creates the default branch and the `admin` account (see step 8).

## 7. Run the backend

From `backend/`:

```bash
go run .
```

The API listens on `SERVER_HOST:SERVER_PORT` from `.env` (default `127.0.0.1:8011`), over
HTTPS if `TLS_CERT`/`TLS_KEY` are both set (see step 3-4), otherwise plain HTTP.

## 8. Run the frontend

From `frontend/`:

```bash
npm install
npm start
```

`npm start` runs `ng serve --port 4211 --ssl true --ssl-cert ./localhost.pem --ssl-key
./localhost-key.pem` — open **https://localhost:4211**.

For a production bundle run `npm run build` — the output goes to `frontend/dist/`
(git-ignored); serve it with any static web server / reverse proxy that forwards `/api` to the
backend (`environment.prod.ts` uses the relative `apiUrl: '/api'`).

## 9. Log in

Default admin account seeded by the migration in step 6:

- Username: `admin`
- Password: `ChangeMe@2026`

This password is public, so the system **forces a password change on the first login**: while the
account still has the default password (migration `20261018214920_add_user_must_change_password`
flags it — only if the hash is still the seeded default), every business API returns `403` with
code `password_change_required` and the web app shows only the "Password change required" screen.
After you change the password you are signed out and sign in again with the new one. The same
rule applies whenever an administrator resets another user's password; changing your own
password clears it.

## 10. Production deployment

Dev uses `go run .` and `ng serve`; production uses a built binary behind nginx. Per-customer
deployment model: **one database + one backend process (own working directory and `.env`) per
customer**; Redis may be shared because keys are prefixed `{APP_CODE}:{APP_ENV}:{DB_NAME}:`.

**Build**

```bash
cd backend  && go build -o cluvix-crm .          # backend binary (also: go build -o migrate ./cmd/migrate)
cd frontend && npm ci && npm run build            # static bundle in frontend/dist/
```

**Required env in production** (`backend/.env`; full list with comments in `backend/.env.example`,
parsed by `backend/config/config.go`):

| Variable | Requirement |
|---|---|
| `APP_ENV` | Only `dev`/`develop`/`local` are dev. Any other value (`prod`…) — or leaving it unset — runs fail-closed prod mode. |
| `DB_HOST` `DB_PORT` `DB_USER` `DB_PASS` `DB_NAME` | A dedicated database per customer. Never share a database between customers. |
| `TOKEN_SECRET` | Required; own secret per customer (`openssl rand -hex 32`). In prod it must be ≥ 32 chars and not the sample value, or the backend refuses to start. |
| `TOTP_ENC_KEY` | 64 hex chars (`openssl rand -hex 32`). Missing/malformed = the backend refuses to start in prod. Changing it forces every user to re-enrol 2FA. |
| `CORS_ALLOWED_ORIGINS` | Comma-separated FE origins. Empty in prod = every cross-origin request rejected. |
| `PERM_ENFORCE` | Defaults to `true` (fail-closed); only the literal `false` disables it. Keep `true`. |
| `TRUSTED_PROXIES` | IP/CIDR of the nginx that connects to Go (e.g. `127.0.0.1`), otherwise client IPs are wrong. |
| `TLS_CERT` / `TLS_KEY` | Leave both empty when nginx terminates TLS (backend then serves plain HTTP). |
| `SERVER_HOST` / `SERVER_PORT` | Bind address of the backend; keep on `127.0.0.1` behind nginx. |
| `REDIS_HOST` `REDIS_PORT` `REDIS_PASSWORD` `REDIS_DB_CACHE` `REDIS_DB_AUTH` | Redis may be shared between customers (key prefix includes `DB_NAME`). |
| `DOWNLOAD_URL` | Leave empty (same origin as the FE) or make sure its origin is in `CORS_ALLOWED_ORIGINS`, otherwise export download fails. |

More options (`DEVICE_BINDING_MODE`, `IP_ALLOWLIST_MODE`, `EXPORT_FILE_TTL_HOURS`, …) are in
`backend/DEPLOY.md`.

**Migrate** (the app never auto-migrates) — run before starting a new version, from `backend/`:
`go run ./cmd/migrate -cmd=up`, or use the compiled `migrate` binary. The seed migration creates
the default branch and `admin`; the system forces the admin to change the default password on the
first login (see step 9).

**nginx + systemd, health checks, backup/restore**: see `backend/DEPLOY.md`
(sections "nginx reverse proxy", "systemd", "Health check", "Backup / restore"). Key points:
nginx terminates TLS, serves `frontend/dist/` and proxies `/api` to the backend port;
do **not** alias `/download/` to the storage directory (export files contain customer PII and are
only served by the authenticated `GET /api/download/export/:name`).

**First deploy after the Redis-prefix / JWT-issuer change**: every user is signed out once and
must log in again.

## AI agent knowledge base (`skill/`)

This repository ships a `skill/` directory: a knowledge base written for AI coding agents
(Claude Code, Codex, Gemini, etc.) — architecture notes, hard rules, and per-domain guides
accumulated while building this project. `CLAUDE.md`, `AGENTS.md` and `GEMINI.md` at the repo
root are symlinks to `skill/INDEX.md`, the entry point each tool reads automatically. You do
not need it to run the app; it's there if you (or your AI assistant) want context before
changing code.

## License

Cluvix CRM is **dual-licensed** by Cluvix Solutions (Copyright © 2026 Cluvix Solutions):

- **GNU Affero General Public License v3.0 only** (`AGPL-3.0-only`) — see [`LICENSE`](./LICENSE)
  for the full text and [`NOTICE`](./NOTICE) for the copyright notice, the additional permission
  and third-party notices; or
- **a commercial license** from Cluvix Solutions — see
  [`COMMERCIAL-LICENSE.md`](./COMMERCIAL-LICENSE.md) for when you may need one (for example,
  running a modified version as a service without publishing your changes, embedding it in a
  closed-source product, or getting support/SLA).

**Network use (AGPL section 13).** If you modify Cluvix CRM and let users interact with it over
a network, you must offer them the Corresponding Source of your modified version. Set
`SOURCE_CODE_URL` in `frontend/src/app/define/brand.define.ts` to the URL of your source code:
the account menu then shows a "Source code (AGPL-3.0)" link (it is hidden while the constant is
empty).

**Additional permission (AGPL section 7).** [`NOTICE`](./NOTICE) grants additional permission to
combine Cluvix CRM with **PrimeNG / PrimeUI** (`primeng`, `@primeuix/*`, `@primeicons/*`, under
the PrimeUI License) and with the **private Ultima-derived layout package** from section 2, and
to convey the result, without those components becoming subject to the AGPL and without
having to publish their source. The rest of the program stays under the AGPL.

**Third-party components.** The **Ultima** PrimeNG template and the **PrimeUI** license key
referenced in section 2 are third-party commercial products from PrimeTek, licensed separately
and **not included** in this repository; the private layout package stays subject to Ultima's
license terms. PrimeNG and the related packages are installed from npm under PrimeTek's own
*PrimeUI License* (free Community tier for eligible organizations, paid Commercial tier
otherwise — see section 2); anyone running or developing this frontend must hold a PrimeUI
license that fits them. The PrimeUI key that Cluvix serves from its own CDN
belongs to Cluvix Solutions and is **not** licensed to third parties — self-hosters must use
their own key (`primeuiLicense`, or `primeuiLicenseCdnUrl` pointing at their own CDN; empty by default). All other Go and npm
runtime dependencies use permissive licenses (MIT, BSD, ISC, Apache-2.0, BlueOak-1.0.0, 0BSD)
or MPL-2.0 (`go-sql-driver/mysql`).

**Trademarks.** "Cluvix", "Cluvix CRM" and the Cluvix logos are trademarks of Cluvix Solutions
and are not licensed under the AGPL — forks that are distributed or offered as a public
service must use their own name and logo. See [`TRADEMARK.md`](./TRADEMARK.md).

**Contributing.** Contributions are welcome and require signing the Contributor License
Agreement — see [`CONTRIBUTING.md`](./CONTRIBUTING.md) and [`CLA.md`](./CLA.md). Security
vulnerabilities: see [`SECURITY.md`](./SECURITY.md).

> 🇻🇳 **Tóm tắt license**: mã nguồn theo **AGPL-3.0-only** hoặc **license thương mại** của Cluvix
> Solutions (mô hình kép). Dùng AGPL miễn phí, nhưng nếu SỬA ĐỔI và cho người khác dùng qua mạng
> (SaaS) thì phải công bố mã nguồn bản sửa (đặt `SOURCE_CODE_URL` để hiện link "Mã nguồn"); không
> muốn thế → mua license thương mại (`COMMERCIAL-LICENSE.md`). Ngoại lệ §7 trong `NOTICE` cho phép
> kết hợp với PrimeNG/PrimeUI và gói layout Ultima mà không kéo chúng vào AGPL. Key PrimeUI trên
> CDN riêng của Cluvix KHÔNG cấp cho bên thứ ba — tự dùng key của mình. Tên/logo "Cluvix" là thương hiệu,
> không cấp theo AGPL (`TRADEMARK.md`). Đóng góp code phải ký CLA (`CLA.md`).

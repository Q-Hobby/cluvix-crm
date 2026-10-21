---
name: deploying-to-production
description: >-
  Quy trình deploy cluvix-crm lên VPS production (1 instance = 1 khách = 1 DB riêng): deploy mỗi
  lần (build FE trên máy dev → upload → server pull main + build Go + migrate + restart +
  health-check + tự rollback), setup instance mới, đổi cấu hình .env/storage S3, và nơi giữ cấu
  hình bí mật ngoài git. Dùng skill này KHI deploy, tạo instance/domain mới, sửa script deploy,
  đổi STORAGE_DRIVER, hoặc deploy lỗi. Ý nghĩa từng biến env, mẫu nginx/systemd, backup DB:
  backend/DEPLOY.md.
---

# Deploy production

> `reference/server-deploy-script.md` = script deploy phía server (mẫu đầy đủ, không chứa bí mật).
> Biến env, nginx, systemd, health check, backup/restore: `backend/DEPLOY.md`.

## 1. Mô hình

Ký hiệu: `<BASE>` = thư mục web gốc trên server, `<instance>` = tên instance (vd `crm`), `<domain>` = domain instance.

| Thành phần | Vị trí / cách chạy |
|---|---|
| Source | `<BASE>/cluvix-crm` — clone repo public qua HTTPS, nhánh `main`. Chỉ `pull`, KHÔNG sửa tay, KHÔNG push |
| Backend instance | `<BASE>/<instance>/backend/` — binary `cluvixCrm` + `migrate`, `.env`, `storage/`, symlink `migrations` → source |
| Frontend instance | `<BASE>/<instance>/public/` (nginx root) + `<BASE>/<instance>/frontend-upload/` (bản build vừa upload) |
| Service | systemd `cluvix-crm` — `User=www-data`, `WorkingDirectory=backend/`, `EnvironmentFile=backend/.env`, bind `127.0.0.1:<port>` |
| nginx | vhost `<domain>`: TLS, SPA fallback, proxy `/api/`, `/uploads/`, `/download/templates/` về backend (mẫu `backend/DEPLOY.md` §6) |
| Dữ liệu | MySQL: DB riêng + user riêng cho instance. Redis dùng chung (prefix `{APP_CODE}:{APP_ENV}:{DB_NAME}:`) |
| Build | FE build trên máy dev (`frontend/dist` gitignored). BE build trên server Linux (go-webp dùng cgo — cần `gcc` + `libwebp-dev`, KHÔNG cross-compile từ macOS) |

## 2. Bí mật — KHÔNG BAO GIỜ vào git

| Thứ | Chỗ duy nhất được để |
|---|---|
| `.env` instance (DB pass, `TOKEN_SECRET`, `TOTP_ENC_KEY`, `STORAGE_SIGNING_KEY`, `S3_KEY`/`S3_SECRET`…) | chỉ trên server, `chmod 600`, owner = user chạy deploy (systemd đọc bằng root) |
| Key PrimeUI | `frontend/src/environments/environment.cluvix.ts` (gitignored; tạo từ `environment.cluvix.example.ts`) |
| Địa chỉ server, user ssh, endpoint/bucket/CDN S3 thật | script deploy local NGOÀI repo + runbook nội bộ |

- Secret sinh ngay trên server: `openssl rand -hex 32`. Mỗi instance một bộ riêng. KHÔNG dán qua chat/log/ticket.
- Xem `.env` thì che giá trị: `sed -E 's/=.*/=…/' .env`. Copy secret giữa 2 file thì làm trên server, không in ra.
- Source trên server không push ⇒ binary build ra (`backend/cluvixCrm`) cho vào `.git/info/exclude`, KHÔNG sửa `.gitignore`.

## 3. Deploy mỗi lần

1. Commit + push backend lên `main` — server chỉ build từ `origin/main`.
2. Trên máy dev chạy script deploy local (ngoài repo). Script làm:
   ```bash
   cd frontend && SW_S3_ORIGIN=<S3_CDN> npm run build:cluvix      # cần environment.cluvix.ts
   rsync -az --delete frontend/dist/ <server>:<BASE>/<instance>/frontend-upload/
   ssh <server> '~/deploy/<instance>.sh'
   ```
   Tuỳ chọn `--no-build` = dùng `frontend/dist` đang có.
3. Script server (`reference/server-deploy-script.md`) chạy theo thứ tự:
   `git pull --ff-only origin main` → `go build` binary + `migrate` → stop service → backup `*.bak` →
   copy binary → `rsync frontend-upload/ → public/` (KHÔNG `--delete`) → symlink `migrations` →
   `./migrate -cmd=up` → restart → health-check `GET /health/ready` (10 lần × 2s).
4. Migrate / restart / health-check lỗi ⇒ script tự chép `*.bak` về, restart bản cũ, `exit 1`.
   Migration đã apply KHÔNG được rollback (migration luôn idempotent, chỉ `.up.sql`).
5. Chỉ deploy backend: ssh chạy thẳng `~/deploy/<instance>.sh` — FE giữ bản đang có trong `frontend-upload/`.

### Kiểm tra sau deploy

```bash
curl -s -o /dev/null -w "%{http_code}\n" https://<domain>/            # 200
curl -s -o /dev/null -w "%{http_code}\n" https://<domain>/api/auth/me # 401 khi chưa login = backend sống
curl -s https://<domain>/ | grep -o "I18N_VERSION = '[^']*'"          # = version vừa build
systemctl is-active cluvix-crm
```

### Gotcha

- `npm run build:cluvix` luôn sửa 3 file tracked (`index.html`, `custom-ngsw-worker.js`, `cached-translate-loader.ts` — prebuild `inject-version.js`) ⇒ KHÔNG commit 3 thay đổi này.
- `go.mod` đòi Go mới hơn bản cài trên server ⇒ `GOTOOLCHAIN=auto` tự tải toolchain; server chặn mạng ra ⇒ cài đúng bản Go.
- `ssh` không tương tác in `Pseudo-terminal will not be allocated` — vô hại.
- `nginx -t` cảnh báo `protocol options redefined` khi các vhost khai `http2` khác nhau — vô hại.
- Lỗi khởi động: xem `<LOG_DIR>/<instance>-error.log` (stderr của service). Prod thiếu `TOKEN_SECRET`/`TOTP_ENC_KEY`/`STORAGE_SIGNING_KEY` (driver local) hoặc cấu hình storage sai ⇒ service từ chối khởi động.

## 4. Sửa cấu hình instance

Sửa `.env` trên server → `sudo systemctl restart cluvix-crm` → `curl -s http://127.0.0.1:<port>/health/ready`. KHÔNG cần redeploy.

### Storage S3 (giống các instance khác)

- `STORAGE_DRIVER=s3`, `S3_FOLDER=<instance>` — prefix riêng, KHÔNG trùng instance khác; folder tự sinh khi có upload đầu tiên.
- Build FE với `SW_S3_ORIGIN=<S3_CDN>` để service worker cache ảnh CDN.
- `S3_PRIVATE_BUCKET` trống ⇒ kho private = cùng bucket, folder `<S3_FOLDER>/private`, ACL private. Bucket có policy public-read toàn bucket ⇒ file đính kèm khách đọc được qua URL. Kiểm: `put_object` ACL `private` vào `<S3_FOLDER>/private/` rồi GET không ký qua endpoint và CDN — `200` = lộ, xoá object test ngay. Chặn: đặt `S3_PRIVATE_BUCKET` = bucket không public.
- Đổi driver: backup `.env` → sửa → restart → dòng log `storage:` trong `<instance>-error.log` phải ghi đúng driver. Đổi từ local khi `storage/` đã có file ⇒ chuyển file trước (`backend/DEPLOY.md` §2.2).

## 5. Setup instance mới (1 lần)

Script setup PHẢI dừng nếu `backend/.env` đã tồn tại (ghi đè = mất secret, hỏng dữ liệu đã mã hoá).

1. Clone source nếu chưa có: `git clone -b main https://github.com/cluvix/cluvix-crm.git <BASE>/cluvix-crm`.
2. `mkdir -p` `backend/storage/{uploads,private,export,templates}`, `public/`, `frontend-upload/`; `chown -R www-data:www-data backend/storage`.
3. `CREATE DATABASE … CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci` + MySQL user riêng, `GRANT ALL` chỉ trên DB đó.
4. `.env` từ `backend/.env.example`: `APP_ENV=prod`, `SERVER_HOST=127.0.0.1`, `SERVER_PORT` chưa ai nghe (`ss -ltn`), `CORS_ALLOWED_ORIGINS=https://<domain>`, `TRUSTED_PROXIES=127.0.0.1`, `DOWNLOAD_URL=https://<domain>`, `DEVICE_BINDING_MODE=log`, `PERM_ENFORCE=true`, secret sinh mới. `chmod 600`.
5. Unit systemd (`backend/DEPLOY.md` §7, `ExecStart` = binary `cluvixCrm`) → `daemon-reload` → `systemctl enable cluvix-crm`.
6. vhost nginx → `sites-available/` + symlink `sites-enabled/` → `nginx -t` → `systemctl reload nginx`.
7. DNS: bản ghi A `<domain>` → IP server; kiểm `dig +short <domain> @1.1.1.1`.
8. Tạo `~/deploy/<instance>.sh` từ `reference/server-deploy-script.md` → deploy lần đầu → đăng nhập admin seed, đổi mật khẩu (`backend/DEPLOY.md` §10).

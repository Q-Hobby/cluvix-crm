# Deploy — Backend

Checklist khi deploy production: migration, env, auth hardening (device binding, audit log,
IP allowlist), nginx reverse proxy, systemd, health check, backup/restore. Biến bắt buộc ở prod
tóm tắt trong `README.md` mục 10; danh sách đầy đủ có chú thích: `.env.example` (đọc bởi `config/config.go`).

## 1. Migration (BẮT BUỘC — app KHÔNG auto-migrate)

Chạy qua migrate tool, KHÔNG `mysql < file.sql`:

```bash
go run cmd/migrate/main.go -cmd=up
```

Tất cả migration idempotent (`IF NOT EXISTS` / `information_schema` check) → chạy lại an toàn.
Quên bước này: bảng `user_device`/`security_audit_log` chưa tồn tại → device tracking +
audit sink fail (best-effort, KHÔNG chặn login).

## 2. Env vars

Mọi biến dưới đây đều có default an toàn → thiếu vẫn chạy. Nhưng `TRUSTED_PROXIES`
phải set đúng (xem mục 3).

| Biến | Default | Prod khuyến nghị | Ghi chú |
|---|---|---|---|
| `TRUSTED_PROXIES` | rỗng | IP/CIDR reverse proxy | ⚠️ Set đúng — mục 3 |
| `DEVICE_BINDING_MODE` | `off` | `log` → `enforce` | Rollout: đo `sec_event=refresh_fp_mismatch` ở `log` vài ngày rồi mới `enforce` |
| `IP_ALLOWLIST_MODE` | `off` | `off` | Bật `log`→`enforce` khi có KH cần chặn IP nhân viên |
| `TOKEN_MAX_SESSIONS` | `3` | (default) | Số thiết bị tối đa/user |
| `TOKEN_IP_BINDING` | `false` | `false` | Bind token vào IP — chỉ bật cho tài khoản IP tĩnh |
| `TOTP_ENC_KEY` | rỗng | **hex 64 ký tự** | ⚠️ Mã hóa TOTP secret 2FA. PROD BẮT BUỘC set (`openssl rand -hex 32`) — thiếu/sai định dạng ⇒ **từ chối khởi động** khi APP_ENV không phải dev. Đổi key = mọi user setup lại 2FA |
| `APP_ENV` | `develop` | `prod` | Chỉ `dev`/`develop`/`local` là môi trường dev (CORS phản chiếu mọi Origin). **Mọi giá trị khác (`production`, `staging`…) chạy chế độ prod** (fail-closed, 2026-10-18) |
| `EXPORT_FILE_TTL_HOURS` | `24` | (default) | File export trong `storage/export/<userID>/` cũ hơn ngần này giờ bị dọn lúc boot + mỗi giờ. Tải file qua `GET /api/download/export/:name` (có JWT, chỉ người tạo) — static `/download/export` đã bỏ |

⚠️ Prefix khoá Redis = `{APP_CODE}:{APP_ENV}:{DB_NAME}:` (từ 2026-10-18 có thêm `DB_NAME` để tách khách
dùng chung Redis). Lần deploy đầu sau thay đổi này: **mọi phiên đăng nhập hết hiệu lực một lần** (refresh
token nằm ở prefix cũ) — user đăng nhập lại; khoá cũ tự hết hạn theo TTL.

⚠️ Issuer/audience JWT = `{APP_CODE}.{APP_ENV}.{DB_NAME}.issuer|aud` và được KIỂM ở mọi request (R-1,
2026-10-18) ⇒ access token của khách A bị 401 ở khách B kể cả khi 2 khách lỡ chung `TOKEN_SECRET`. Vẫn
khuyến nghị `TOKEN_SECRET` riêng cho mỗi khách. Access token phát trước bản này (issuer chưa có `DB_NAME`)
bị 401 ⇒ FE tự refresh/đăng nhập lại — gộp chung lần "đăng nhập lại một lần" ở trên. Đổi `DB_NAME` của
một khách cũng làm mọi phiên của khách đó hết hiệu lực.

⚠️ Tải file export đi bằng XHR: `DOWNLOAD_URL` để trống (cùng origin với FE) hoặc origin của nó phải nằm
trong `CORS_ALLOWED_ORIGINS` — prod fail-closed CORS ⇒ thiếu là hỏng tải. Mỗi khách chạy một thư mục làm
việc riêng (`./storage/export/<uid>/` dùng uid, trùng nhau giữa các khách nếu chung thư mục) (R-8).

```env
# Tối thiểu cần thêm khi deploy:
TRUSTED_PROXIES=127.0.0.1        # ← điền IP nginx/LB thật
DEVICE_BINDING_MODE=log
IP_ALLOWLIST_MODE=off
```

### 2.1 Lưu file upload (`STORAGE_DRIVER=local|s3|r2`)

Hai kho tách biệt: **public** (avatar, logo, ảnh dịch vụ/chi nhánh/nhóm dịch vụ, ảnh TinyMCE — có URL
công khai) và **private** (file đính kèm khách hàng — KHÔNG có URL, chỉ tải qua
`GET /api/crm/attachment/:id/download` có JWT + quyền `viewCustomer` + scope khách như màn chi tiết khách).
DB lưu KEY (đường dẫn tương đối), URL ghép lúc trả về ⇒ đổi CDN/driver không phải sửa dữ liệu; giá trị cũ
là URL tuyệt đối (http/https) trả nguyên. **Sai/thiếu cấu hình của driver đã chọn ⇒ server DỪNG khởi động**
(không âm thầm rơi về local). `STORAGE_DRIVER` trống: có `S3_KEY` ⇒ s3 (cấu hình cũ, log cảnh báo), không ⇒ local.
Chi tiết từng biến + ví dụ: `.env.example`.

| Driver | Bắt buộc | Kho private |
|---|---|---|
| `local` (mặc định) | — (`STORAGE_PATH`, `STORAGE_PRIVATE_PATH`, `STORAGE_PUBLIC_PREFIX=/uploads`, `STORAGE_PUBLIC_BASE`) | thư mục `STORAGE_PRIVATE_PATH`, không bao giờ mount static; trùng/lồng thư mục public ⇒ dừng khởi động |
| `s3` | `S3_KEY`, `S3_SECRET`, `S3_BUCKET`, `S3_ENDPOINT` (hoặc `S3_REGION` cho AWS) | `S3_PRIVATE_BUCKET` (trống = `S3_BUCKET`), folder `<S3_FOLDER>/private`, ACL `private` |
| `r2` | như s3 + `S3_CDN` + `S3_PRIVATE_BUCKET` **khác** `S3_BUCKET` | bucket riêng không bật public (R2 không có ACL object) |

- `local`: 2 thư mục phải nằm ngoài vòng đời deploy (bind mount/volume) và nằm trong backup. FE khác origin
  với API ⇒ đặt `STORAGE_PUBLIC_BASE=https://<domain API>`; cùng origin ⇒ để trống + nginx `location /uploads/`
  (mục 6). KHÔNG bao giờ `alias`/`root` tới `STORAGE_PRIVATE_PATH`.
- `s3`: `S3_PUBLIC_ACL` trống ⇒ object public gửi ACL `public-read` (S3Cloud/MinIO chưa có bucket policy);
  bucket đã có policy public ⇒ `S3_PUBLIC_ACL=none`. ⚠️ Policy public-read TOÀN bucket làm file private đọc
  được ⇒ giới hạn policy vào folder public hoặc đặt `S3_PRIVATE_BUCKET` riêng.
- File đính kèm cũ (trước 2026-10-19, đã public-read trên CDN) vẫn tải được: endpoint download redirect 302 tới URL cũ.
- **Xem/phát trực tiếp bằng URL ký ngắn hạn** (video/audio tua được, ảnh/PDF xem trước, nút tải): FE gọi
  `POST /api/crm/attachment/:id/signed-url` (JWT + `viewCustomer` + scope như download) ⇒ `{url, expires_at, legacy}`.
  Hạn `STORAGE_SIGNED_URL_TTL` (mặc định 5 phút, trần 15 phút; nhận `300` hoặc `5m`).
  - `local`: URL `/api/files/signed/<token>` — route **công khai** (ngoài JWT), xác thực bằng chữ ký HMAC-SHA256
    với `STORAGE_SIGNING_KEY` (⚠️ **bắt buộc ở prod**, ≥ 32 ký tự, vd `openssl rand -hex 32`; thiếu ⇒ server dừng
    khởi động). Đổi khoá ⇒ URL đã cấp hết hiệu lực ngay. Hỗ trợ HTTP Range (206). Nginx: `location /api/` đã
    proxy về backend là đủ — KHÔNG cache route này ở proxy (`Cache-Control: private`).
  - `s3`/`r2`: presigned GET của bucket private (ký bằng `S3_SECRET`, kèm `response-content-type` +
    `response-content-disposition`). **Không cần CORS** trên bucket: URL dùng trực tiếp trong `<video src>`/`<a href>`
    (không phải XHR). Đồng hồ server lệch nhiều ⇒ URL presign bị từ chối — giữ NTP.
  - Bản ghi cũ (URL tuyệt đối) ⇒ trả nguyên URL cũ, `legacy=true`.

### 2.2 Chuyển file đính kèm cũ sang kho private (`cmd/attachment-privatize`, chạy 1 lần)

Khi nào: sau khi deploy bản có kho private, cấu hình storage đã đúng, server chạy ổn. Lệnh đọc `.env`
như app (DB + `STORAGE_*`/`S3_*`), in DB đích + driver + kho private ở đầu. Bản ghi `attachment` có `file_url`
là URL tuyệt đối ⇒ đọc object cũ (URL thuộc kho public hiện tại ⇒ đọc qua kho; còn lại ⇒ HTTP GET, CHỈ tới host
của `S3_CDN`/`S3_ENDPOINT`/`STORAGE_PUBLIC_BASE` hoặc `--allow-host`, không theo redirect sang host lạ, tối đa
32MB) → sniff MIME theo whitelist → ghi kho private key `crm/attachments/<customer_id>/…` → UPDATE `file_url`/
`file_type`/`file_size`. Bản ghi đã là key bị bỏ qua ⇒ chạy lại an toàn; lỗi 1 bản ghi ⇒ log, chạy tiếp.

1. Backup DB (mục 9).
2. Dry-run: `make attachment-privatize` (= `go run ./cmd/attachment-privatize`) — đọc số "Sẽ chuyển" và danh
   sách lỗi (host lạ ⇒ thêm `--allow-host cdn-cu.example.com` nếu đúng là CDN cũ của mình). Thử ít trước:
   `ARGS='--apply --limit 20'`.
3. Chạy thật: `make attachment-privatize ARGS='--apply'` — gõ lại tên DB để xác nhận (`--yes` cho script).
   Kiểm: tải vài file trên màn khách (đi qua `/download`, không còn 302); `SELECT COUNT(*) FROM attachment
   WHERE file_url LIKE 'http%'` chỉ còn các bản ghi lỗi trong báo cáo.
4. Mỗi lần `--apply` ghi manifest `attachment-privatize-<db>-<thời điểm>.csv` (`id,old_url,new_key`, đổi bằng
   `--manifest`) — GIỮ file này. Khi đã kiểm xong: `make attachment-privatize ARGS='--delete-public-from <file.csv>'`
   xoá object public cũ, chỉ khi bản ghi VẪN trỏ đúng `new_key`, object nằm trong kho public hiện tại và không còn
   bản ghi nào khác dùng chung URL (còn lại ⇒ "giữ lại", xoá tay). Muốn xoá ngay trong cùng lượt:
   `ARGS='--apply --delete-public'` (xoá CHỈ SAU khi UPDATE thành công). Cờ khác: `--db`, `--batch`, `--limit`, `--yes`.

## 3. TRUSTED_PROXIES + reverse proxy (điểm dễ sai nhất)

Gin chỉ tin `X-Forwarded-For` từ proxy trong `TRUSTED_PROXIES`. Chạy sau nginx/LB mà
**để rỗng** → `ClientIP()` trả IP proxy thay vì IP thật → **rate-limit login, IP binding,
IP allowlist, IP trong audit log đều sai** (thấy cùng 1 IP).

- Sau proxy: set `TRUSTED_PROXIES` = IP/CIDR nơi proxy kết nối tới Go (thường `127.0.0.1`
  nếu cùng máy, hoặc subnet LB).
- Go serve trực tiếp (không proxy): để rỗng.

nginx phải forward 2 header:

```nginx
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
```

`X-Forwarded-Proto: https` cần cho refresh cookie `Secure`/`SameSite=None` (đọc header
trực tiếp, không qua trusted-proxy gate — nên cookie đúng miễn nginx gửi header này).

## 4. Redis

Không cần config mới. Data auth mới (`rts:{uid}`, `rt:` có fingerprint, `user_scope` có
allowedIps, throttle `ip_deny_log:`, audit sink) dùng `REDIS_DB_AUTH` sẵn có,
backward-compatible với token/session cũ.

## 5. Sau deploy

- Theo dõi log `sec_event=refresh_fp_mismatch` (device binding) vài ngày ở `log` mode.
  Tỷ lệ gần 0 → đổi `DEVICE_BINDING_MODE=enforce`.
- Nhật ký bảo mật: super-user xem ở `config/security-audit` (FE).

## 6. nginx reverse proxy (TLS)

nginx terminate TLS, phục vụ bản build FE và proxy `/api` tới backend (`SERVER_HOST:SERVER_PORT`,
mặc định `127.0.0.1:8011`; để `TLS_CERT`/`TLS_KEY` rỗng ⇒ backend nói HTTP thường). Chi tiết cert/vhost:
`skill/skills/navigating-backend-architecture/reference/tls-certificates-and-nginx.md`.

```nginx
server {
    listen 443 ssl;
    server_name crm.example.com;
    ssl_certificate     /etc/ssl/crm.example.com/fullchain.pem;
    ssl_certificate_key /etc/ssl/crm.example.com/privkey.pem;

    root /var/www/cluvix-crm/frontend/dist/browser;   # thư mục output của `npm run build` (kiểm lại đường dẫn thực tế)
    index index.html;
    location / { try_files $uri $uri/ /index.html; }  # SPA fallback

    client_max_body_size 50m;                         # chỉnh theo giới hạn upload thực tế

    location /api/ {
        proxy_pass http://127.0.0.1:8011;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    location /uploads/ { proxy_pass http://127.0.0.1:8011; }   # chỉ cần khi STORAGE_DRIVER=local (kho public)
    location /download/templates/ { proxy_pass http://127.0.0.1:8011; }   # file mẫu import tĩnh, không chứa data khách
}
```

⛔ KHÔNG `alias`/`root` `/download/` (hay `/download/export`) thẳng tới thư mục `storage/` — file export
chứa PII khách. Chỉ tải qua `GET /api/download/export/:name` (cần JWT, chỉ người tạo). `TRUSTED_PROXIES`
phải khớp IP nginx kết nối tới Go (mục 3).

## 7. systemd service cho backend

Mỗi khách một unit + một `WorkingDirectory` riêng (có `.env` và `storage/` riêng):

```ini
# /etc/systemd/system/cluvix-crm.service
[Unit]
Description=Cluvix CRM backend
After=network.target mysql.service redis.service

[Service]
Type=simple
User=cluvix
WorkingDirectory=/opt/cluvix-crm/backend
ExecStart=/opt/cluvix-crm/backend/cluvix-crm
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload && sudo systemctl enable --now cluvix-crm
journalctl -u cluvix-crm -f
```

Quy trình cập nhật phiên bản: build binary mới → backup DB (mục 9) → `go run ./cmd/migrate -cmd=up`
(hoặc binary `migrate`) → `systemctl restart cluvix-crm`.

## 8. Health check

- `GET /health` — liveness: luôn 200 khi process sống (kèm trạng thái `decimal_column_map`/`json_column_map`), không đụng DB/Redis.
- `GET /health/ready` — readiness: ping DB + Redis (timeout 2s mỗi phần). 200 khi cả hai OK; **503** khi hỏng, body
  `{"status":"unavailable","checks":{"database":"ok|fail","redis":"ok|fail"}}` (không lộ host/DSN).
  Dùng cho monitor/LB; nginx có thể bọc `location = /health/ready { proxy_pass ...; }` nếu cần probe từ ngoài.

## 9. Backup / restore (`scripts/db-backup-all.sh`, `scripts/db-restore-all.sh`)

Chỉ thao tác DB được chỉ định rõ bằng `--db` và nằm trong `ALLOWED_DBS` (mặc định `cluvix_crm`; `cluvix_drc`
luôn bị chặn). Mật khẩu MySQL không đi qua tham số dòng lệnh (file `--defaults-extra-file` tạm chmod 600,
xoá khi thoát); đặt `MYSQL_USER`/`MYSQL_PWD` hoặc để script hỏi.

```bash
cd backend/scripts
ALLOWED_DBS=cluvix_khachA ./db-backup-all.sh --db cluvix_khachA --out /backup/khachA
# chứng minh dump restore được (DB tạm restoretest_*, tự DROP):
ALLOWED_DBS=cluvix_khachA ./db-restore-all.sh /backup/khachA --db cluvix_khachA --test
# restore THẬT (ghi đè): phải gõ lại đúng tên DB, hoặc --yes cho tự động hoá có chủ đích
ALLOWED_DBS=cluvix_khachA ./db-restore-all.sh /backup/khachA --db cluvix_khachA
```

Backup chỉ có giá trị khi gate PASS và bản sao nằm NGOÀI máy chủ.

## 10. Lần đăng nhập đầu & sau deploy

- Admin seed `admin` / `ChangeMe@2026` (công khai): hệ thống BẮT BUỘC đổi mật khẩu ở lần đăng nhập đầu
  (mọi API nghiệp vụ trả 403 `password_change_required` tới khi đổi).
- Sau lần deploy đầu có thay đổi prefix Redis/issuer JWT: mọi user đăng nhập lại một lần.

# Docker — môi trường DEV bằng 1 lệnh

Đây là **môi trường dev** (source mount từ host, hot reload cho cả Go lẫn Angular) —
KHÔNG phải image production. Build production xem `backend/DEPLOY.md` và `npm run build`
trong `frontend/`.

| Service | Container port | Port trên host | Ghi chú |
|---|---|---|---|
| `frontend` (Angular `ng serve`) | 4211 | **14211** | Proxy `/api` → `backend:8011`, HTTP thuần |
| `backend` (Go) | 8011 | **18011** | `go run .`, tự chạy migration trước khi serve |
| `mysql` (9.7) | 3306 | **13306** | DB `cluvix_crm_docker` |
| `redis` (7) | 6379 | **16379** | session / cache / rate limit |

Port host cố ý lệch port chuẩn để chạy **song song** với bản chạy trực tiếp trên máy
(air 8011, `ng serve` 4211, MySQL 3306, Redis 6379). Đổi được qua `*_HOST_PORT` trong `.env.docker`.

---

## 1. Chuẩn bị

Cài [Docker Desktop](https://www.docker.com/products/docker-desktop/) (có sẵn Compose v2).

```bash
docker --version
docker compose version
```

### 1.1. Tạo `.env.docker`

```bash
cp .env.docker.example .env.docker
```

`.env.docker` nằm trong `.gitignore`. Giá trị mẫu chạy được ngay, nhưng nên tự sinh secret riêng:

```bash
openssl rand -hex 32   # → TOKEN_SECRET
openssl rand -hex 32   # → TOTP_ENC_KEY (đúng 64 ký tự hex)
```

### 1.2. License + key PrimeUI

Layout (sidebar, topbar, menu…) là mã nguồn mở của dự án, có sẵn trong repo — không cần template
trả phí. Chỉ cần license key PrimeNG.

Mỗi dev làm frontend cần license PrimeUI riêng (Community miễn phí nếu đủ điều kiện, còn lại mua
Commercial) — cách đăng ký và lấy key: [README.md](README.md) §2.1–2.2.
`frontend/src/environments/environment.docker.ts` để `primeuiLicense: ''` và
`primeuiLicenseCdnUrl: ''`: chưa có key thì app vẫn chạy nhưng PrimeNG hiện banner đỏ "Invalid
PrimeUI License" (và dùng PrimeNG khi chưa có license là vi phạm điều khoản PrimeTek). Có key thì dán
vào `primeuiLicense` trong file đó **trên máy bạn, đừng commit** — có thể chạy
`git update-index --skip-worktree frontend/src/environments/environment.docker.ts` để git bỏ qua.

---

## 2. Chạy

```bash
docker compose --env-file .env.docker up --build
```

Lần đầu chậm (tải image, `go mod download`, `npm ci`, biên dịch Go, migration — vài phút). Các lần sau
nhanh nhờ named volume cache. Chạy nền: thêm `-d`.

Khi log hiện frontend compile xong, mở **http://localhost:14211**.

- Đăng nhập: `admin` / `ChangeMe@2026` (mật khẩu mặc định công khai — README §9). Hệ thống **bắt đổi
  mật khẩu ở lần đăng nhập đầu**, đổi xong phải đăng nhập lại.
- **Migration tự chạy** mỗi lần backend khởi động (`go run ./cmd/migrate -cmd=up` trong
  `backend/docker-entrypoint.sh`), kể cả seed chi nhánh + tài khoản admin trên DB rỗng.
- Kiểm backend: `curl http://localhost:18011/health`.

Sửa file `.go` hoặc `.ts` trên host: Angular tự reload. **Backend KHÔNG hot reload** (`go run`, không
có air trong container) — sửa Go xong chạy `docker compose --env-file .env.docker restart backend`.

---

## 3. Lệnh hay dùng

```bash
docker compose ps                                         # trạng thái
docker compose logs -f backend                            # log backend (hoặc frontend, mysql, redis)
docker compose --env-file .env.docker restart backend     # nạp lại code Go
docker compose --env-file .env.docker down                # dừng, GIỮ dữ liệu
```

Kết nối MySQL từ máy host: `mysql -h 127.0.0.1 -P 13306 -uroot -p` (mật khẩu = `MYSQL_ROOT_PASSWORD`).

### Chạy test trong container

```bash
# Test Go thường (không cần DB)
docker compose --env-file .env.docker run --rm --no-deps --entrypoint go backend test ./pkg/...

# Typecheck frontend
docker compose --env-file .env.docker run --rm --no-deps frontend npx tsc --noEmit -p tsconfig.app.json
```

Integration test Go cần **DB dùng-một-lần** qua `TEST_MYSQL_DSN` (test sẽ DROP mọi bảng!). TUYỆT ĐỐI
không trỏ vào `DB_NAME` đang chạy. Tạo DB riêng, ví dụ:

```bash
docker compose exec mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "CREATE DATABASE cluvix_test_tmp"
docker compose --env-file .env.docker run --rm --no-deps --entrypoint go \
  -e TEST_MYSQL_DSN='root:<MYSQL_ROOT_PASSWORD>@tcp(mysql:3306)/cluvix_test_tmp?parseTime=True' \
  backend test ./internal/...
docker compose exec mysql mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -e "DROP DATABASE cluvix_test_tmp"
```

(Xem `skill/skills/testing-backend/SKILL.md` để biết định dạng DSN chính xác.)

---

## 4. Reset dữ liệu

Xoá sạch DB + Redis (mất toàn bộ dữ liệu, kể cả admin đã đổi mật khẩu):

```bash
docker compose --env-file .env.docker down -v
```

`-v` xoá cả named volume cache Go/npm — lần `up` sau tải lại từ đầu. Chỉ muốn reset DB:
`docker volume rm <tên-project>_mysql-data` sau khi `down`.

---

## 5. Lỗi thường gặp

| Triệu chứng | Nguyên nhân / cách xử lý |
|---|---|
| `port is already allocated` | Port host bị chiếm — đổi `*_HOST_PORT` trong `.env.docker` |
| Backend `TOKEN_SECRET is required` | `.env.docker` thiếu hoặc chạy thiếu `--env-file .env.docker` |
| Backend `TOTP_ENC_KEY must be 64 hex chars` | Giá trị khác rỗng nhưng không phải 64 ký tự hex — `openssl rand -hex 32` hoặc để rỗng |
| Đổi `DB_NAME` / `APP_CODE` xong mọi người bị đăng xuất | Hai giá trị này nằm trong issuer JWT và prefix khoá Redis — chủ đích |
| `migrate up: detected dirty state` | Migration giữa chừng bị ngắt. Dev: `down -v` rồi `up` lại |
| Backend chưa healthy lâu, frontend chưa lên | Lần đầu đang biên dịch Go + migration; xem `logs -f backend` (healthcheck cho tới 5 phút) |
| Sửa code Go không có tác dụng | Backend không hot reload — `restart backend` |
| Angular không nhận thay đổi file (macOS/Windows) | Đã bật `--poll 1000`; file nặng có thể trễ vài giây |
| Upload/ảnh/đính kèm lỗi | Chưa cấu hình `S3_*` trong `.env.docker` — tính năng lưu file cần bucket S3/MinIO |
| Có `backend/.env` trên host, container vẫn chạy khác ý | `./backend` được mount nên godotenv đọc file đó cho biến chưa khai trong compose; các biến chính đã được compose đặt tường minh để thắng file |

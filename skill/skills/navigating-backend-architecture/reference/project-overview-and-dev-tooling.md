# Project Overview & Dev Tooling

> Gộp từ `skills/navigating-backend-architecture/reference/project-overview-and-dev-tooling.md` + `skills/navigating-backend-architecture/reference/project-overview-and-dev-tooling.md`.

## Mục lục

- ERP Cluvix
  - Coding Style
- Color Palette Rules — Cluvix Brand
- Common
  - Config / Integrations
  - Dev Tooling

---

# ERP Cluvix

ERP quản lý phòng khám. Monorepo: `frontend/` (Angular 22, PrimeNG 22, Tailwind — git repo riêng), `backend/` (Go 1.25, Gin, GORM, MySQL, Redis, S3), `be/` (PHP Yii2 — BE cũ, tham chiếu khi convert), `convert/` (tài liệu Yii2→Go).

Modules: crm, config, emr, med, hr, inventory, loyalty, marketing, zalo, ai, export.

- **Thương hiệu (user chốt 2026-09-04, làm rõ 2026-09-17)**: **Cluvix Solutions** (`cluvixsolutions.com`) = công ty (outsource, product);
  **CluvixHealth** (`cluvixhealth.com`) = sản phẩm SaaS cho phòng khám chuyên khoa (tên hiển thị "Cung cấp bởi CluvixHealth" giữ).
  Link/email trong code, issue template, footer widget trỏ về `cluvixsolutions.com` — KHÔNG tạo email/domain `cluvixhealth.vn`.
  Liên hệ/pháp lý đầy đủ cho marketing: `skill/bmad/personas/cluvixhealth-brand-kit.md` (file local, gitignored).

## Coding Style
- Trả lời tiếng Việt, code tiếng Anh
- Comment tiếng Việt giải thích logic trước mỗi function/method
- Model fields: comment tiếng Việt mô tả field dùng để làm gì, đặc biệt các field không rõ nghĩa
- DB access → Xem `skill/skills/writing-mysql-migrations/SKILL.md`

---

# Color Palette Rules — Cluvix Brand

**ĐÃ CHUYỂN** sang repo frontend (2026-07-16): `frontend/skill/skills/using-primeng-components/reference/ui.md` — canonical, đi theo repo FE để người làm FE dùng chung. Nội dung: Purple `#7B3F9E` primary duy nhất, accent Orange, danger Red, gradient Burgundy, neutral/semantic/chart colors, component quick reference, accessibility, don'ts.


---

# Common

| File | Khi nào đọc |
|------|-------------|
| `frontend/skill/skills/using-primeng-components/reference/ui.md` | Project overview + coding style; color palette/brand ĐÃ CHUYỂN → `frontend/skill/skills/using-primeng-components/reference/ui.md` |

## Config / Integrations

→ `skills/handling-auth-and-workspace/SKILL.md` — auth, indexeddb, workspace, zalo, service-worker, erp-option, crm options...

- **Thêm biến env mới đọc bằng `os.Getenv` → PHẢI khai vào CẢ `backend/.env` LẪN `backend/.env.example`,
  cùng lúc với code.** Chỉ nằm trong code là biến vô hình: `os.Getenv` trả rỗng, tính năng im lặng
  không chạy, và người muốn bật phải đọc source mới biết bật bằng gì.
  Thật: 3 biến `ZALO_WEBHOOK_PROBE*` (story-04, 2026-08-12) không được khai ở đâu cả ⇒ phép đo chữ ký
  webhook treo **11 ngày** không phải vì khó, mà vì không ai biết cách bật.
  Mỗi biến kèm comment nêu: bật để làm gì · giá trị mặc định và VÌ SAO mặc định thế · tác dụng phụ khi
  bật · công cụ tạm thì ghi rõ khi nào gỡ. `.env` gitignored nên comment ở `.env.example` là bản duy
  nhất người mới đọc được.

- **Biến env BẮT BUỘC ở prod (guard fail-closed trong `config.go`) → phải vá `.env` của TẤT CẢ tenant
  TRƯỚC khi deploy tenant đầu tiên.** Prod chạy 13 systemd unit `erp-cluvix@<tenant>`, mỗi tenant có
  `.env` + binary RIÊNG ở `/var/www/example.com/<tenant>/backend/`, deploy độc lập. Tenant chưa
  deploy vẫn chạy binary CŨ nên **không lộ lỗi** — `systemctl` báo `active (running)` bình thường dù
  `.env` đã thiếu biến, thành mìn hẹn giờ nổ ở lần deploy sau.
- Panic lúc bootstrap = systemd restart vô hạn. Log app nằm ở `/var/www/logs/<tenant>-error.log`,
  **KHÔNG** ở `journalctl` (unit dùng `StandardError=append:`) — `journalctl` chỉ thấy vòng lặp restart,
  không thấy nguyên nhân.

## Dev Tooling

- Script `.sh` trong repo: **KHÔNG dùng `declare -A`** (associative array) — macOS mặc định bash 3.2, sẽ `unbound variable` lúc chạy. Dùng hàm `case` thay thế. `bash -n` KHÔNG bắt được lớp lỗi này, chỉ lộ khi chạy thật.
- Playwright MCP đã cài (`claude mcp add playwright -- npx -y @playwright/mcp@latest`) — dùng khi cần agent tự chạy/test UI thật (navigate/click/screenshot). Tool chỉ load sau khi mở session Claude Code mới (không refresh giữa chừng được).
- `backend/scripts/run_dev.sh` (air wrapper) có **lock 1-instance** (thư mục `backend/.run/`, khoá tên `run_dev.lock` tạo bằng `mkdir` atomic — CHỈ tồn tại lúc server đang chạy, nên đừng tìm nó khi máy rảnh; đặt NGOÀI `tmp/` vì air `clean_on_exit` xoá sạch `tmp/`): instance thứ 2 chờ ~10s rồi `exit 1`, KHÔNG giành port; lock mồ côi (chủ bị SIGKILL) tự thu hồi. Thấy `status=137` lặp liên tục trong `tmp/run.log` = có 2 instance chia nhau `tmp/run_dev.pid` → **không phải OOM**; gỡ: kill instance thừa. ⚠ Thông báo do chính script in ra ghi `./tmp/run_dev.lock` là SAI đường dẫn — đừng làm theo.
- **Deadlock wrapper mồ côi (air không lên lại được sau khi sửa code)**: `run_dev.sh` mồ côi (PPID=1) còn sống giữ lock **hợp lệ** → air mới build xong nhưng `run_dev.sh` mới `exit 1`, air thoát và xoá `tmp/`; port 8081 vẫn phục vụ **binary CŨ**. Dấu hiệu: `lsof -nP -iTCP:8081 -sTCP:LISTEN` ra PID có `ELAPSED` rất dài trong khi mtime binary lại mới. Gỡ: `kill <PID run_dev.sh> <PID con>` rồi chạy lại `air` — SIGTERM chạy trap EXIT tự nhả lock, lock còn sót cũng tự thu hồi vì owner đã chết, KHÔNG cần `rm -rf`.
- air `include_ext = ["go"]` ⇒ đổi file **non-`.go`** (artifact `go:embed` như `permissions.gen.json`, `route_perm_map.json`) KHÔNG trigger rebuild — BE vẫn chạy bản cũ. Ép rebuild: **ghi lại nội dung** 1 file `.go` (`cp f /tmp/x && cp /tmp/x f`); `touch` KHÔNG đủ (macOS fsnotify không bắn WRITE khi chỉ đổi mtime). air **không** tự respawn `run_dev.sh` nếu nó bị TERM — đừng kill supervisor rồi ngồi đợi.
- **Xoá cache app bằng `redis-cli` — key PHẢI có prefix `<APP_CODE>:<ENV>:app:`**: `cache.New(..., cfg.CachePrefix(), cfg.RedisDBCache)` ⇒ key thật là `CluvixDev:dev:app:<key>` (cache ở Redis DB `REDIS_DB_CACHE`=0, auth ở `REDIS_DB_AUTH`=1). `DEL <key>` không prefix trả `0` = "không có key", **KHÔNG** phải "đã xoá" — kiểm `EXISTS` trước rồi mới `DEL`. `user_scope:<uid>` TTL 5 phút: sửa `user_profile.companyActive_id` bằng SQL mà không xoá key này thì BE vẫn phục vụ scope cũ.
- **Agent KHÔNG BAO GIỜ tự chạy backend server** (air/`go run`/`./tmp/main`) để build/test/verify — port 8081 luôn là dev server user đang chạy qua air. Chỉ dùng `go build`/`go vet`/`go test` (không bind port). Cần gọi API thật → hỏi user xác nhận air đang chạy port nào rồi gọi vào đó, không tự spin up instance riêng (2 instance cùng giành port 8081 → đúng vòng lặp kill -9 ở trên).

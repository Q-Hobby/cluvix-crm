# Cluvix Skill Index

Go BE / Angular FE / MySQL multi-tenant ERP.

**Quy tắc đọc:** Luôn đọc file này trước. Khi làm việc với domain cụ thể → đọc file tương ứng.

**Cập nhật skill/:** Sau mỗi session → xem `skill/HOW_TO_UPDATE.md` trước khi ghi bất kỳ thứ gì.

---

## Development Protocol

### 1. Understand First
- Ambiguous requirements → stop and ask.
- Multiple interpretations → present options.
- Identify:
  - Business process impact
  - User roles & permissions
  - Reporting impact
  - Audit trail requirements
  - Data migration impact

### 2. Design (Medium/Large Changes)
Required for:
- New features
- Database/schema changes
- Cross-module changes
- Architecture changes

**Small fixes (1–3 lines, single file) → skip to step 3.**

- Present a short plan: `1. [Step] → verify: [check]`
- Identify affected modules and DB impact.
- Wait for approval before major implementation.

### 3. Implement
- Solve only the requested problem.
- Prefer the simplest solution.
- No speculative features.
- Make surgical changes only.
- Every change must trace back to the requirement.
- Follow existing architecture and patterns.
- Reuse before creating new abstractions.
- Minimize breaking changes.

### 4. Validate
- Verify requirements are satisfied.
- Verify edge cases.
- Build passes (`go build ./...` / `npx tsc --noEmit`).
- Lint passes.
- Every changed line traces back to the requirement.
- Confirm plan objectives are met.

### ⚠ Correctness over Speed
Never optimize for getting the task done quickly.
Optimize for correctness, maintainability, and long-term architecture.

If the only solution you can find is a workaround:
- Say so explicitly before implementing it.
- Keep investigating the root cause — the correct solution usually exists.
- A workaround that hides a structural problem is worse than no solution.

---

## Hard Rules — LUÔN áp dụng, kể cả fix nhỏ không đọc thêm file nào

- **Mô hình tenant (deployment, D15/E6 — 1 thương hiệu/DB, thay mô hình multi-company cũ)**: 1 database = 1 khách hàng SaaS (DB-per-customer) = **1 thương hiệu** — các khách SaaS khác nhau KHÔNG BAO GIỜ chung DB. KHÔNG còn khái niệm `company`/`company_id`: schema mới (story-62) đã bỏ hẳn cột này. `branch_id` là trục scope DUY NHẤT còn lại, phân tách các chi nhánh CÙNG MỘT thương hiệu bên trong DB (vd 1 thương hiệu có 2 chi nhánh). Thông tin thương hiệu (tên, logo, timezone, tiền tệ, 2FA, IP allowlist) đọc qua `pkg/brand` (bảng 1 dòng `business_profile`, D22) — KHÔNG đọc bảng company cũ, KHÔNG tự chế cache riêng. KHÔNG thiết kế feature giả định nhiều khách SaaS trong 1 DB: không cross-DB query, không global-unique cross-customer; unique/codegen/cache chỉ cần đúng trong phạm vi 1 DB.
- **Tenant scope**: GET/UPDATE/DELETE theo id phải qua `tenant.ScopedDBNullBranch(c, db.DB, "table")` — `ScopedDB` DEPRECATED. FK lookup (load record liên quan, cố ý không lọc branch — vd customer của appointment) dùng `tenant.CrossBranchLookup(db, "table")`/`CrossBranchLookupScope` (thay `ScopedDBCompanyOnly` cũ, đã xoá — story-41). KHÔNG `db.DB.First(&item, id)` trực tiếp. Record `branch_id` IS NULL LUÔN được lấy kể cả khi active; raw SQL fragment phải bọc ngoặc `AND (x.branch_id = ? OR x.branch_id IS NULL)`.
- **Tiền**: `decimal.Decimal` (type `decimal(20,2)`), KHÔNG float64, KHÔNG toán tử `+ - * /` — dùng `.Add/.Sub/.Mul/.Div`.
- **Response**: helpers `response.*` + message qua `i18n.T(c, "key")` cho CRUD — KHÔNG `c.JSON()` trực tiếp, KHÔNG hardcode string. Ngoại lệ: list handler dùng `c.JSON(http.StatusOK, result)` sau `pagination.Paginate`.
- **Migration**: CHỈ `.up.sql`, KHÔNG `.down.sql`. Timestamp 14 ký tự từ `date +%Y%m%d%H%M%S`, phải lớn hơn file mới nhất VÀ lớn hơn `SELECT MAX(version) FROM schema_migrations` trên DB đích (nhiều phiên chạy song song → `date` có thể nhỏ hơn version đã apply; tên file ≤ version hiện tại thì golang-migrate bỏ qua vĩnh viễn, không lỗi không cảnh báo). Timestamp columns: `BIGINT` Unix, KHÔNG `DATETIME`. **Idempotent bắt buộc**: `ADD COLUMN` check tồn tại qua `information_schema.COLUMNS` + `PREPARE/EXECUTE` (MySQL 8.0 KHÔNG support `ADD COLUMN IF NOT EXISTS`); `DROP COLUMN/FK/INDEX` dùng dynamic lookup qua `information_schema` + `PREPARE/EXECUTE` — KHÔNG hardcode tên constraint, KHÔNG raw `ALTER TABLE DROP` trực tiếp. Lý do: migrate có thể chạy lại file cũ khi dirty state, non-idempotent sẽ fail và block toàn bộ deploy.
- **FK column PHẢI có FOREIGN KEY constraint thật (CRITICAL)**: mọi cột tham chiếu tới bảng khác (`xxx_id`) — kể cả self-reference (`parent_id`) — BẮT BUỘC `ADD CONSTRAINT ... FOREIGN KEY ... REFERENCES ... ON DELETE ...`, KHÔNG chỉ tạo `KEY`/`INDEX` rồi coi như xong. Lý do: `models.DeleteWithCheck` (`internal/models/base.go`) là cơ chế DUY NHẤT chặn xóa record đang bị tham chiếu — nó chỉ bắt lỗi MySQL 1451 (FK violation) khi DELETE, KHÔNG tự kiểm tra bảng con ở tầng app. Bảng thiếu FK thật → `DeleteWithCheck` không bảo vệ được gì, xóa parent xong con vẫn mồ côi mà không báo lỗi. Trước khi `ADD FOREIGN KEY` trên bảng đã có data: query check orphan trước (xem `skill/skills/writing-mysql-migrations/SKILL.md`), nếu có phải xử lý (SET NULL/gán lại) trước khi apply constraint — không thì migration fail ngay.
  - ⚠ **NGOẠI LỆ DUY NHẤT: `created_by` / `updated_by` — CẤM thêm FK cho 2 cột này (user chốt 2026-09-05, migration `20260905223106_drop_audit_fk_user.up.sql` đã bỏ 244 FK)**. Cột audit vẫn giữ `KEY`/`INDEX` (cho JOIN lấy tên người tạo/sửa) nhưng KHÔNG có constraint. Lý do đo được, không phải phỏng đoán: MySQL 9.7 trả giá **~5ms cho MỖI FK con trên MỖI lần UPDATE một dòng của bảng cha**, kể cả khi cột được tham chiếu (`user.id`) KHÔNG hề đổi. `user` từng bị 355 FK con trỏ tới ⇒ mọi `UPDATE user` tốn **1.9s** trên máy rảnh + DB rỗng (bản sao y hệt bỏ hết FK: **0.001s**); trên production đã nở thành **29.6s cho một request `PUT /api/hr/user/:id`** (toàn bộ code Go trong request đó chỉ 0.24s). Bỏ 244 FK audit ⇒ còn 0.42s. Đổi lại: 235/244 FK đó là `ON DELETE RESTRICT`, tức trước đây **không xoá được bất kỳ nhân viên nào đã từng tạo/sửa 1 record** — FK audit vừa vô ích (một `created_by` mồ côi chỉ làm cột "người tạo" hiển thị rỗng, không sai nghiệp vụ) vừa chặn thao tác hợp lệ. **111 FK quan hệ thật còn lại (`user_id`, `doctor_id`, `changed_by`, `approved_by`…) GIỮ NGUYÊN** — 120 nơi gọi `DeleteWithCheck` đang dựa vào chúng. Hệ quả bắt buộc nhớ: `created_by`/`updated_by` nay có thể trỏ tới user đã xoá ⇒ mọi JOIN lấy tên người tạo/sửa PHẢI là `LEFT JOIN` (đã verify: 54/54 chỗ trong repo đều đã là LEFT JOIN). Quy tắc chung rút ra: bảng cha bị **>100 FK con** thì mỗi UPDATE dòng cha đã tốn ~0.5s — cân nhắc trước khi thêm FK vào bảng bị tham chiếu dày (đo 2026-09-22 trên DB dev: `user` 132, `company` 93, `customer` 57 — `customer` mỗi UPDATE đo được 35–75ms lúc rảnh, ~150ms trong transaction đang giữ khoá; con số trôi theo thời gian, tự đo lại bằng `information_schema.REFERENTIAL_CONSTRAINTS`).
- **Layout `internal/modules/` 4 nhóm (restructure 2026-07-28)**: `webapp/` (23 domain module web-app: crm, emr, hr, accountant, config, omnichat...), `client/` (bên thứ 3 PartnerAuth), `mobile/` (BFF app staff), `public/` (webhook/callback/livechat-visitor không auth — chỉ route + handler mỏng, logic nặng ở webapp; import MỘT CHIỀU public→webapp, KHÔNG có chiều ngược). Knowledge base đã sửa hết path (2026-08-17, story-03) — không còn phải tự dịch `internal/modules/<domain>` → `webapp/<domain>` nữa; **file plan/story trong `bmad/` thì KHÔNG sửa** (ghi chép lịch sử, đúng với thời điểm viết) nên đọc chúng vẫn cần tự dịch. Gate kiểm: `skill/skills/_tooling/verify-paths.sh`. Route public mới (webhook/callback) → đăng ký trong `public/`, KHÔNG thêm `RegisterPublic` trong module webapp.
- **Entity mới**: vào `internal/modules/webapp/<domain>/<resource>/dao/entity.go` — KHÔNG tạo file trong `internal/models/`.
- **KHÔNG `SELECT *`** — explicit select theo field FE cần.
- **Path param id PHẢI parse ra số trước khi vào query**: `Where("id = ?", c.Param("id"))` để MySQL ép `'5abc'` → `5` (warning 1292 bị nuốt) ⇒ URL rác **sửa/xoá trúng record thật** trên đường GHI. Dùng `helpers.ParseID(c)` (param tên khác: `helpers.ParseParamID(c, "assignmentId")`), không parse được ⇒ 404; hàm service nhận id thì khai `int`/`uint`, KHÔNG `string`, để compiler gác. **Áp cho CẢ query param tên id** (`c.Query("idCustomer")`) — cùng cơ chế ép kiểu, chỉ khác cửa vào; ở đó giữ nguyên response cũ của guard `== ""`, chỉ thêm phép parse. Tồn kho: 119+38 điểm đã VÁ HẾT 2026-09-07 (còn 1 điểm `med/dqg_config` chờ phiên khác xong) — gate `skill/skills/_tooling/verify-id-params.sh` nay phủ cả `c.Param` lẫn `c.Query`, baseline = 1.
- **Timezone**: `loc := tz.LocFromContext(c, db.DB)` — KHÔNG `time.Local` / `time.Now().Location()`.
- **Constants**: `pkg/define/` — KHÔNG magic number/string.
- **Sau CUD**: `bumpDexieVersion()` + xóa cache liên quan.
- **Verify**: `go build ./...` / `npx tsc --noEmit` trước khi báo xong.
- **KHÔNG tự chạy server bind port 8081 — cần server thật thì dùng `scripts/run_qa.sh` (port 8079)**: user luôn có sẵn `air` (hot-reload dev server) chạy nền trên 8081. Thủ phạm giành port LUÔN là việc chạy **binary app** (`go run .`, `./tmp/main`, tự start `air`) — nó đọc `SERVER_PORT=8081` từ `.env`; `go test` KHÔNG gây ra (test dựng server dùng `httptest.NewServer` = port ephemeral). Hậu quả nặng hơn "đụng port": `scripts/run_dev.sh` CỐ Ý không kill process lạ mà `exit 1` → **air không tự phục hồi, dev server chết hẳn tới khi user kill tay**. Cũng KHÔNG `go build -o ./tmp/main` (ghi đè chính binary air đang chạy → air restart vào binary của agent).
  - Verify mặc định: `go build ./...` (không chạy binary) hoặc `go test ./path/...`. Cần log runtime → đọc `backend/tmp/run.log`, KHÔNG spawn process mới.
  - ⚠ **`go build ./...` đỏ KHÔNG có nghĩa việc của mình sai** (dính 2026-09-07): phiên Claude khác đang sửa dở một package là cả cây đỏ theo — hôm đó `pkg/dqg` + `med/dqg_config/dao` đang giữa chừng bỏ field `ApiKey`. Trước khi truy lỗi: `git status --porcelain` xem package đỏ có nằm trong danh sách file MÌNH sửa không; không nằm ⇒ để nguyên, verify bằng build **theo scope file mình đổi** (`go build $(git status --porcelain -- internal/ | ... | xargs -n1 dirname | sort -u)`) và nói rõ trong báo cáo là full build đỏ vì việc của phiên khác. Sửa hộ họ = giẫm lên uncommitted work.
  - Thật sự cần server sống (smoke endpoint, curl thử) → **cửa DUY NHẤT: `backend/scripts/run_qa.sh`** — port 8079, build ra `backend/tmp/qa/` (KHÔNG phải `tmp/main`), tự kill process + xoá binary khi thoát/Ctrl-C/hết `--timeout` (mặc định 120s). Từ chối chạy nếu port đã bận và KHÔNG tự kill process lạ. **Trước khi gửi request đầu tiên: ĐỌC dòng `run_qa.sh: DB đang dùng = ...` và xác nhận đúng DB dùng-một-lần** — đừng tin biến `DB_NAME` mình đặt ở đầu lệnh. Script từng gán đè vô điều kiện từ `.env` khiến server QA chạy thẳng vào DB dev (đã vá 2026-08-19 để env của caller thắng, nhưng đọc-trước mới là lưới thật). Server QA phục vụ **HTTPS**: `curl -sk https://localhost:8079/...`; gọi `http://` chỉ nhận `TLS handshake error` trong log và request KHÔNG tới handler — dễ tưởng nhầm là app hỏng. Lệch DB → kill ngay rồi truy vết tác dụng phụ (bảng nghiệp vụ trong cửa sổ thời gian + `backend/tmp/qa/` log): server khởi động là chạy luôn tác vụ nền trong process (`helpers.RunBackground`: dọn `user_device`, dọn audit...) nên server sống 1 phút cũng đủ ghi/xoá dữ liệu trên DB đích. (Scheduler/cron đã bỏ 2026-10-17 — bài học giữ nguyên với mọi tác vụ nền còn lại.)
  - ⚠ File/thư mục tạm phải nằm trong `backend/tmp/` — air watch cả **thư mục mới** ở gốc `backend/` (không chỉ file `.go`), tạo `backend/.qa/` là đủ kích rebuild → kill server (đã đo 2026-08-17).
  - ⚠ Nhưng **KHÔNG đặt file `.go` vào `backend/tmp/`** (dính 2026-09-07): `tmp/` nằm TRONG module nên `go build ./...` biên dịch luôn. Copy `internal/` ra `backend/tmp/backup` để thủ sẵn trước một đợt sửa hàng loạt ⇒ build đỏ ngay với `use of internal package erpCluvix/internal/zago/internal/... not allowed` — lỗi không hề nhắc tới thư mục tạm, rất dễ tưởng mình vừa phá `internal/zago`. Hai rule này kéo ngược nhau: file tạm **không phải `.go`** (log, SQL, script `.py`/`.sh`, dump) → `backend/tmp/`; bản sao code `.go` / cây backup → **thư mục scratchpad của phiên, NGOÀI module** (Claude Code có sẵn scratchpad riêng). Binary spike: `go build -tags spike -o backend/tmp/<tên>`.
- **CẤM test/migrate chạm DB thật (`DB_NAME` trong `.env`) — CATASTROPHIC (bài học 2026-07-18)**: `testhelpers.New` (`dropAllTables` = DROP mọi bảng), integration test, `cmd/migrate` khi verify CHỈ chạy trên **DB dùng-một-lần** (tạo `*_test`/`*_tmp` đầu → drop cuối). TUYỆT ĐỐI không trỏ `TEST_MYSQL_DSN`/`-db` vào `DB_NAME`. **Plan/story KHÔNG BAO GIỜ ghi tên DB dev thật làm target verify backfill** — cần data thật thì clone ra `*_tmp` rồi drop. Mọi seed idempotent (`INSERT IGNORE` khi khoá seed có UNIQUE thật / `WHERE NOT EXISTS` khi không — vd `setting.key`, `user_option` theo khoá nghiệp vụ; KHÔNG raw INSERT id cố định). Guard code là lưới cuối, KHÔNG thay kỷ luật. Sự cố + kịch bản nếu dính production (mất sạch data 1 khách, DB-per-customer): `skill/skills/writing-mysql-migrations/SKILL.md` (mục ⛔ đầu file).
- **`frontend/` nằm CHUNG repo gốc (KHÁC ERP cũ — CRM standalone không tách repo)**: `git status`/`git diff` chạy từ root ĐÃ thấy thay đổi trong `frontend/`, không cần `cd frontend` chạy git riêng.

**Lưới cưỡng chế bằng máy**: KHÔNG có (repo CRM đã bỏ toàn bộ hooks — user chốt 2026-09-23). Các rule trên chỉ còn dựa vào kỷ luật; DB CRM là `cluvix_crm`; TUYỆT ĐỐI không chạm database của hệ thống KHÁC cùng máy MySQL (tên cụ thể trên máy dev: `CLAUDE.local.md` ở gốc repo — file local, gitignored; agent không tự đọc file đó thì hỏi user trước khi chạm bất kỳ DB nào khác `cluvix_crm` hoặc `*_tmp`).

Chi tiết + lý do từng rule: `skill/skills/backend-hard-rules/SKILL.md`.

---

## Subagent Model Policy — Planner/Executor (user chốt 2026-07-20, thay bảng cũ)

### Planner — Fable 5 (main agent, KHÔNG code)
- Requirement analysis · architecture discussion · review implementation · create plan.md · điều phối wave · QA/adversarial synthesize.
- Main agent LUÔN chạy model user chọn — KHÔNG tự đổi; KHÔNG dùng Fable cho dev story, kể cả diện nhạy cảm.

### Executor — chọn model theo loại việc
| Model | Loại việc | Ví dụ repo này |
|---|---|---|
| `opus` | Complex business logic, authentication/security, refactoring, cross-module, large context, transaction/idempotency, migration đổi hành vi, hash/ký số, backfill/dual-write | Inventory flow, accounting logic, payment/treatment, permission/auth |
| `sonnet` | CRUD, API, UI, reports, settings, forms, tests, medium complexity, research + tổng hợp có phân tích | API CRUD, Angular form, PrimeNG table, report |
| `haiku` | Small edit, boilerplate, rename, comment, docs thuần, tiny refactor, grep/verify/đếm cơ học | Migration nhỏ, rename, comment, docs |

### Ràng buộc vận hành
- Giữ ≤2 executor song song (user online cho phép nâng 5-10); wave theo dependency, không 2 agent cùng sửa 1 file.
- **Disk khi fan-out dev Go (sự cố ENOSPC 2026-07-21)**: >4 executor build Go song song → Go build cache phình rất nhanh (19GB/buổi) → đĩa đầy 100%, MỌI agent (kể cả main) tê liệt vì harness không ghi được output. Trước wave lớn: `df -h /System/Volumes/Data` phải còn ≥15GB; giữa các wave: `rm -rf ~/Library/Caches/go-build` khi cần (`go clean -cache` có thể fail "directory not empty"). Agent chết vì ENOSPC: partial state thường nhất quán + report chứa resume plan — đọc report trước khi relaunch, đừng chạy lại mù.
- **RAM khi fan-out verify build/test (sự cố 2026-07-28)**: mỗi Dev/QA persona đều tự chạy full build/test làm gate (BE `go build ./...`, mobile `flutter analyze`+`flutter test`, FE `npx tsc --noEmit`) — ĐÚNG chủ đích (QA không tin lời Dev tự khai), KHÔNG bỏ. Nhưng khi nhiều executor song song + `air` tự rebuild mỗi lần save (BE) cùng lúc, nhiều lệnh build full-repo chồng lên nhau (mỗi `go build` full monorepo spawn nhiều `compile` worker, không throttle giữa các lệnh) → RAM vọt 30-40GB. Giảm bằng: (1) Dev/QA build **scope theo path đã đổi** (`go build ./internal/modules/webapp/<module>/...`) thay vì luôn `./...`; full `./...` chỉ chạy 1 lần cuối wave, không mỗi story; (2) set `GOFLAGS=-p=4` giảm parallelism/lệnh; (3) BE: air đã tự rebuild sẵn — ưu tiên đọc `backend/tmp/run.log` để biết build fail/pass thay vì tự chạy lại `go build` khi chỉ cần biết "có lỗi compile không" (lưu ý air chỉ build main package, KHÔNG phủ `cmd/migrate`/`cmd/recalc-reconcile` — story đụng các cmd/ đó vẫn phải tự build riêng). (4) Nghi ngờ air bị treo/ăn RAM bất thường → check orphan: `ps -eo pid,ppid,etime,command | awk '$2==1' | grep -E "compile|go build"` (PPID=1 = mồ côi do air chết bất thường không cascade-kill con) — kill an toàn, không phải server đang serve request. Kích hoạt hay gặp nhất: vừa `rm -rf ~/Library/Caches/go-build` → air rebuild toàn cây, dễ chồng nhiều lượt thành orphan làm build treo hàng chục phút — dọn cache xong nên check orphan ngay. **(5) `-race` là ngoại lệ phá vỡ rule `-p=4` ở trên (đo lại 2026-08-31)**: cùng 1 package, `go test -c` peak 0.98GB nhưng `go test -c -race` peak **4.60GB** (~4.7×). `go test -race ./internal/modules/webapp/<module>/...` = 13 package có test × `-p=4` ⇒ **~18GB từ MỘT lệnh của MỘT agent**, trong khi nền máy dev lúc rảnh đã 11GB/16GB (GoLand 4.0 + node AngularService 1.7 + mysqld 1.1 + Chrome 2.5 + các phiên claude 1.8) — 2 agent QA song song ⇒ ~47GB nhu cầu trên 16GB RAM, swap cạn, máy đơ cứng. Đây là lý do agent tuân thủ đúng `-p=4` mà user VẪN bị đơ lặp đi lặp lại. Luật: có `-race` ⇒ **luôn `-p 1`**; `-race` chỉ chạy trên package thật sự có concurrency (package có goroutine), KHÔNG quét cả `./<module>/...`; **KHÔNG 2 agent cùng chạy `-race`** (race phải xếp tuần tự). Triệu chứng user báo "terminal chiếm 40GB" = Activity Monitor chế độ hiển thị phân cấp cộng dồn `compile`/`link`/test binary vào dòng `Terminal`, KHÔNG phải terminal rò rỉ. Chi tiết + cách tự tính ngân sách `-p`: `skill/skills/testing-backend/SKILL.md`.
- File dùng chung nhiều story cùng sửa (`pkg/tenant/scope.go`, `config/router.go`, `indexeddb/sync.go`, `route_perm_map.json`, `pkg/i18n/*`): edit thuần additive + đọc bản mới nhất ngay trước khi sửa; story đụng cùng MODULE (không chỉ cùng file) → không chạy song song.
- **CẤM `git checkout/restore/stash` file dùng chung khi fan-out (sự cố 2026-07-21, tái phát 2026-08-12)**: file chung chứa uncommitted work của agent khác — checkout là XÓA VĨNH VIỄN không recover được (`git fsck` vô ích với thứ chưa staged). **`git stash` cũng cấm**: nó quét NGUYÊN working tree, cuốn cả việc của phiên khác (thật: ~170 file), và `stash pop` FAIL nếu giữa chừng họ ghi tiếp vào 1 file đã stash. Cần so với bản HEAD → `git show HEAD:<file>` ra scratchpad, KHÔNG đụng working tree. Lỡ làm hỏng file chung → sửa bằng Edit ngược lại chính xác phần mình gây ra, KHÔNG revert cả file. JSON registry lớn: KHÔNG parse-rewrite bằng script (đổi format toàn file) — chỉ Edit anchor additive. Cũng CẤM `rm -rf <thư mục dùng chung>` khi mình chỉ tạo thư mục CON trong đó (`backend/spike/` chứa spike của nhiều phiên) — xoá đúng thư mục con của mình. Lỡ xoá file đang được git theo dõi: khôi phục bằng `git show HEAD:<file> > <file>` từng file (hook chặn `git checkout -- `), rồi `git status` phải rỗng mới coi là xong. ⚠ Chỉ dùng khi HEAD chứa bản đúng — epic chưa commit (HEAD = snapshot cũ) mà chép HEAD đè lên là xoá sạch việc chưa commit của cả epic (2026-09-25: mất catalog quyền sau khi "hoàn tác" `perm:codegen`); muốn thử script có ghi file → chạy trên bản copy ngoài repo.
- **Fan-out: anchor edit KHÔNG chống được xung đột SỐ HỌC (sự cố 2026-07-29)**: file dùng chung — KHÔNG để 2 agent cùng cộng dồn vào một con số tổng / bảng đối chiếu. Anchor edit chỉ chống xung đột văn bản; 2 agent đọc bản mới nhất rồi cộng từ **base khác nhau** sẽ cho 2 tổng khác nhau mà mỗi bên đều "đúng theo bản mình đọc" (thật: một SKILL.md kho vật tư (đã xoá khỏi CRM) có 3 tổng mâu thuẫn 57/68/57). Cách tránh: 1 agent giữ quyền ghi con số tổng, agent khác chỉ append mục + Δ, Planner cộng cuối.
- **Quét mutation trước khi commit**: QA adversarial cố ý cấy `if false &&`/cờ `*_MUTATION` vào code sản phẩm để chứng minh lưới đỏ được. Bỏ quên 1 cờ = vô hiệu hoá đúng bản vá vừa làm, mà **build xanh + test qua**. Trước commit: `grep -rn "MUTATION\|if false &&" --include='*.go' internal/ pkg/` phải = 0. Phiên 2026-07-29 đã có 2 lần cờ nằm trong code sản phẩm giữa lúc agent khác chạy. ⚠ **Phải QUOTE `'*.go'`** (sửa 2026-08-11): viết trần `--include=*.go` thì zsh nuốt glob, báo `no matches found` rồi trả **0 hit** → lưới an toàn LUÔN xanh kể cả khi còn cờ mutation thật. Cùng lỗi lớp này: pattern grep có `+`/khoảng trắng phải dùng `-E` và quote — vd `'Status \*int'` (đúng 1 space) KHÔNG khớp field căn lề nhiều space, phải `-E '^[[:space:]]*Status +\*int'`. Cùng lớp, chiều NGƯỢC lại: mọi lệnh **sửa** hàng loạt (`str.replace`, `sed`) **không khớp thì im lặng thành công** — bắt buộc `assert` số chỗ đã thay. Và khi đổi hành vi, đọc lại test đang phủ nó: test assert hành vi CŨ sẽ giữ suite xanh xuyên qua bản vá (2026-08-18: commit tuyên bố đính kèm chạy trong khi nhánh chặn vẫn còn nguyên). Cùng lớp, 3 biến thể nữa (2026-08-24): (a) **biến shell chứa nhiều path** (`grep ... $P` với `P="a b c"`) → shell không tách từ, grep coi là MỘT tên file, in `No such file or directory` rồi trả **0** — kiểm `ls` path trước; với `git diff/status -- $P` còn tệ hơn: KHÔNG in lỗi, trả rỗng exit 0 (2026-09-02, suýt kết luận "0 vi phạm Hard Rules" từ bộ quét chạy trên tập file rỗng); cùng gốc, biến thể **vòng lặp** (2026-09-10): `for x in $f` với `f` chứa nhiều dòng path cũng chỉ chạy MỘT vòng — zsh không tách từ biến không quote, khác bash; dạng an toàn là `... | while IFS= read -r x; do`; (b) **comment chứa chính từ khoá** làm hit giả (`// KHÔNG dùng :host`, câu văn có chữ `MUTATION`) → `grep -v` dòng comment trước khi đếm; (c) **mã màu ANSI** làm `grep "error TS"` trên output có màu trả **0** → bóc `\x1b\[[0-9;]*m` trước khi khớp. (d) **`grep -c` đếm DÒNG, không đếm lần xuất hiện**: file 1 dòng (bundle đã minify, JSON nén) luôn cho 0 hoặc 1 — dùng `grep -o … | wc -l`; và chuỗi non-ASCII trong bundle bị escape `\uXXXX` nên grep tiếng Việt trả 0 dù có thật (2026-08-28). Cùng lớp, 2 biến thể riêng của code Go/GORM (2026-09-07, gây 3 lần báo sai số trong một phiên): (e) **chuỗi GORM xuống dòng** — `\.Order\(`/`\.Joins\(` đòi dấu chấm liền kề nên trượt hết `.\n\t\tOrder(`, phải bỏ dấu chấm (`\bOrder\(`); (f) **tên hàm Go**: `split("(")[0]` cắt ngay ở `(h *Handler)` ⇒ MỌI method ra tên rỗng ⇒ script "sửa theo hàm" âm thầm biến thành "sửa theo file", dùng `^func\s+(?:\([^)]*\)\s*)?(\w+)`. Hai hệ quả kèm theo: GORM **nối** nhiều `.Order()` thành một `ORDER BY` nên phải gộp term rồi mới xét, và phải theo vào hàm builder (`Build*Query`, `buildXxxFilters`) trước khi kết luận "query này chưa có ORDER BY". (g) *(đặc thù Claude Code)* **nhiều lệnh Bash chạy song song dùng chung cwd**: một lệnh `cd` khiến các lệnh cùng lượt dùng đường dẫn tương đối chạy sai thư mục → grep/gate trả **0** hoặc `go build` báo `directory prefix … does not contain main module` (2026-09-14, 4 lần/1 phiên) — lệnh song song dùng đường dẫn TUYỆT ĐỐI + in `pwd`/`ls` đầu lệnh. (h) *(đặc thù Claude Code)* `grep` là shell function trỏ tới ugrep: `-E` kèm `\|` (cú pháp BRE) báo `empty (sub)expression` rồi trả **0 hit** → trong `-E` dùng `|` trần (`-E 'a|b'`). Quy tắc chung: mọi lệnh kết luận bằng "= 0" phải kèm đối chứng chứng minh lệnh CÓ chạy (một mẫu chắc chắn có hit, hoặc `ls` path). (i) Kết luận "0 caller" bằng grep `Tên(` bỏ sót hàm truyền như giá trị (`f(pkg.Tên, …)`) → grep tên trần; kết luận "file phình" phải kiểm file có phải đổi tên không (so `git show HEAD:<tên cũ>`) — 2026-09-28 retro: 2/2 finding "cao" của subagent sai vì 2 lỗi này.
- Prompt cho executor phải TỰ CHỨA context (Dev Notes trích path/pattern/gotcha sẵn) — bắt executor tự tra `skill/` là đổi rẻ lấy sai.
- Fallback limit: executor chạm limit → relaunch CÙNG bậc sau reset (ScheduleWakeup ≤1h/nhịp); KHÔNG leo bậc tự ý. Resume = file story (Status + Checkpoint), executor chạy lại story dở idempotent (không commit, migration idempotent). Chi tiết: memory [[auto-resume-on-limit]].
- Workflow script: qua `opts.model` từng `agent()` call; stage cơ học thêm `effort: 'low'`.
- Phân vân giữa 2 bậc executor → chọn bậc CAO hơn (Correctness over Speed).
- **Phiên Claude Code khác (session/cửa sổ độc lập của user, KHÔNG phải subagent fan-out) có thể đang chạy song song trên cùng `backend/`/`frontend/`** và tự commit bất cứ lúc nào — commit đó quét NGUYÊN working tree (kể cả uncommitted work của phiên mình), không chỉ riêng phần của nó. Trước khi báo "đã dọn xong X file" hay hành động dựa trên `git status`: đối chiếu `git log`/`git show` xem nội dung có đúng ý mình không, đừng suy diễn từ trạng thái sạch/bẩn của working tree. Phát hiện commit lạ → KHÔNG revert/reset, chỉ xác minh nội dung + báo user. Tự mình commit lúc này: build danh sách file đã sửa (từ Checkpoint/nhớ lại), `git add` từng file đó (KHÔNG `git add -A`/`-u`), `git diff --cached --stat` đối chiếu đúng danh sách trước khi commit — tránh cuốn theo uncommitted work của phiên khác vào commit của mình. ⚠ `git add` từng file **KHÔNG đủ**: index có thể đã chứa thứ phiên khác stage sẵn (họ vừa `git mv`), và `git commit` lấy TẤT CẢ những gì đang staged. Nên `git diff --cached --stat` phải chạy ở **lượt riêng**, KHÔNG nối cùng lệnh `git commit` — nối thì output kiểm tra về tới nơi lúc commit đã chạy rồi.

---

## Base — Đọc trước khi sửa bất kỳ code nào

- BE rules đầy đủ: `skill/skills/backend-hard-rules/SKILL.md` | Code examples ❌/✅: `skill/skills/backend-hard-rules/reference/examples.md`
- Arch docs + module map: `skill/skills/navigating-backend-architecture/SKILL.md`
- Project overview + dev tooling: `skill/skills/navigating-backend-architecture/reference/project-overview-and-dev-tooling.md`
- Cập nhật skill/: `skill/HOW_TO_UPDATE.md`

---

## Skills — bảng tra cứu (đọc TRƯỚC khi sửa code)

> Agent có auto-trigger (Claude): các skill dưới đây tự kích hoạt theo `description`, không cần đọc bảng này.
> **Agent KHÔNG có auto-trigger (Codex, Gemini, Cursor...): bảng này là đường vào duy nhất** — tự mở đúng file SKILL.md trước khi sửa.
> Ví dụ ❌/✅, gotcha chi tiết nằm trong `reference/` của từng skill (sâu đúng 1 tầng).

| Khi task chạm... | Đọc TRƯỚC khi sửa |
|---|---|
| **Bất kỳ file `.go` nào** trong `backend/` — kể cả sửa 1 dòng | `skill/skills/backend-hard-rules/SKILL.md` |
| Tạo MỚI resource BE (entity→dao→service→handler→route) | `skill/skills/writing-backend-resources/SKILL.md` |
| Query sai / list rỗng / GORM lạ / filter / log runtime · che secret trong log lỗi HTTP | `skill/skills/debugging-backend-queries/SKILL.md` |
| Viết/sửa test Go, integration test đụng DB | `skill/skills/testing-backend/SKILL.md` |
| Không biết code nằm đâu · tech stack · routing · S3 · dev tooling | `skill/skills/navigating-backend-architecture/SKILL.md` |
| **Gia hạn/nạp SSL · cert hết hạn · sửa vhost nginx · deploy host** | `skill/skills/navigating-backend-architecture/reference/tls-certificates-and-nginx.md` |
| Tạo/sửa migration, đổi schema, thêm cột/FK, backfill | `skill/skills/writing-mysql-migrations/SKILL.md` |
| Khách hàng · lịch hẹn · liệu trình · đơn hàng · thanh toán | `skill/skills/working-on-crm/SKILL.md` |
| Nhân sự · chấm công · lương · hợp đồng · OT | `skill/skills/working-on-hr/SKILL.md` |
| Kế toán · dashboard · export · import | `skill/skills/working-on-ops/SKILL.md` |
| Tổng đài/Omicall · ký số · mẫu in · cấu hình bảng · AI config | `skill/skills/working-on-config/SKILL.md` |
| Auth/JWT/2FA/device limit · workspace · IndexedDB (BE) | `skill/skills/handling-auth-and-workspace/SKILL.md` |
| Danh mục `user_option` / OptionCrm | `skill/skills/managing-crm-options/SKILL.md` |
| **Frontend** (Angular/PrimeNG/Signal Forms/i18n) | `frontend/skill/INDEX.md` — 6 skill |
| Bảo trì chính `skill/` (tối ưu, dedup, chốt mốc, 7 gate) | `skill/skills/maintaining-skill-base/SKILL.md` |

**Gate tự kiểm knowledge base** (chạy được bởi bất kỳ agent hoặc người):
`skill/skills/_tooling/{verify-paths,verify-links,verify-frontmatter,verify-depth,verify-no-dup,verify-symbols,verify-orphan}.sh`

**Gate quét CODE backend** (khác nhóm trên — soi `backend/`, không soi knowledge base):
`skill/skills/_tooling/verify-id-params.sh` — path param `c.Param()` đi thẳng vào query chưa parse
ra số (MySQL ép `'5abc'` → `5`, warning 1292 bị nuốt). Chạy theo BASELINE: chỉ đỏ khi số vi phạm
TĂNG. Rule: `skills/backend-hard-rules/SKILL.md` §"Path param `id` PHẢI parse ra số".

---

## Cửa vào theo TRIỆU CHỨNG — agent chọn giúp, user không phải nhớ

> Bảng skill ở trên trả lời *"đọc gì trước khi sửa"*. Bảng này trả lời *"chạy command nào"*.
> Khoá tra cứu là **lời user nói**, không phải tên vai. Danh sách đầy đủ + mô tả từng command:
> `skill/commands/INDEX.md`.

**Luật gợi ý — áp cho AGENT, không phải user**: khi việc sắp làm chạm **>3 file**, đổi
**schema/migration**, hoặc là **bug chưa rõ nguyên nhân** → nêu ĐÚNG 1 dòng
*"Việc này hợp với `/X` vì Y"* rồi **làm tiếp luôn**, KHÔNG dừng chờ trả lời. Việc nhỏ (1–3
dòng, 1 file) → im lặng làm, KHÔNG gợi ý. Gợi ý mà chặn việc thì lần sau user sẽ bỏ qua nó.

| User nói kiểu... | Cửa |
|---|---|
| "lỗi rồi", "sao không chạy", "hôm qua còn được" — bug chưa rõ nguyên nhân | `/investigate` — Iron Law: KHÔNG sửa khi chưa xác nhận nguyên nhân gốc bằng bằng chứng |
| "làm cho tôi feature X", "thêm màn hình…" — việc mới, nhiều file | `/bmad-plan` → `/bmad-stories` → `/bmad-run` |
| "làm story-07 đi" | `/bmad-dev <id>` → `/bmad-qa <id>` |
| "review giúp", "code này ổn chưa" | `/code-review` — **cửa DUY NHẤT review code**. Nghiệm thu cả mảng việc (truy vết yêu cầu THIẾU + checklist hard-rules) → đọc thêm `skill/skills/backend-hard-rules/reference/review-checklist.md` |
| "story này Done được chưa" | `/bmad-review` — adversarial 3 lens, subagent fresh-context, soi 1 story/1 diff |
| "màn này chậm", "load lâu", "sao nó gọi API nhiều thế" | `/check-component` — trace FE→BE→DB |
| "nhiều dữ liệu là chậm", "liệu có scale không", "chạy được với 1 triệu dòng chứ" | `/check-scale` — bench trên DB clone ≥500k dòng. Iron Law: KHÔNG kết luận hiệu năng từ DB dev (vài trăm dòng che sạch đường đi 6.8s ở quy mô) |
| "nhìn xấu", "canh lệch", "màu sai", "dark mode hỏng" | `/check-ui` — review-only |
| "epic xong rồi, nhìn lại xem" | `/bmad-retro` — cấp EPIC, ra phán quyết nghiệm thu |
| "nhớ cái này đi", "ghi lại bài học" | `/save-skill` |
| "dọn `skill/`", sau một đợt BMAD lớn | `/skill-optimize` |

Không khớp dòng nào → làm thẳng theo Development Protocol, KHÔNG ép vào một command cho có.

---

## BMAD (AI-Driven Agile cho task lớn)

> ⚠ **2 vị trí khác bản chất (root repo, cập nhật 2026-08-30 — thay ghi chú "đã dời" 2026-08-17)**: bộ khung BMAD (persona, executor-contract, prompts-playbook, router INDEX) là **tooling bền, tái dùng** → nằm ở **`skill/bmad/`** (git theo dõi, cùng repo `skill/`). Plan/story thật của từng feature đang chạy/đã xong là **artifact transient** → nằm ở **`bmad/` gốc erp-cluvix** (ngang hàng `skill/`), **KHÔNG git theo dõi** (`/bmad` trong `.gitignore`). Xoá nhầm trong `bmad/` gốc là mất vĩnh viễn — không có git cứu. `frontend/bmad/`, `mobile/bmad/` (plan FE/mobile) hiện vẫn giữ nguyên gộp chung như cũ — chưa tách theo mô hình này.

Xem sub-index: `skill/bmad/INDEX.md` — dùng khi làm feature mới/đổi schema/cross-module cần qua Analyst→PM→Architect→SM→Dev→QA; fix nhỏ 1–3 dòng vẫn đi thẳng theo Development Protocol phía trên, không cần qua BMAD.


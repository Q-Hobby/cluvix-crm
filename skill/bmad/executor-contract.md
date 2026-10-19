# Executor Contract — luật vận hành BẮT BUỘC cho mọi agent chạy BMAD

Áp dụng cho MỌI executor (Dev/QA/review subagent) và cả main agent khi tự làm. Đây là luật **chống đụng nhau**: tài nguyên dùng chung (port, RAM, đĩa, working tree, file chung) mà 2 agent giành nhau thì cả wave hỏng, không phải riêng agent phạm luật.

Đọc file này TRƯỚC khi chạm code. Chi tiết + sự cố gốc từng mục: root `CLAUDE.md` §Hard Rules và §Ràng buộc vận hành — file này là bản hành động, KHÔNG chép lại lịch sử.

---

## 1. Port + server

- KHÔNG chạy binary app: `go run .`, `./tmp/main`, tự start `air`. Port 8081 là của user.
- KHÔNG `go build -o ./tmp/main` — ghi đè binary air đang chạy.
- Cần server thật → `backend/scripts/run_qa.sh` (port 8079, tự kill + xoá binary). Chạy nền, curl `http://localhost:8079/...`, xong kill script.
- File/thư mục tạm chỉ đặt trong `backend/tmp/` — tạo thư mục mới ở gốc `backend/` sẽ kích air rebuild và giết dev server.
- Không cần server cho phần lớn việc: `go build` + `go test ./path/...` là đủ (test dựng server dùng `httptest.NewServer`, port ephemeral).

## 2. Build — scope theo path, không `./...` mỗi lần

- Mặc định: `go build ./internal/modules/webapp/<module>/...` theo path đã đổi. Full `go build ./...` **1 lần cuối wave**, không mỗi story.
- Nhiều executor song song → `export GOFLAGS=-p=4` trước khi build.
- Chỉ cần biết "có lỗi compile không" → đọc `backend/tmp/run.log` (air đã build sẵn). Air KHÔNG phủ `cmd/migrate`, `cmd/recalc-reconcile` — đụng các cmd đó phải build riêng.
- `go build` không bắt lỗi format → `gofmt -l <file đã đổi>` trước khi kết luận sạch.
- FE: `npx tsc --noEmit` **VÀ** `npx ngc -p tsconfig.app.json --noEmit` — `tsc` KHÔNG type-check binding trong `.html` (strictTemplates); epic fe-remove-company-picker 2026-09-03 qua 3 pass Dev+QA "tsc 0 lỗi" mà dev server user đỏ TS2322 ở `[companyId]`. `ngc` không ghi `public/`; output có màu → bóc ANSI rồi đếm `error TS` trước/sau (không tin exit code, có lỗi nền pre-existing — xem `frontend/skill/skills/using-primeng-components/reference/fe-patterns-gotchas-build.md`). KHÔNG `ng build` (ghi đè `public/` git-tracked của repo cha).
- FE `ngc` PHẢI kèm `-p tsconfig.app.json`: chạy trần `npx ngc --noEmit` dùng `tsconfig.json` (gồm spec/file ngoài app) → ~17 lỗi giả (`image-comparison`, `money-shorthand`, `notification-item` (component đã bỏ)) trong khi app sạch — Dev 2026-09-03 báo nhầm "pre-existing". FE vitest PHẢI `npx vitest run -c vitest-base.config.ts <spec>`: thiếu `-c` bỏ `setupFiles` → cả file đỏ `PlatformLocation needs JIT compiler` / `no tests`, trông y hệt spec hỏng. **NHƯNG spec render component + `ComponentRef.setInput()` trên signal input thì `vitest-base` đỏ GIẢ** (config không có plugin biên dịch Angular ⇒ `setInput` im lặng không set, NG0303 async, component render rỗng) — loại spec đó chạy `npx ng test --include='<spec>' --watch=false`; vitest-base chỉ cho hàm thuần/service (2026-09-05, tái hiện trên 4 spec `omnichat/thread/*`). ⚠ Trừ component nặng PrimeNG (`p-select`/`p-fileupload`): `ng test` dựng `TestBed.createComponent` treo CPU 100% không dứt — tách `value()`/`isEmpty()`/schema thành hàm thuần export, test qua `vitest -c vitest-base.config.ts`, component chỉ còn wrapper mỏng.

## 3. Đĩa

- Trước wave lớn: `df -h /System/Volumes/Data` phải còn ≥15GB.
- Giữa các wave, khi cần: `rm -rf ~/Library/Caches/go-build`.
- **Thứ ăn đĩa thường KHÔNG phải cache mà là temp dir của build BỊ GIẾT** (đo 2026-08-31): build bị kill không tự dọn `$TMPDIR/go-build*` — cache lúc đó 545MB còn temp dir chết **15GB**. Dọn: xoá thư mục `go-build*` trong `$TMPDIR` không được ghi trong vài phút gần đây (chừa build đang chạy). Hồi 1.7GB → 19GB.
- **Xoá cache rồi fan-out NGAY là kết hợp tệ nhất**: mọi agent buộc phải build lại từ đầu cùng lúc. Đo sàn 15GB **sau khi cache đã ấm lại**, không phải ngay sau khi xoá.
- Chuỗi hệ quả khó nối: đĩa đầy → air chết → `backend/tmp/` biến mất → mọi lệnh ghi log vào đó fail `no such file or directory`, trông như lỗi lệnh chứ không như lỗi hạ tầng.
- Agent chết vì ENOSPC → đọc report/Checkpoint trước, resume; KHÔNG chạy lại từ đầu.

## 4. Git — vùng cấm

- CẤM `git checkout` / `git restore` / `git stash` trên file bất kỳ. Working tree chứa việc chưa commit của agent và phiên khác — mất là mất vĩnh viễn, `git fsck` vô ích.
- Cần so với bản HEAD → `git show HEAD:<file>` ra scratchpad. KHÔNG đụng working tree.
- Lỡ hỏng file chung → Edit ngược lại đúng phần mình gây ra, KHÔNG revert cả file.
- File `go:embed` (catalog/seed JSON) lỡ bị ghi đè mà chưa commit: bản cũ còn trong package archive ở `~/Library/Caches/go-build` — tìm bằng `grep -rlaF '<chuỗi chắc có>'`, chọn entry có mtime trước sự cố, `json.JSONDecoder().raw_decode` để trích. Đừng `rm -rf` go-build cache trước khi cứu xong.
- BMAD **không commit**. Nếu user yêu cầu commit: `git add <từng file>` (KHÔNG `-A`/`-u`), `git diff --cached --stat` đối chiếu đúng danh sách trước khi commit.
- ⛔ **`git add <từng file>` KHÔNG ĐỦ khi một FILE chứa cả việc của mình lẫn của phiên khác** (đo 2026-08-25, epic omnichat-message-reaction): `manager.go` có 413 dòng thêm mà của mình chỉ 11; `container/omnichat.ts` 388/130; `route_perm_map.json` 234/18. `git add` cả file = cuốn việc dở của họ vào commit của mình, và **land nửa vời** (khai báo field mà không có code dùng nó).
  - **Đối chiếu số dòng TRƯỚC khi commit**: `git diff --cached --numstat` so với số dòng ghi trong `## Checkpoint` của story. Lệch = có kẻ lạ.
  - Tách được bằng hunk: `git diff -U3 -- <file>` → giữ hunk có dấu vết của mình → `git apply --cached`. ⚠ Patch cắt từ
    `git diff HEAD` **FAIL `patch does not apply`** khi phía trên file có hunk phiên khác làm lệch offset — và chuỗi lệnh vẫn
    `commit-tree` phần còn lại ⇒ commit THIẾU file mà log trông xanh (3 lần/phiên 2026-09-04: model.ts, i18n JSON, SW).
    Luôn `git diff --cached --stat` và kiểm CÓ file đó TRƯỚC `write-tree`; lệch offset → dùng cách blob dưới, đừng vá patch.
  - Hunk của mình DÍNH chung với của họ (cách nhau <3 dòng context): dựng nội dung stage chính xác từ HEAD rồi `git hash-object -w` + `git update-index --add --cacheinfo` — cách này KHÔNG đụng working tree.
  - **Phiên khác vẫn ghi trong lúc mình làm**: đo lần đầu rồi `git add` lần sau là ăn thêm việc mới của họ (`omni_channel.go` phình 16→36 dòng giữa 2 lệnh). Luôn đối chiếu lại NGAY TRƯỚC khi commit.
  - **Index đã chứa việc phiên khác stage sẵn** → `git commit -m … -- <path>…` chỉ commit các path đó, **BỎ QUA index**; những gì họ stage vẫn nguyên. ⚠ `-m` phải đứng TRƯỚC `--`: sau `--` mọi thứ bị coi là path (kể cả `-m` và cả nội dung message).
  - **File lẫn hunk của mình với của họ** → dựng **index tạm**, KHÔNG đụng index thật:
    `GIT_INDEX_FILE=<tmp> git read-tree HEAD` → `git apply --cached <patch chỉ chứa hunk của mình>` → `git write-tree` → `git commit-tree -p HEAD -F <msg>` → `git update-ref HEAD <sha>`.
  - ⛔ **Xong phải `git reset HEAD -- <các path vừa commit>`** — index THẬT vẫn giữ blob TRƯỚC commit, và `git commit` lần sau (của mình hay của phiên khác) sẽ lấy blob cũ đó ⇒ **revert âm thầm** thứ vừa land. Working tree sạch không phát hiện được.
  - Nhận diện: `git status` hiện `MM`/`M ` nhưng `git diff --stat HEAD -- <file>` **rỗng** = index cũ, KHÔNG phải việc của phiên khác. Quét cả repo trước khi kết luận S6.
  - Xong phải **build lại từ trạng thái ĐÃ COMMIT**, không phải working tree: `git worktree add --detach <scratchpad>/verify HEAD` rồi build trong đó. Working tree xanh KHÔNG chứng minh commit xanh.
  - Hook `pre-commit` đỏ vì file của phiên khác (vd `permissions.gen.json` stale) → `--no-verify` + ghi LÝ DO vào commit message; KHÔNG chạy codegen ghi đè file đang dở của họ.
- `frontend/` là git repo riêng — `git status` từ root KHÔNG thấy; phải `cd frontend`.

## 5. File dùng chung + song song

- Edit thuần **additive theo anchor**; đọc bản mới nhất ngay trước khi sửa.
- **Snapshot phạm vi (danh sách file bẩn của phiên khác) phải BẤT BIẾN**: đặt tên có timestamp trong scratchpad, `chmod a-w`, subagent KHÔNG được ghi đè/"chụp lại". Retro 2026-09-03: `fe-dirty.txt` bị agent con ghi đè giữa epic nên chứa cả file story-01..04 → phép trừ `git status − snapshot` thiếu ~25 file, phải dựng lại phạm vi bằng chữ ký hunk.
- KHÔNG 2 agent cùng sửa 1 file, cũng KHÔNG cùng 1 MODULE.
- KHÔNG 2 agent cùng cộng dồn vào một con số tổng / bảng đối chiếu — anchor edit không chống được xung đột số học. Executor chỉ append mục + Δ; Planner cộng tổng.
- JSON registry lớn (`route_perm_map.json`...): chỉ Edit anchor, KHÔNG parse-rewrite bằng script.

## 6. DB

- Test/migrate CHỈ chạy trên DB dùng-một-lần (`*_test` / `*_tmp`), tạo đầu → drop cuối. KHÔNG trỏ `TEST_MYSQL_DSN`/`-db` vào `DB_NAME`.
- `run_qa.sh` GHI THẬT vào DB trong `.env` — nó in tên DB lúc khởi động, đọc dòng đó trước khi thao tác ghi.

## 7. Trước khi báo Review / Done

- `grep -rnE 'MUTATION|if false &&' --include='*.go' internal/ pkg/` phải = 0 (cờ mutation của QA adversarial lọt vào code sản phẩm thì build vẫn xanh mà bản vá bị vô hiệu).
- ⚠ **Nhưng `= 0` KHÔNG chứng minh commit sạch**: mutation **"sạch"** (xoá một vế `WHERE`, đảo `<` thành `>`) **không để lại dấu vết văn bản** — lưới cờ chỉ bắt mutation **có nhãn**. Khi phiên khác commit song song, so `git show HEAD:<file>` với `shasum` bản đúng, đừng tin lệnh quét cờ. (2026-09-01: `a75b4be7` chụp `omnichat/service/availability.go` đúng lúc đang mutate ⇒ commit thiếu cả hai vế `company_id`, quét cờ trên HEAD vẫn ra 0.)
- **Phải QUOTE glob** `'*.go'` — viết trần `--include=*.go` thì zsh nuốt glob, trả 0 hit giả.
- Pattern có `+` hoặc khoảng trắng → dùng `-E` và quote: `-E '^[[:space:]]*Status +\*int'`.

**Bẫy lệnh im lặng trả rỗng** (bổ sung 2026-08-31, epic teamchat-customer-ref — 6 lần kết luận sai, 2 lần do chính Planner). Danh sách đầy đủ các biến thể khác: `skill/INDEX.md` §Quét mutation.
- `git status <pathspec>` sai cwd → **rỗng im lặng, KHÔNG báo lỗi** (pathspec không khớp file nào vẫn exit 0). Kết luận "working tree sạch" phải kèm đối chứng: một file chắc chắn bẩn phải hiện ra trong cùng lệnh.
- `vitest <đường dẫn spec sai>` → **im lặng chạy các file còn lại**, không báo file thiếu. Luôn đọc dòng `Test Files N passed` và kiểm N khớp số spec đã truyền.
- `grep -c` trả **exit 1 khi đếm được 0** ⇒ `grep -c ... && echo "..."` không in gì, dễ tưởng lệnh không chạy. Dùng `; echo` hoặc `|| true`.

**Mutation-verify — bằng chứng phải trỏ vào FILE SẢN PHẨM** (2026-08-31, story-32 epic teamchat-customer-ref):
- Mutation-verify **phải sửa file sản phẩm**, KHÔNG phải bản chép trong spec. Báo cáo ghi rõ **đường dẫn file đã sửa tạm** + output ĐỎ + output XANH sau khôi phục + `shasum`/grep xác nhận đã khôi phục nguyên văn.
- Test khoá **tự chép lại predicate** rồi kiểm bản chép của chính nó = **lưới GIẢ**, luôn xanh kể cả khi sản phẩm hỏng. Dấu hiệu nhận biết: spec KHÔNG `import` file mà nó tuyên bố đang khoá.
- Không execute được component → dùng khuôn **quét mã nguồn** (`readFileSync` file thật + bóc comment trước khi khớp + test đối chứng "file tồn tại trên đĩa"). ⚠ Cấy cờ mutation dạng **comment** thì lưới này VẪN XANH (có `stripComments`) — phải cấy **mã thật** mới kết luận được lưới còn sống.
- **"0 đỏ" có thể là 0 GIẢ do mutation làm VỠ BIÊN DỊCH** (2026-08-31, epic omnichat-meta-event-coverage): gỡ một lời gọi khiến các biến truyền vào thành unused ⇒ test binary không dựng được ⇒ **0 dòng `--- FAIL`**, trông y hệt "lưới không bắt được". Mutate kiểu KHÔNG phá build (`_ = f(...)` để biến vẫn được dùng), và trước khi đọc số phải kiểm output không chứa `build failed` / `declared and not used`. Cùng ca đó mutate lại đúng cách cho ra 2 đỏ thật.

## 8. Lưới cưỡng chế bằng máy (2026-08-17)

`.claude/settings.json` → PreToolUse/Bash → `.claude/hooks/guard-bash.sh` chặn THẲNG (deny, không hỏi): chạy server chính (`go run .` kể cả kèm cờ `-race`/`-tags`, `go run main.go`, `./tmp/main`, `air`), build đè binary của air, `git stash` (trừ `list`/`show`), `git restore`, `git checkout <đường-dẫn>`, `git add -A/-u/.` (kể cả `git add -- .`). Lệnh bọc qua `bash -c`/`sh -c`/`eval`/`xargs` cũng bị soi — không lách được bằng một lớp bọc.

`git checkout <X>` phân biệt bằng **X có tồn tại trên đĩa hay không** (xét cả `cd` đứng trước), không phải theo đuôi file: X là path có thật → phục hồi file → chặn; X không tồn tại → tên nhánh → cho qua.

Vẫn cho phép: `go run ./cmd/...`, `git checkout <branch>` và `-b`, `git add <file>`, `backend/scripts/run_qa.sh`, mọi lệnh đọc.

- Bị chặn = làm sai cách, KHÔNG phải hook hỏng. Đọc lý do trong thông báo rồi đổi cách; KHÔNG tìm đường lách (bọc `bash -c`, tách chuỗi qua biến, đổi cwd).
- Hook đặt ở `settings.json`, KHÔNG phải `settings.local.json` — harness ghi đè lại file local mỗi khi thêm permission, xoá mất hook.

## 9. Khi bí

- Chạm limit → relaunch CÙNG bậc model sau reset, resume từ `## Checkpoint` của story. KHÔNG tự leo bậc.
- Phát hiện commit lạ trong `git log` (phiên khác của user) → KHÔNG revert/reset, chỉ xác minh nội dung + báo user.
- Story/architecture sai hoặc thiếu → dừng, báo user. KHÔNG tự sửa kiến trúc ngầm rồi code tiếp.

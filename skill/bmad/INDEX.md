# BMAD — Breakthrough Method for Agile AI-Driven Development (Cluvix)

BMAD là cách **hiện thực hoá** Development Protocol 4 bước trong `CLAUDE.md` root (Understand → Design → Implement → Validate) cho **task lớn** (feature mới, đổi schema, cross-module). Nó KHÔNG thay thế CLAUDE.md, KHÔNG áp dụng cho fix nhỏ 1–3 dòng — fix nhỏ đi thẳng theo CLAUDE.md, không cần qua BMAD.

Quan hệ với `skill/`: `skill/` là wiki tĩnh (rule/pattern đã đúc kết, đã dedup) — mọi persona BMAD **đọc** nó làm context nguồn trước khi viết artifact. **KHÔNG persona nào được sửa `skill/`.** Cập nhật `skill/` vẫn đi qua `/save-skill` sau khi việc xong, như bình thường.

**Tách vị trí (2026-08-30):** bộ khung BMAD (persona, executor-contract, prompts-playbook, file này) nằm ở `skill/bmad/` — kiến thức/tooling bền, tái dùng qua nhiều epic. Plan/story thật của TỪNG feature nằm ở **`bmad/<feature-slug>/`** ở gốc repo (ngang hàng `skill/`, `backend/`, `frontend/`) — artifact **động/transient**, KHÔNG git theo dõi (`/bmad` trong `.gitignore` gốc). Đây là 2 thứ khác bản chất: tooling ổn định thì ở `skill/`, artifact gắn với 1 task đang chạy thì ở `bmad/` gốc.

---

## Vòng đời (6 giai đoạn, 6 persona)

```
Ý tưởng thô
   │  /bmad-brief
   ▼
Analyst  → brief.md         (vấn đề, mục tiêu, phạm vi, câu hỏi mở)
   │  /bmad-prd
   ▼
PM       → prd.md           (yêu cầu chức năng/phi chức năng, user story cấp cao, AC)
   │  /bmad-architect
   ▼
Architect→ architecture.md  (module ảnh hưởng, DB impact, API contract, rủi ro)
   │  /bmad-stories
   ▼
SM       → epics.md + stories/story-NN-*.md   (cắt nhỏ, mỗi story tự chứa đủ context)
   │  /bmad-dev <story-id>
   ▼
Dev      → implement + cập nhật Status + Checkpoint (Draft → In Progress → Review; Blocked nếu cần user)
   │  /bmad-qa <story-id>   (+ /bmad-review để săn sâu 3 lens)
   ▼
QA       → adversarial review (săn lỗi, không rubber-stamp), Done hoặc trả về Dev kèm findings
   │  /bmad-retro <slug>   ← MỘT LẦN cho CẢ EPIC, sau khi mọi story Done

   ⚡ /bmad-run <slug> — chạy TUẦN TỰ trọn vòng Dev→QA→fix→QA cho mọi story còn dở rồi tự gọi
      /bmad-retro. Dừng ở 8 HARD STOP cần người (ac_gap · gap kiến trúc · cần user thao tác tay ·
      Review-loop>3 · viết lại phần đã verify · file phiên khác đang sửa · việc khó đảo ngược ·
      story tự ghi điều kiện phải hỏi). KHÔNG commit, KHÔNG song song.
   ▼
Retro    → retrospective.md (phán quyết nghiệm thu) — săn khuyết tật là thuộc tính của CẢ epic:
           trùng lặp giữa story, file phình dần, lệch pattern, spec↔as-built, Hard Rules toàn diff
```

`/bmad-plan <ý tưởng>` chạy gộp Analyst → PM → Architect (giai đoạn planning) trong 1 lệnh khi không cần dừng lại review từng bước.

**⛔ Luật vận hành BẮT BUỘC cho mọi executor:** `skill/bmad/executor-contract.md` — chống đụng nhau về tài nguyên dùng chung (port 8081/`run_qa.sh` 8079 · build scope theo path · đĩa · CẤM `git checkout/stash` · file chung · cờ mutation). `/bmad-dev`, `/bmad-qa`, `/bmad-run` đều BẮT BUỘC đọc file này trước khi chạm code.

**Prompts playbook cho đợt BMAD lớn tự hành:** `skill/bmad/prompts-playbook.md` — 10 prompt cấp lệnh theo trình tự + 7 quy tắc nền (đúc kết 2026-07-21).

**Model khi fan-out Dev/QA subagent:** theo Planner/Executor policy trong root `CLAUDE.md` §Subagent Model Policy — Fable chỉ planning/review (Analyst→SM + QA synthesize); dev story = opus (khó/auth/security/cross-module) / sonnet (CRUD/UI/form/report) / haiku (edit nhỏ/boilerplate).

**3 cơ chế mượn từ BMAD-METHOD (official, 50k⭐) — 2026-07-18 + 2026-07-22; lens 2+3 của `/bmad-review` đã đối chiếu + đồng bộ hardening theo upstream v6.10.0 (2026-07-22, cả 3 repo root/FE/mobile):**
- **Adversarial QA + `/bmad-review`**: QA giả định CÓ lỗi phải đi tìm, đánh giá artifact KHÔNG tin lời tự khai (information asymmetry), 0-finding phải giải trình. `/bmad-review` chạy 3 lens trực giao qua subagent fresh-context: correctness/rule · edge-case path-enumeration · verification-gap. Xem `personas/qa.md` + `commands/bmad-review.md`.
- **Status state machine + Checkpoint resume**: mỗi story có `## Checkpoint` Dev cập nhật LIÊN TỤC (AC đã xong / file đã đụng / việc kế tiếp). Agent chết giữa chừng (session limit) → relaunch RESUME từ Checkpoint thay vì chạy lại từ đầu (giải bài toán memory [[auto-resume-on-limit]]). Resume-by-status: Draft→plan, In Progress→resume, Review→chờ QA, Blocked→báo user.
- **Review triage + loopback log** (từ `bmad-dev-auto` upstream, 2026-07-22): QA route findings vào 5 loại `ac_gap / story_defect / patch / defer / reject` (cascade: ac_gap → Blocked hỏi user; story_defect → sửa story + Dev làm lại, guard `Review-loop` >3 → Blocked). Story có 2 section APPEND-ONLY: `## Review Triage Log` (mỗi pass) + `## Story Change Log` (mỗi lần sửa story, kèm **KEEP** — cái đúng phải giữ khi làm lại); header thêm `Baseline` (Dev ghi rev khi bắt đầu — neo diff cho QA) + `Followup-review` (khi Done). Nợ có sẵn lộ khi review → `bmad/<slug>/deferred-work.md`. Chi tiết: `personas/qa.md` §Triage + `personas/dev.md` + template `_example/stories/`.

## Dọn dẹp: Done → save-skill → ARCHIVE vào `done/` (user chốt 2026-07-30, THAY luật xoá cũ)

`bmad/<slug>/` (feature folder, gốc repo) là artifact TRANSIENT ở vị trí active — feature làm XONG phải **dọn khỏi khu active**, không để phình folder. Folder này KHÔNG git theo dõi nên KHÔNG `git mv` — **di chuyển bằng `mv` thường vào `bmad/done/<slug>/`** để giữ dấu vết tra cứu (ai đã làm gì, quyết định gì, khi nào) — xoá mất lịch sử quyết định là cái giá quá đắt so với việc folder active hơi dài. Luật cũ (`git rm -r`, user chốt 2026-07-21, còn từ thời `bmad/` nằm trong `skill/` và được git theo dõi) đã lộ rủi ro thật: nhiều folder untracked (`??`) bị xoá trước khi kiểm `git status` → mất vĩnh viễn, không `git fsck` được.

**LUẬT (user chốt 2026-07-21, bổ sung 2026-07-22): feature folder CHỈ chứa plan** — KHÔNG note kiến thức chuẩn mực vào `bmad/<slug>/`. NGAY TRONG lúc chạy: hợp đồng/spec mà code tham chiếu (IG, API contract) → đặt cạnh code (`backend/internal/modules/<x>/`); rule/pattern/gotcha → `skill/` đúng module theo HOW_TO_UPDATE; **document tham khảo dài (spec pháp lý, assessment, khảo sát, checklist đối chiếu, hướng dẫn nghiệp vụ) → `documents/<topic>/` repo chính — nơi viết document DUY NHẤT, KHÔNG tạo `bmad/<slug>/docs/`** (tiền lệ: `documents/fhir-compliance/`). Folder chuyển sang `done/` lúc nào cũng không mất chuẩn mực (đã đúc vào `skill/` trước khi chuyển). Quy tắc dọn:

**Tracking (bắt buộc):** mỗi feature ghi 1 dòng trạng thái tổng ở ĐẦU `epics.md` (hoặc brief nếu chưa có epics):
`> Trạng thái: Planning | In-progress | Blocked(<lý do>) | DONE-removable`
Mọi story `Done` (hoặc `Deferred`/`Cancelled`/`Superseded` cố ý) + build/verify/commit + **0 flag mở** → đổi thành `DONE-removable` = tín hiệu chuyển vào `done/`. **"0 flag mở" là điều kiện thật, không phải hình thức**: dòng "CHỜ user smoke UI"/"CHỜ commit"/"CHỜ apply migration" ở đầu `epics.md` VẪN tính là flag mở cho tới khi xác nhận lại — code đã có mặt trong `git log` không tự động đóng flag "user đã smoke test thật chưa".

**Bước dọn (khi DONE-removable):**
0. `deferred-work.md` còn entry chưa xử → chuyển thành story mới / flag mở / ghi memory TRƯỚC — KHÔNG chuyển đi mất nợ.
1. **`/bmad-retro <slug>`** — BẮT BUỘC trước khi chuyển. `/bmad-qa` chỉ soi 1 story; khuyết tật là
   thuộc tính của CẢ epic thì không story lẻ nào thấy. Tiền lệ 2026-08-13: epic
   `mysql97-backend-alignment` đã qua **2 pass QA per-story** vẫn còn lỗ hổng cổng an toàn cho qua
   `SELECT ... INTO OUTFILE` (ghi bệnh án ra đĩa) — chỉ retro bắt được, cùng 3 finding khác.
   Phán quyết retro (`Accepted` / `Accepted-with-open-items` / `Rejected`) là **đầu vào** cho quyết
   định chuyển: `Rejected` → KHÔNG chuyển.
2. `/save-skill` — đúc kết bài học non-obvious CHƯA có vào `skill/` (đa số đã lưu trong session build → thường 0-2 mục mới).
3. **Xác nhận code đã commit** (không chỉ story Status=Done): `git log --oneline -- <path liên quan>` phải có commit thật ở cả repo BE lẫn FE (nếu feature đụng cả 2). Story Done nhưng code còn `??`/uncommitted → CHƯA đủ điều kiện chuyển, dù epics.md ghi DONE-removable (bài học 2026-07-30: `mobile-workspace-profile` 18/18 story Done nhưng 0 evidence commit trong `mobile/` — không chuyển).
4. `mv bmad/<slug> bmad/done/<slug>` — giữ nguyên nội dung, chỉ đổi vị trí (`bmad/` không git-tracked nên là `mv` thường, không phải `git mv`).
5. Cập nhật dòng trạng thái đầu `epics.md` trong folder đã chuyển: `> Trạng thái: DONE — archived <ngày>`.

**GIỮ LẠI Ở KHU ACTIVE (KHÔNG chuyển vào `done/`):**
- Feature `In-progress` / `Blocked` (chờ cred/traffic/user-decision) — kể cả khi phần lớn story đã Done nhưng còn ≥1 flag/quyết định nghiệp vụ chưa chốt (vd `hr-ot-request`: quyết định bật ON cho khách hiện hữu khi deploy).
- **Có `design/` chưa-implement-hết hoặc flag mở** → CHƯA done thật (bài học 2026-07-18: suýt xóa omni-core khi nav-rail v2 còn defer). **Review kỹ** design-vs-impl + mọi flag TRƯỚC khi chuyển.
- Reference docs (vd `omnichannel/` review-chatwoot/review-zcrm) — không phải feature transient, KHÔNG bao giờ chuyển vào `done/` (không có "xong" — là kho tham khảo sống).
- Roadmap/plan dẫn hướng epic con khác vẫn đang chạy (vd `mobile-operations-roadmap`) — chuyển sớm làm mất context cho epic con tham chiếu ngược.
- `done/` (chính nó).

## `done/` — kho lưu trữ epic đã hoàn tất, KHÔNG phải backlog

`bmad/done/<slug>/` = đúng nguyên nội dung folder active lúc chuyển (epics.md, stories/, brief/prd/architecture nếu còn) — dùng để **tra cứu** ("epic X làm gì, quyết định gì, ai QA, khi nào") khi có câu hỏi về việc cũ, KHÔNG phải nơi tiếp tục làm việc. Cần làm thêm cho feature đã archive (bug mới phát sinh, mở rộng) → tạo feature-slug MỚI (có thể tham chiếu ngược `done/<slug-cũ>/` làm context), không "hồi sinh" folder trong `done/`.

---

## Khi nào dùng persona nào

| Tình huống | Persona | Lệnh |
|---|---|---|
| Có ý tưởng/yêu cầu thô, chưa rõ vấn đề | Analyst | `/bmad-brief` |
| Đã có brief, cần đặc tả yêu cầu | PM | `/bmad-prd` |
| Đã có PRD, cần thiết kế kỹ thuật | Architect | `/bmad-architect` |
| Đã có architecture, cần cắt việc để làm | SM | `/bmad-stories` |
| Có story cụ thể cần code | Dev | `/bmad-dev <story-id>` |
| Story đã implement, cần review | QA | `/bmad-qa <story-id>` |
| Cần săn lỗi sâu (diff nhạy cảm: tiền/tenant/transaction/migration) | — | `/bmad-review <story-id\|changes>` |

## Feature-slug

Mỗi feature/task đang chạy qua BMAD có 1 folder `bmad/<feature-slug>/` (gốc repo). Xác định feature-slug theo thứ tự ưu tiên (mỗi command tự áp dụng, xem chi tiết trong từng file `.claude/commands/bmad-*.md`):
1. Tham số truyền thẳng vào command (vd `/bmad-brief checkout-discount`).
2. Slug suy ra từ tên nhánh git hiện tại (bỏ prefix `feature/`, `fix/`...).
3. Nếu đang có đúng 1 folder `bmad/<slug>/` dở dang (có brief.md nhưng thiếu artifact sau) → dùng slug đó tiếp.
4. Không xác định được / nhiều slug dở dang cùng lúc → hỏi user.

`skill/bmad/_example/` là folder mẫu minh hoạ cấu trúc — KHÔNG phải feature thật, không dùng làm slug.

## File khung trong `skill/bmad/` (tooling — kiến thức bền, tái dùng nhiều epic)

- `INDEX.md` — file này, router + vòng đời + luật dọn dẹp
- `executor-contract.md` — luật vận hành bắt buộc (đọc trước khi chạm code)
- `prompts-playbook.md` — 10 prompt cấp lệnh + 8 quy tắc nền
- `_example/` — folder mẫu cấu trúc feature
- `personas/` — 6 persona chuẩn + persona bổ sung (xem index dưới)

## Ở gốc `bmad/` (transient — plan/story thật, KHÔNG git theo dõi)

- `<feature-slug>/` — plan/story đang chạy của từng epic
- `done/` — kho epic đã hoàn tất (archive, không phải backlog)

## Persona index

- `personas/analyst.md`
- `personas/pm.md`
- `personas/architect.md`
- `personas/sm.md`
- `personas/dev.md`
- `personas/qa.md`

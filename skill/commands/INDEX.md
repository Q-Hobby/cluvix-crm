# Commands (Slash Commands)

Prompt definition cho các slash command của project. Symlink vào `.claude/commands/`.

> **Chọn cửa theo TRIỆU CHỨNG** (user nói gì → chạy gì): bảng trong `CLAUDE.md`, mục
> "Cửa vào theo TRIỆU CHỨNG". Bảng dưới đây là tra cứu theo TÊN command.
>
> ⚠ `/post-review` đã **gộp vào built-in `/code-review`** (2026-09-02) để bớt cửa review chồng
> lấn. Phần built-in không làm — truy vết yêu cầu THIẾU + checklist hard-rules Cluvix — giữ ở
> `skill/skills/backend-hard-rules/reference/review-checklist.md`, đọc thêm khi nghiệm thu cả
> mảng việc.

| File | Command | Mục đích |
|------|---------|----------|
| `check-component.md` | `/check-component` | Review kiến trúc + performance một component — trace FE → BE → DB |
| `check-scale.md` | `/check-scale` | Chứng minh một endpoint/query có scale hay không bằng SỐ ĐO — bench trên DB clone ≥500k dòng, đo 5 ca (gồm ca "từ khoá không khớp"), đối chiếu kết quả cũ/mới trước khi ship. Iron Law: KHÔNG kết luận hiệu năng từ DB dev |
| `skill-optimize.md` | `/skill-optimize` | Tối ưu knowledge base `skill/` — chạy lại được, chỉ soi phần MỚI kể từ mốc delta lần trước |
| `save-skill.md` | `/save-skill` | Trích xuất knowledge từ session → ghi vào `skill/` đúng chỗ, đúng format |
| `investigate.md` | `/investigate` | Root-cause debugging cho bug/incident — bổ trợ BMAD (BMAD = feature có kế hoạch, đây = bug phát sinh). Iron Law: không sửa khi chưa xác nhận nguyên nhân gốc bằng bằng chứng |
| `check-ui.md` | `/check-ui` | Review UI/visual polish một component/page — đối chiếu `skills/navigating-backend-architecture/reference/project-overview-and-dev-tooling.md` + dark-safe B.18 + so với module mẫu cùng loại. Review-only |
| `bmad-brief.md` | `/bmad-brief` | Persona Analyst (BMAD) — biến ý tưởng thô thành `brief.md` |
| `bmad-prd.md` | `/bmad-prd` | Persona PM (BMAD) — từ `brief.md` viết `prd.md` |
| `bmad-architect.md` | `/bmad-architect` | Persona Architect (BMAD) — từ `prd.md` viết `architecture.md` |
| `bmad-plan.md` | `/bmad-plan` | Gộp Analyst → PM → Architect (planning BMAD) trong 1 lệnh |
| `bmad-stories.md` | `/bmad-stories` | Persona SM (BMAD) — cắt `prd.md` + `architecture.md` thành `epics.md` + story files |
| `bmad-dev.md` | `/bmad-dev` | Persona Dev (BMAD) — implement 1 story cụ thể (`$ARGUMENTS` = story-id) |
| `bmad-qa.md` | `/bmad-qa` | Persona QA (BMAD) — review story đã implement, Status = Review (`$ARGUMENTS` = story-id) |
| `bmad-review.md` | `/bmad-review` | Review sâu adversarial 3 lens qua subagent fresh-context — soi **1 story / 1 diff** trước khi Done |
| `bmad-run.md` | `/bmad-run` | Chạy **tuần tự** Dev→QA→fix→QA cho mọi story còn dở của 1 epic (`$ARGUMENTS` = feature-slug), rồi `/bmad-retro`. Dừng ở 8 HARD STOP cần người. Không commit, không chạm DB thật, không song song |
| `bmad-retro.md` | `/bmad-retro` | Retrospective cấp **EPIC** dựa trên bằng chứng (`$ARGUMENTS` = feature-slug) — săn khuyết tật mà story lẻ không thấy, ra phán quyết nghiệm thu |

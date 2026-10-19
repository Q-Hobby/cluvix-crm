# ⚠ BMAD: lịch sử 2 lần dời (2026-08-17 → 2026-08-30)

## Trạng thái hiện tại (2026-08-30)

**Tách theo bản chất, KHÔNG gộp chung 1 chỗ nữa:**

```
skill/bmad/personas/            ← tooling bền (persona, đọc-trước-khi-viết, output template)
skill/bmad/executor-contract.md ← tooling bền (luật vận hành executor)
skill/bmad/prompts-playbook.md  ← tooling bền (prompt playbook)
skill/bmad/INDEX.md             ← tooling bền (router, vòng đời, luật dọn dẹp)
skill/bmad/_example/            ← tooling bền (folder mẫu minh hoạ)

bmad/<slug>/                    ← artifact transient (plan/story đang chạy) — gốc repo, KHÔNG git theo dõi
bmad/done/                      ← artifact transient (plan/story đã xong) — gốc repo, KHÔNG git theo dõi
```

`frontend/bmad/`, `mobile/bmad/` **chưa** tách theo mô hình này — vẫn gộp chung tooling+plan ở gốc repo con như từ 2026-08-17, chờ áp dụng sau khi có nhu cầu thật.

## Lịch sử

**Lần 1 (2026-08-17):** dời TOÀN BỘ `skill/bmad/` (gồm cả tooling lẫn plan/story) ra `bmad/` gốc, vì trước đó 24.6k dòng knowledge vs 59.4k dòng plan/story trong cùng cây `skill/` khiến `grep` lội qua nhiều nhiễu.

**Lần 2 (2026-08-30, user chốt):** nhận ra gộp chung sang phía ngược lại cũng sai — tooling (persona/executor-contract/prompts-playbook) là kiến thức bền, tái dùng qua nhiều epic, đáng ra vẫn nên ở `skill/` (git theo dõi, có lịch sử, đi qua review như mọi thay đổi `skill/` khác); chỉ phần THẬT SỰ transient (plan/story gắn 1 feature) mới cần nằm ngoài `skill/`. Tách lại theo đúng ranh giới: `skill/bmad/` = tooling, `bmad/` gốc = plan.

## Lưu ý quan trọng

- **`bmad/` gốc KHÔNG được git theo dõi** (`/bmad` trong `.gitignore` gốc). Xoá nhầm là mất vĩnh viễn — không có `git fsck` cứu. `skill/bmad/` (tooling) THÌ có git theo dõi bình thường như mọi thứ khác trong `skill/`.
- Lịch sử tới lần dời đầu được giữ trong bundle: `~/backup/skill-20260817-1340.bundle` (`git bundle verify` PASS).
- Luật vòng đời BMAD, feature-slug, quy tắc dọn `done/`: xem `skill/bmad/INDEX.md`.

Thuộc epic `skill-optimization` (story-05, cập nhật 2026-08-30). File này giữ lại để phiên/agent nào còn nhớ path cũ (`bmad/personas/`, `bmad/INDEX.md`...) thì đâm vào đây thay vì đi lạc.
</content>

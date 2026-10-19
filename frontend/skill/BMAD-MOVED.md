# ⚠ BMAD đã dời khỏi `skill/` — 2026-08-17

**Vị trí mới: `bmad/` ở gốc repo `frontend`** (ngang hàng `skill/`, `src/`).

```
skill/bmad/<slug>/   →  bmad/<slug>/
skill/bmad/done/     →  bmad/done/
skill/bmad/personas/ →  bmad/personas/
```

Repo cha erp-cluvix và repo `mobile` làm y hệt: `bmad/` ở gốc mỗi repo.

## Vì sao

`skill/` là knowledge bền, `bmad/` là artifact transient của một feature đang chạy. Để chung thì transient áp đảo: trước khi dời, FE có 6.8k dòng knowledge vs **61.1k dòng** plan/story trong cùng cây.

## Lưu ý

- **`bmad/` KHÔNG được git theo dõi** (`/bmad` trong `.gitignore`). Xoá nhầm là mất vĩnh viễn.
- Lịch sử tới thời điểm dời vẫn nằm trong git history của repo này.
- Luật vòng đời BMAD: `bmad/INDEX.md`.

Thuộc epic `skill-optimization` (story-06).

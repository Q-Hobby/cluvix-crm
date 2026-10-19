Chạy gộp Analyst → PM → Architect (giai đoạn planning BMAD) trong 1 lệnh, khi không cần dừng lại review từng bước.

---

## Bước 0 — Xác định feature-slug

Theo đúng cách xác định trong `skill/bmad/INDEX.md` mục "Feature-slug": tham số truyền vào > tên nhánh git > folder dở dang duy nhất > hỏi user.

---

## Bước 1 — Analyst

Làm theo `.claude/commands/bmad-brief.md` (Bước 1-2) → tạo `bmad/<feature-slug>/brief.md`.

Nếu brief phát sinh "Câu hỏi mở" quan trọng (ảnh hưởng lớn tới PRD/kiến trúc) → dừng lại, hỏi user trước khi qua PM. Câu hỏi nhỏ, không chặn → ghi nhận và tiếp tục.

---

## Bước 2 — PM

Làm theo `.claude/commands/bmad-prd.md` (Bước 1-2) → tạo `bmad/<feature-slug>/prd.md`.

---

## Bước 3 — Architect

Làm theo `.claude/commands/bmad-architect.md` (Bước 1-2) → tạo `bmad/<feature-slug>/architecture.md`.

---

## Bước 4 — Báo cáo

- 3 file đã tạo: `brief.md`, `prd.md`, `architecture.md`.
- Tổng hợp mọi câu hỏi mở còn tồn đọng qua cả 3 giai đoạn.
- Dừng lại chờ user duyệt architecture.md trước khi gợi ý `/bmad-stories` — KHÔNG tự động cắt story trong lệnh này.

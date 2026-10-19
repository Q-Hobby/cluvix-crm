Chạy persona Architect (BMAD) — từ prd.md viết architecture.md.

---

## Bước 0 — Xác định feature-slug

1. Tham số truyền vào (`$ARGUMENTS`) → dùng làm feature-slug.
2. Không có → suy từ tên nhánh git hiện tại, hoặc folder `bmad/<slug>/` duy nhất có `prd.md` nhưng chưa có `architecture.md`.
3. Nhiều folder dở dang / không xác định được → hỏi user.
4. Không có `bmad/<feature-slug>/prd.md` → báo user chạy `/bmad-prd` trước, dừng lại.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/architect.md` — làm đúng theo persona đó, đặc biệt mục "Đọc trước khi viết" (BẮT BUỘC, không bỏ qua).

---

## Bước 2 — Thực thi

1. Đọc `bmad/<feature-slug>/prd.md`.
2. Đọc lần lượt: `skill/INDEX.md` (router 28 skill) → `skill/skills/backend-hard-rules/SKILL.md` → sub-INDEX domain cụ thể → (nếu chạm FE) `skills/working-on-crm/SKILL.md` + `frontend/skill/skills/frontend-hard-rules/SKILL.md`.
3. Tìm module/resource đã có gần giống để tái sử dụng — ưu tiên trước khi đề xuất mới.
4. Viết `bmad/<feature-slug>/architecture.md` theo đúng cấu trúc trong `skill/bmad/personas/architect.md` — KHÔNG lặp lại Hard Rules, chỉ dẫn chiếu `CLAUDE.md`.

---

## Bước 3 — Báo cáo

- File đã tạo: `bmad/<feature-slug>/architecture.md`
- Chờ user duyệt trước khi gợi ý chạy `/bmad-stories` (Design cần approval theo CLAUDE.md).

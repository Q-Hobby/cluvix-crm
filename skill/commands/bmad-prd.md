Chạy persona PM (BMAD) — từ brief.md viết prd.md.

---

## Bước 0 — Xác định feature-slug

1. Tham số truyền vào (`$ARGUMENTS`) → dùng làm feature-slug.
2. Không có → suy từ tên nhánh git hiện tại, hoặc nếu chỉ có đúng 1 folder `bmad/<slug>/` có `brief.md` nhưng chưa có `prd.md` → dùng slug đó.
3. Nhiều folder dở dang cùng lúc / không xác định được → hỏi user.
4. Không có `bmad/<feature-slug>/brief.md` → báo user chạy `/bmad-brief` trước, dừng lại.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/pm.md` — làm đúng theo persona đó.

---

## Bước 2 — Thực thi

1. Đọc `bmad/<feature-slug>/brief.md`.
2. Nếu brief còn "Câu hỏi mở" chưa trả lời → hỏi user trước, không tự giả định.
3. Đọc sub-INDEX domain liên quan theo Quick Decision Map.
4. Viết `bmad/<feature-slug>/prd.md` theo đúng cấu trúc trong `skill/bmad/personas/pm.md`.

---

## Bước 3 — Báo cáo

- File đã tạo: `bmad/<feature-slug>/prd.md`
- Gợi ý user chạy `/bmad-architect` khi PRD được duyệt.

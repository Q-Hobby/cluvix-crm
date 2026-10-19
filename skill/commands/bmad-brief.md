Chạy persona Analyst (BMAD) — biến ý tưởng thô thành brief.md.

---

## Bước 0 — Xác định feature-slug

1. Có tham số truyền vào (`$ARGUMENTS`) → dùng làm gợi ý slug (kebab-case, ngắn gọn).
2. Không có tham số → lấy tên nhánh git hiện tại (`git branch --show-current`), bỏ prefix `feature/`, `fix/`...
3. Không suy ra được → hỏi user tên feature-slug.
4. Tạo `bmad/<feature-slug>/` nếu chưa có.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/analyst.md` — làm đúng theo persona đó (input/output/nguyên tắc).

---

## Bước 2 — Thực thi

1. Ghi nhận yêu cầu thô từ user (`$ARGUMENTS` hoặc context hội thoại).
2. Đọc `CLAUDE.md` root mục "Understand First" + `skill/INDEX.md` Quick Decision Map.
3. Nếu yêu cầu chạm domain cụ thể → đọc sub-INDEX `skill/` tương ứng.
4. Nếu phát hiện đây là fix nhỏ (1-3 dòng, 1 file) → báo user, dừng, không tạo brief.
5. Viết `bmad/<feature-slug>/brief.md` theo đúng cấu trúc trong `skill/bmad/personas/analyst.md`.

---

## Bước 3 — Báo cáo

- File đã tạo: `bmad/<feature-slug>/brief.md`
- Câu hỏi mở còn lại (nếu có) → hỏi user trước khi gợi ý chạy `/bmad-prd`.

Chạy persona SM (BMAD) — cắt prd.md + architecture.md thành epics.md và story files.

---

## Bước 0 — Xác định feature-slug

1. Tham số truyền vào (`$ARGUMENTS`) → dùng làm feature-slug.
2. Không có → suy từ tên nhánh git hiện tại, hoặc folder `bmad/<slug>/` duy nhất có `architecture.md` nhưng chưa có `epics.md`/`stories/`.
3. Nhiều folder dở dang / không xác định được → hỏi user.
4. Thiếu `prd.md` hoặc `architecture.md` → báo user chạy command tương ứng trước, dừng lại.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/sm.md` — làm đúng theo persona đó.

---

## Bước 2 — Thực thi

1. Đọc `bmad/<feature-slug>/prd.md` + `bmad/<feature-slug>/architecture.md`.
2. Xác định các epic (nhóm việc lớn), viết `bmad/<feature-slug>/epics.md`.
3. Với mỗi epic, cắt thành story nhỏ (làm được trong 1 buổi). Với mỗi story: xác định domain → đọc sub-INDEX `skill/` tương ứng → trích Dev Notes cụ thể (path, pattern, Hard Rules áp dụng).
4. Viết từng `bmad/<feature-slug>/stories/story-NN-<slug>.md`, Status = `Draft`.

---

## Bước 3 — Báo cáo

- Liệt kê file đã tạo: `epics.md` + từng `stories/story-NN-*.md`.
- Gợi ý user chạy `/bmad-dev <story-id>` cho story đầu tiên (theo thứ tự phụ thuộc nếu có).

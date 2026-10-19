Chạy persona Dev (BMAD) — implement 1 story cụ thể.

Tham số: `$ARGUMENTS` = story-id (vd `story-02` hoặc `story-02-checkout-discount`).

---

## Bước 0 — Xác định feature-slug + story

1. Không có `$ARGUMENTS` → hỏi user story-id cần làm.
2. Tìm file `bmad/<feature-slug>/stories/story-NN-*.md` khớp story-id:
   - Nếu chỉ định rõ feature-slug (vd `<slug>/story-02`) → dùng luôn.
   - Không rõ feature-slug → tìm trong toàn bộ `bmad/*/stories/` theo số/tên story; khớp đúng 1 file thì dùng, khớp nhiều file thì hỏi user chọn.
3. Không tìm thấy story → báo user chạy `/bmad-stories` trước, dừng lại.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/dev.md` — làm đúng theo persona đó.

Đọc `skill/bmad/executor-contract.md` — luật vận hành chống đụng nhau (port/build/đĩa/git/file chung). BẮT BUỘC, không bỏ qua kể cả story nhỏ.

---

## Bước 2 — Thực thi

1. **Resume-by-status** — đọc `Status` + `## Checkpoint` của story TRƯỚC:
   - `In Progress` → RESUME từ Checkpoint (đọc "AC đã xong"/"File đã đụng"/"Việc kế tiếp"), KHÔNG làm lại từ đầu.
   - `Review`/`Done` → HỎI user (mặc định không re-implement). `Blocked` → báo điều kiện block, dừng.
   - `Draft` → làm từ đầu.
2. Đọc kỹ story: Mô tả, AC, Dev Notes, Ghi chú kỹ thuật.
3. Mở các file `skill/` được Dev Notes dẫn chiếu để đọc đầy đủ, không chỉ tin phần trích ngắn.
4. Đọc `CLAUDE.md` root Hard Rules — áp dụng bắt buộc.
5. Cập nhật Status story → `In Progress`.
6. Implement đúng phạm vi story (KHÔNG lan sang story khác). **Cập nhật `## Checkpoint` liên tục** sau mỗi bước có ý nghĩa (xong 1 AC / file lớn / trước build) — điểm resume nếu bị ngắt.
7. Build **scope theo path đã đổi** (`go build ./internal/modules/webapp/<module>/...`) và/hoặc `npx tsc --noEmit`. KHÔNG `go build ./...` mỗi story — full repo chỉ 1 lần cuối wave (xem `skill/bmad/executor-contract.md` §2).
8. Build/tsc pass → cập nhật Status → `Review`, thêm dòng vào "Log thực hiện" (ngày, file đã đổi, tóm tắt).

---

## Bước 3 — Báo cáo

- Story đã cập nhật: đường dẫn file story.
- File code đã thay đổi.
- Kết quả build/tsc.
- Gợi ý user chạy `/bmad-qa <story-id>`.

Chạy persona QA (BMAD) — review story đã implement (Status = Review).

Tham số: `$ARGUMENTS` = story-id.

---

## Bước 0 — Xác định feature-slug + story

1. Không có `$ARGUMENTS` → hỏi user story-id cần review.
2. Tìm file `bmad/<feature-slug>/stories/story-NN-*.md` khớp story-id (cùng cách tìm như `/bmad-dev`).
3. Story chưa ở Status `Review` → hỏi user xác nhận có muốn review sớm không, mặc định dừng lại.

---

## Bước 1 — Đọc

Đọc `skill/bmad/personas/qa.md` — làm đúng theo persona đó.

---

## Bước 2 — Thực thi

1. Đọc story: Mô tả, AC, Dev Notes.
2. `git diff` để xem code thay đổi tương ứng story (nếu chưa commit: working tree diff; nếu đã commit: diff của commit liên quan — hỏi user commit nào nếu không rõ).
3. Đọc `skill/skills/backend-hard-rules/SKILL.md` (+ file `skill/` khác story dẫn chiếu) và `CLAUDE.md` Hard Rules để đối chiếu độc lập.
3b. Đọc `skill/bmad/executor-contract.md` — QA tự chạy build/test/server nên phải theo đúng luật vận hành (KHÔNG bind 8081, KHÔNG `git checkout/stash`, build scope theo path).
4. Đối chiếu từng AC — pass/fail cụ thể.
5. Cân nhắc chạy `/code-review` nếu cần soi sâu logic/performance (`/post-review` đã gộp vào đó 2026-09-02; nghiệm thu theo yêu cầu → `skill/skills/backend-hard-rules/reference/review-checklist.md`).
6. Kiểm tra bằng chứng build/tsc pass (chạy lại nếu nghi ngờ).

---

## Bước 3 — Kết luận

- Tất cả AC pass + không vi phạm Hard Rules → cập nhật Status story → `Done`.
- Có vấn đề → giữ `In Progress`, ghi rõ lý do (AC nào fail, rule nào vi phạm, cần sửa gì) vào "Log thực hiện".
- Không tự sửa code — chỉ khi user yêu cầu rõ.

---

## Bước 4 — Báo cáo

- Story: đường dẫn + Status mới.
- Danh sách AC: pass/fail.
- Nếu trả về Dev: lý do cụ thể, actionable.

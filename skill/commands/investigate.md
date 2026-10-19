Root-cause debugging cho bug/incident đã xảy ra — bổ trợ BMAD (BMAD lo feature có kế hoạch, `/investigate` lo bug phát sinh). KHÔNG thay thế Development Protocol ở CLAUDE.md root, chỉ hiện thực hoá nghiêm ngặt hơn phần "Correctness over Speed" cho ca debug.

**Iron Law: không sửa code khi chưa xác nhận được nguyên nhân gốc bằng bằng chứng cụ thể (log/query/test tái hiện). Đoán rồi sửa thử không phải là investigate.**

---

## Bước 0 — Input

Có `$ARGUMENTS` mô tả triệu chứng → dùng làm điểm bắt đầu. Không có / mô tả mơ hồ ("bug ở chỗ X không đúng") → hỏi lại cụ thể: bước tái hiện, kết quả mong đợi vs thực tế, từ khi nào / sau thay đổi gì bắt đầu xảy ra. KHÔNG tự đoán triệu chứng.

## Bước 1 — Trace trước khi đoán

1. Trace data flow từ nơi phát hiện triệu chứng ngược lên nguồn: FE component → service → HTTP endpoint → BE handler → service → repo → DB (đọc file thật, không giả định — như `/check-component` bước 0).
2. Đối chiếu với bug pattern đã biết trước khi nghĩ giả thuyết mới: `skill/skills/backend-hard-rules/SKILL.md` (Hard Rules — tenant scope, decimal, migration idempotent...) và các memory bug pattern đã ghi nhận (vd FK nullable=0, NULL tenant scope, race điều kiện auth...). Nhiều bug lặp lại đúng những pattern này.

## Bước 2 — Giả thuyết có thể kiểm chứng

Với mỗi giả thuyết, nêu rõ: **cách kiểm chứng cụ thể** (query DB, log, test case tái hiện, `go build`/`tsc` nếu là lỗi kiểu) — không đưa giả thuyết mà không có cách xác minh kèm theo.

- Giả thuyết đúng (có bằng chứng) → sang Bước 3.
- Giả thuyết sai → quay lại, đưa giả thuyết khác dựa trên bằng chứng vừa loại trừ.
- Sai liên tiếp 3 giả thuyết → DỪNG. Trình bày những gì đã loại trừ + bằng chứng, hỏi user thay vì tiếp tục đoán.

## Bước 3 — Chốt nguyên nhân gốc

Trình bày: nguyên nhân gốc, bằng chứng xác nhận, phạm vi ảnh hưởng (chỗ khác trong code có cùng pattern lỗi không — grep để kiểm tra, đừng chỉ sửa 1 chỗ rồi bỏ qua chỗ tương tự).

Sau đó mới đề xuất fix, theo đúng Bước 3 (Implement) của Development Protocol — surgical, không refactor thừa.

## Bước 4 — Sau khi fix

Nếu nguyên nhân gốc là pattern có thể tái diễn ở module khác (không chỉ lỗi logic một lần) → cân nhắc chạy `/save-skill` để ghi lại thành bug pattern, tránh lặp lại điều tra từ đầu ở lần sau.

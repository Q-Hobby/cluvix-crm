# Prompts Playbook — chạy 1 đợt BMAD lớn tự hành (đúc kết từ fhir-compliance-2026, 2026-07-21)

Bộ prompt cấp lệnh theo trình tự + quy tắc nền. Prompt ngắn được vì **cấu trúc nằm ở repo** (artifact BMAD, story tự chứa, Checkpoint) — thiếu Phần 2 thì Phần 1 vô dụng.

## Phần 1 — Bộ prompt theo trình tự

| # | Khi nào | Prompt (generic) |
|---|---|---|
| P1 | Khởi động — đánh giá trước, code sau | "Lên plan đánh giá dự án hiện tại so với [chuẩn/mục tiêu X], đồng thời research để hiểu rõ [X]. Giao sub-agent model thích hợp, còn bạn tổng hợp và làm việc trực tiếp với tôi." |
| P2 | Ép kế hoạch vào repo | "Lên plan chi tiết, lưu TOÀN BỘ vào file trong repo theo quy trình BMAD — memory là bạn tự nhớ, tôi phải nắm được và duyệt được từng phần." |
| P3 | Review thiết kế trước duyệt | "Duyệt [architecture] càng kỹ càng tốt — nhiều reviewer độc lập khác lens, giả định NÓ CÓ LỖI phải đi tìm, đối chiếu code thật không tin văn bản tự khai. Sửa xong báo gì đã sửa, gì cần tôi quyết." |
| P4 | Cắt việc | "Cắt story nhỏ (1 buổi/story), mỗi story TỰ CHỨA context (path/pattern/gotcha trích sẵn), kèm Checkpoint để chết giữa chừng resume đúng chỗ." |
| P5 | Chạy tự hành | "Chạy dev tất cả story tới hết không hỏi lại. Model theo policy Planner/Executor (CLAUDE.md); tối đa N song song, wave theo dependency, không 2 agent cùng sửa 1 file." |
| P6 | Điều tiết realtime | "Đang online, nâng N agent." / "Sắp hết limit, cái nào xong tạm dừng." / "Limit reset, tiếp tục — resume từ Checkpoint, đừng làm lại." |
| P7 | Nạp input gốc | "[Văn bản X] đã bỏ vào [folder] — đọc NGUYÊN VĂN rồi làm, đừng tin nguồn thứ cấp." |
| P8 | Duyệt gate | "Duyệt [spec/IG], chạy tiếp [epic] đi." |
| P9 | Chạy đến hết + hợp đồng báo cáo | "Chạy dần hết. Xong báo tôi: việc còn sót, việc cần confirm, thiếu gì báo ngay." |
| P10 | Đúc kết | "Lưu bài học vào skill để lần sau an toàn hơn. Note vào plan gì hoàn thành / còn thiếu để tôi rà soát." |

## Phần 2 — 8 quy tắc nền bắt buộc

1. **Artifact sống trong repo**: brief/prd/architecture/story/plan = file; trạng thái (Status/Checkpoint) trong file → chết session/đổi agent vẫn resume.
2. **Story tự chứa context**: executor rẻ chỉ đúng khi được trích sẵn path/pattern/gotcha — bắt tự tra là đổi rẻ lấy sai.
3. **Review/QA adversarial mặc định**: giả định có lỗi; verify bằng code thật + test thật trên DB dùng-một-lần. (Đợt mẫu: bắt 3 CRITICAL trước khi code — sender không tồn tại, hash gap, tenantTables.)
4. **Discovery trước build** với mọi thứ phụ thuộc văn bản/spec ngoài — không bịa cấu trúc khi chưa đọc nguyên văn; file thiếu trang → dừng đúng chỗ thiếu, báo user.
5. **Kỷ luật song song**: wave theo dependency; file dùng chung edit additive + đọc-mới-nhất; chi tiết + sự cố mẫu: root `CLAUDE.md` §Subagent Model Policy → Ràng buộc vận hành.
6. **Máy chặn lỗi mọi cửa**: build/tsc/validator/integration là ĐIỀU KIỆN Review, không phải lời hứa.
7. **Contract 2 đầu chạy song song → đối chiếu code thật trước Review**: story BE + FE của CÙNG contract chạy song song thì story sau (hoặc story tích hợp) BẮT BUỘC đối chiếu code phía kia THẬT (route/handler/entity), không tin architecture.md làm nguồn cuối — ghi thẳng vào AC "verify contract với code thật". (Đợt mẫu hr-ot-request: 3 lệch flat/nested, `reason`, YYYYMM.)
8. **KHÔNG giành port 8081 của user (sự cố 2026-08-17)**: user luôn chạy `air` trên 8081; executor chạy `go run .`/`./tmp/main`/`go build -o ./tmp/main` là giết dev server của user (run_dev.sh cố ý `exit 1` thay vì kill process lạ → air KHÔNG tự phục hồi). Cần server thật → chỉ `backend/scripts/run_qa.sh` (port 8079, tự kill + xoá binary khi xong). Mặc định không cần server: build + package test là đủ.

## Tham chiếu

- Model policy Planner/Executor + ràng buộc vận hành: root `CLAUDE.md` §Subagent Model Policy.
- Vòng đời + persona + Checkpoint/resume: `skill/bmad/INDEX.md`.
- Đợt mẫu đầy đủ (31 story, 2 sự cố, QA 3 cụm): `bmad/fhir-compliance-2026/` ở gốc repo (gitignored — có thể đã dọn sang `bmad/done/`).

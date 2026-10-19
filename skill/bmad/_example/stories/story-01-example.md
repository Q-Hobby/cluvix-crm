# Story 01: <tên ngắn>

> File mẫu minh hoạ cấu trúc. Xem `personas/sm.md` để biết cách viết story thật.

Status: Draft
Baseline: —        <!-- Dev ghi `git rev-parse HEAD` khi bắt đầu (NO_VCS nếu không có) — neo diff cho QA -->
Review-loop: 0     <!-- QA tăng +1 TRƯỚC mỗi loopback story_defect; kết quả >3 → Blocked (review không hội tụ) -->
Followup-review: — <!-- QA set khi Done: yes nếu pass gần nhất có patch high hoặc 3×medium + 1×low ≥ 5, ngược lại no -->

## Mô tả
[Là 1 phần việc cụ thể, làm được trong 1 buổi]

## Acceptance Criteria
- [ ] AC1
- [ ] AC2

## Dev Notes (context trích sẵn, Dev KHÔNG cần tự tra skill/ lại)
- Module/path liên quan: ...
- Pattern cần theo: ...
- Hard Rules áp dụng cho story này: ...
- File/entity liên quan đã có (tái sử dụng): ...

## Ghi chú kỹ thuật khác
- Migration cần thiết (nếu có): ...
- Rủi ro đã biết từ architecture.md: ...

## Checkpoint (Dev cập nhật LIÊN TỤC khi làm — điểm resume nếu bị ngắt)
- Bước hiện tại: —
- AC đã xong: —
- File đã đụng: —
- Việc kế tiếp: —
- Blocked bởi (chỉ khi Status=Blocked): —

## Story Change Log (APPEND-ONLY — QA ghi khi trả story_defect; KHÔNG sửa/xoá entry cũ)
<!-- Mỗi entry: finding kích hoạt · đã sửa gì trong story (KHÔNG đổi AC gốc — AC sai là ac_gap, phải hỏi user)
     · known-bad tránh · KEEP: cái Dev đã làm đúng phải giữ khi làm lại.
     Dev đọc TOÀN BỘ section này trước khi re-implement. Trống cho tới loopback đầu tiên. -->

## Review Triage Log (APPEND-ONLY — QA/`/bmad-review` append MỖI pass, kể cả pass sạch/Blocked; KHÔNG sửa entry cũ)
<!-- ### [ngày] — Pass N (/bmad-qa | /bmad-review)
     - ac_gap / story_defect / patch / defer / reject: mỗi loại `0` hoặc `N (high h, medium m, low l)`
     - addressed:
       - [severity][loại] <finding + đã xử gì trong pass này>   (hoặc `- none`)
     Trống cho tới pass review đầu tiên. -->

## Log thực hiện (Dev/QA cập nhật)
- [ngày] [ai] [trạng thái mới] — [ghi chú]

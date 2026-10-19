Retrospective cấp EPIC dựa trên bằng chứng — đọc lại toàn bộ dấu vết một epic đã xong, săn lỗi mà
không story lẻ nào thấy được, rồi ra phán quyết nghiệm thu.

Tham số: `$ARGUMENTS` = feature-slug (vd `mysql97-backend-alignment`). Tìm ở
`bmad/<slug>/` rồi `bmad/done/<slug>/`. Không thấy → hỏi user.

---

## Vì sao cần lệnh này (đừng bỏ qua)

`/bmad-qa` và `/bmad-review` soi **1 story / 1 diff**. Chúng mù với khuyết tật là **thuộc tính của cả
epic**: 5 story mỗi story thêm 200 dòng, không story nào thấy cái class 1.000 dòng chúng cùng dựng lên.
Lệnh này đọc **toàn bộ** diff của epic, không phải từng lát cắt.

**Luật vàng — không có nguồn thì không phải finding.** Mỗi finding phải chỉ được `file:dòng`, commit,
hoặc output lệnh. Không chỉ được → **bỏ**, không ghi. Ưu tiên **suy ra bằng script** (`git`, `grep`,
`wc`) — số đo là bằng chứng, cảm nhận của model thì không.

---

## Bước 0 — Dựng phạm vi bằng chứng

1. Đọc `epics.md`, mọi `stories/story-*.md`, `deferred-work.md` (nếu có).
2. Lấy **dải diff**: `Baseline` ở header story đầu tiên → `HEAD`. Story không có Baseline → hỏi user
   hoặc lấy commit đầu tiên chạm path của epic. Code CHƯA commit ⇒ dải diff = Baseline → working tree,
   khoanh vùng BẮT BUỘC theo "File đã đụng" trong Checkpoint từng story rồi loại hunk của phiên khác
   theo `@@` header (`git diff` không biết ai viết dòng nào); Checkpoint thiếu file ⇒ retro không có
   phạm vi — ghi rõ là không quét được, KHÔNG đoán (2026-09-02).
3. **Hai repo**: chạy riêng ở repo gốc VÀ `skill/` (`skill/` là git repo lồng). Feature đụng
   `frontend/`/`mobile/` → chạy thêm ở đó.
4. ⚠ **Khoanh phạm vi theo PATH, KHÔNG theo commit message.** Message trong repo này không phản ánh
   nội dung — thật 2026-09-10: `39d5a117a "Fix gg cal"` chứa toàn EMR dashboard + med-DQG, không một
   file gcal nào. Lọc bằng `-- <path>`, rồi đối chiếu với "File đã đụng" trong Checkpoint từng story.
5. ⚠ **Baseline là hash của MỘT repo.** `frontend/` (và `mobile/`) là git repo riêng — hash Baseline
   của repo gốc dùng ở đó sẽ trả về **rỗng, exit 0**, không báo lỗi gì. Dải diff FE phải lấy riêng:
   `git log --oneline --reverse -- <path FE> | head -1`.
6. Ghi lại con số thô trước khi diễn giải:
   ```bash
   git log --oneline <base>..HEAD -- <path>
   git diff --stat <base>..HEAD -- <path>
   git diff <base>..HEAD --numstat -- <path> | sort -rn | head -15   # file churn cao nhất
   ```
7. Phạm vi nào **không** quét được (repo không truy cập, diff quá lớn, file binary) → ghi ra, đừng
   im lặng bỏ.

---

## Bước 1 — 5 view cấp epic (cái mà story lẻ không thấy)

Với mỗi view: có finding thì kèm nguồn; không có thì ghi "đã kiểm, sạch" + đã kiểm bằng cách nào.

1. **Trùng lặp giữa các story** — hai story giải cùng một bài theo hai cách, hoặc story sau viết lại
   helper story trước đã có. Tìm bằng: tên hàm/hằng/pattern lặp trong diff.
2. **File phình qua cả epic** — xếp hạng theo `--numstat`, rồi **mở file đọc kích thước và cấu trúc
   thật** trước khi kết luận. Churn cao = ứng viên cần soi, **không phải** phán quyết.
3. **Lệch pattern với codebase xung quanh** — đặt tên, xử lỗi, cấu trúc test, ranh giới module. Agent
   học convention bằng cách nhìn code cũ, nên lệch là lỗi cộng dồn.
4. **Đối chiếu spec ↔ as-built** — so `prd.md`/`architecture.md` với thứ thực sự đã build. Mỗi chỗ
   lệch phải xếp đúng 1 trong 3: **lỗi** (sửa) · **lệch có chủ đích** (ghi lại để retro sau không
   báo lại) · **spec cần cập nhật theo thực tế** (đề xuất, KHÔNG tự sửa spec).
5. **Hard Rules toàn epic** (thay "architecture delta" của upstream — hợp dự án này hơn). Quét TOÀN
   BỘ diff, không phải từng story:
   ```bash
   grep -rn "MUTATION\|if false &&" --include='*.go' internal/ pkg/     # phải = 0
   grep -rnE "db\.DB\.(First|Find)\(&[a-z]+, *id" --include='*.go'      # tenant scope
   grep -rn "GetCompanyID(" --include='*.go'                            # đã xoá, không được tái sinh
   ```
   Cộng: tiền có `decimal` không · migration idempotent + FK thật · `SELECT *` · timezone ·
   **NFR2: có lệnh nào trong log/story trỏ test/migrate vào `DB_NAME` không**.

Có subagent → giao mỗi view 1 con fresh-context, **trả bằng chứng + phạm vi đã kiểm, KHÔNG trả phán
quyết**; cha tổng hợp và quyết. Không có → làm inline view 4 và 5 trước (giá trị cao nhất), ghi rõ
view nào bị thu hẹp hoặc bỏ.

---

## Bước 2 — Đối chiếu nợ của retro/epic trước

Đọc `deferred-work.md`. Với mỗi mục **chưa** đánh dấu xong:
- **Cách gọi tên nó**: số mục + câu tóm tắt nguyên văn.
- **Đã làm chưa** — kèm nguồn chứng minh (commit/file:dòng/test). Không tìm thấy bằng chứng thì ghi
  **"không tìm thấy bằng chứng"**, KHÔNG ghi "chưa làm" — người đọc phải phân biệt được *đã kiểm mà
  không thấy* với *chưa kiểm*.
- **Trạng thái đề xuất** — chỉ đề xuất, không tự sửa file.

Không có `deferred-work.md` → ghi rõ là *không có file*, khác với *có file mà rỗng*.

---

## Bước 3 — Định tuyến finding (mỗi finding HAI quyết định độc lập)

| Quyết định | Giá trị |
|---|---|
| Xử lý ca này | `fix-now` · `defer` · `accept-as-is` |
| Chặn ca sau | bài học thượng nguồn: sửa câu chữ AC · cắt story khác đi · thiếu convention/lưới · hoặc *không có* |

Cột 2 là chỗ một finding lặp lại biến thành **thay đổi quy trình** thay vì vá lẻ.

Finding đến từ subagent hoặc từ log của Dev là **báo cáo chưa kiểm chứng**. Trước khi biến thành hành
động: mở lại nguồn gốc xác minh. Nguồn không đứng vững → **bỏ**, không định tuyến.

Lệnh này **đề xuất, không tự thi hành**: không tự sửa code, không tự sửa `prd.md`/`architecture.md`.

---

## Bước 4 — Phán quyết nghiệm thu

So trạng thái cuối với AC mà chính epic đặt ra. Epic không khai AC → dựng AC từ diff + story rồi ghi
rõ phán quyết là **suy ra**, không phải đã khai.

- **Accepted** — AC đạt có bằng chứng, 0 finding chặn, **mọi story `Done`**.
- **Accepted-with-open-items** — AC đạt, còn finding đã defer và có người theo dõi; vẫn đòi **mọi
  story `Done`**.
- **Rejected** — AC không đạt, HOẶC còn finding chặn chưa xử, HOẶC **bất kỳ story nào chưa `Done`**
  (kể cả `Blocked`).

Ba luật cứng:
1. Quyết định của người **luôn** đè phán quyết máy.
2. Epic trượt AC mà **không** có quyết định của người → ghi **"không nghiệm thu"**, TUYỆT ĐỐI không
   âm thầm ghi accepted.
3. Story chưa `Done` → phán quyết máy là **rejected**. Không được làm nhẹ thành
   accepted-with-open-items: giao hàng dở dang không phải "finding mở của một epic đã xong" — bản
   thân epic chưa xong.

---

## Bước 5 — Ghi kết quả

Tạo `bmad/<slug>/retrospective.md` (hoặc trong `done/<slug>/`):

```markdown
# Retrospective — <slug>            (ngày, dải diff, repo đã quét)
## Phán quyết: Accepted | Accepted-with-open-items | Rejected | Không nghiệm thu
## Số liệu thô            <- commit, file, +/- dòng, mỗi repo
## 5 view cấp epic        <- mỗi view: finding kèm nguồn, HOẶC "đã kiểm, sạch — kiểm bằng ..."
## Nợ epic trước          <- đã làm / không tìm thấy bằng chứng / không có file
## Finding + định tuyến   <- bảng 2 cột quyết định
## Phạm vi KHÔNG quét được
```

Báo cáo cho user: phán quyết + finding chặn + việc cần user quyết. Nợ mới → append
`deferred-work.md` (không sửa entry cũ). Bài học quy trình đáng giữ → gợi ý `/save-skill`.

---

## Cấm

- Finding không có nguồn chỉ được.
- Tự sửa code / spec / story.
- Ghi accepted khi còn story chưa `Done`.
- Im lặng bỏ qua phạm vi không quét được.

Chạy TUẦN TỰ vòng Dev → QA → fix → QA cho mọi story còn dở của 1 epic, dừng lại đúng chỗ cần người.

Tham số: `$ARGUMENTS` = feature-slug (`mysql97-backend-alignment`), tuỳ chọn thêm story-id để bắt
đầu từ đó. Không rõ slug → hỏi user.

---

## Ranh giới của lệnh này

**Làm:** lặp `/bmad-dev <story>` → `/bmad-qa <story>` → nếu QA ra `patch` thì quay lại Dev → QA lại,
cho tới khi story `Done`; xong story này sang story kế theo thứ tự phụ thuộc; hết story thì chạy
`/bmad-retro`.

**KHÔNG làm** (không thương lượng):
- **KHÔNG chạy song song.** Một story tại một thời điểm. Lý do: fan-out build Go song song đã gây
  ENOSPC (19GB cache/buổi) và RAM 30-40GB — xem `CLAUDE.md` §Ràng buộc vận hành.
- **KHÔNG commit.** Kể cả khi mọi story `Done`. Commit là quyết định của user.
- **KHÔNG chạm DB thật.** Verify chỉ trên `*_tmp` dùng-một-lần (NFR2).
- **KHÔNG tự sửa `architecture.md` / `prd.md` / AC gốc.** Thấy sai → dừng, hỏi.
- **KHÔNG tự vượt HARD STOP** dưới đây bằng cách suy diễn ý user.

---

## HARD STOP — gặp là dừng cả run, báo user, KHÔNG chạy story kế

Bảng này rút từ 5 lần thực tế phải dừng trong epic `mysql97-backend-alignment` (2026-08-13).

| # | Điều kiện | Vì sao không tự quyết được |
|---|---|---|
| S1 | QA triage ra **`ac_gap`** | AC/ý định thiếu hoặc mơ hồ — suy diễn ý user là đoán |
| S2 | Phát hiện **`architecture.md` sai/thiếu** giữa lúc Dev | Tiền lệ: gap `ALTER DATABASE` — thêm AC mới là đổi kiến trúc, Dev không có thẩm quyền |
| S3 | Story cần **user thao tác tay**: smoke browser/UI, truy cập production | Agent không có mắt người/quyền prod. (Server BE thì agent TỰ chạy được: `backend/scripts/run_qa.sh` port 8079 — đừng dừng ở S3 chỉ vì cần gọi API) |
| S4 | **`Review-loop` > 3** | Review không hội tụ — vá thêm chỉ đào sâu |
| S5 | Phải **viết lại phần đã verify xong** | Tiền lệ: rút gọn story-01 843→72 dòng. Đổi "sạch hơn" lấy "verify lại từ đầu" là cái giá user trả |
| S6 | Phải sửa **file dùng chung đang có việc chưa commit của phiên khác** | Tiền lệ: `skill/INDEX.md`. Commit sẽ cuốn theo việc của họ |
| S7 | Việc **khó đảo ngược**: apply migration lên DB thật, xoá dữ liệu, đẩy remote | Luôn cần user chốt |
| S8 | **Chính story ghi điều kiện phải hỏi**: Dev Notes/AC có câu "hỏi user trước khi…", "cần user quyết", "KHÔNG tự chọn…" | SM đã lường trước chỗ mơ hồ; bỏ qua là phá ý định của story. Tiền lệ: story-01 Dev Notes yêu cầu hỏi trước khi chọn phương án hardcode |

> S8 chỉ kích hoạt khi **điều kiện trong câu đó thành hiện thực**. Story-01 ghi "nếu phương án (1)
> quá xấu thì hỏi user" — Dev chọn được (1) mà vẫn giữ AD1 nên **không** phải hỏi, run chạy tiếp.
> Đọc điều kiện, đừng thấy chữ "user" là dừng.

Gặp HARD STOP → set Status story = `Blocked`, ghi **điều kiện gỡ block cụ thể** vào Checkpoint, báo
user, **dừng run**. Không nhảy sang story kế: story sau thường phụ thuộc story trước, chạy tiếp trên
nền chưa chốt là nhân bản lỗi.

---

## Bước 0 — Preflight (làm trước, đừng bỏ)

0. Đọc `skill/bmad/executor-contract.md` — luật vận hành áp cho cả run này và mọi story bên trong.
1. `df -h /System/Volumes/Data` — còn **< 15GB** thì dừng, báo user (`rm -rf ~/Library/Caches/go-build`).
2. `git status --short` ở repo liên quan → ghi lại **file đang bẩn của phiên khác**. Suốt run, mọi
   file trong danh sách này là vùng cấm sửa (S6).
3. Đọc `epics.md`: thứ tự story + **phụ thuộc**. Story phụ thuộc story chưa `Done` → chưa tới lượt.
4. Liệt kê story sẽ chạy kèm Status hiện tại. **Trình danh sách cho user xem trước khi bắt đầu** —
   đây là lần cuối user thấy toàn cảnh trước khi run tự chạy.

---

## Bước 1 — Vòng lặp mỗi story

```
Draft/In Progress → /bmad-dev <story>  → Status: Review
                  → /bmad-qa  <story>  → Done | In Progress(patch) | Blocked
     ┌─ In Progress(patch) ──► /bmad-dev sửa đúng finding ──► /bmad-qa lại
     │                          (tối đa 3 vòng/story, vượt = S4)
     └─ Done ──► story kế
```

- **Resume-by-status** (không chạy lại từ đầu): `Draft` → làm mới · `In Progress` → đọc `Checkpoint`
  làm tiếp · `Review` → chạy thẳng QA · `Done` → bỏ qua · `Blocked` → HARD STOP ngay.
- Dev sửa theo finding của QA thì **chỉ sửa đúng finding đó**, không nhân tiện làm thêm.
- QA pass sau vẫn phải săn thật, không chỉ kiểm lại finding cũ — bug hay nằm **trong chính bản vá**
  (tiền lệ: `MESSAGE_TEXT > 128 ký tự` làm guard bắn nhưng nuốt mất lý do).
- Trước khi giao MỖI story: đo lại `df -h /System/Volumes/Data` — story Go build + test integration nạp lại
  ~2–3GB cache. Dưới 15GB ⇒ xoá go-build cache ngay rồi mới giao, KHÔNG để Dev tự Blocked giữa chừng.
- Build **scope theo path story đã đổi** (`go build ./internal/modules/webapp/<module>/...`), KHÔNG
  `./...` mỗi story — RAM. Full `go build ./...` **1 lần** ở Bước 3.

---

## Bước 2 — Giữa các story

- Cập nhật dòng trạng thái đầu `epics.md` (bao nhiêu story Done / còn lại).
- Nợ phát sinh → append `deferred-work.md`, KHÔNG sửa entry cũ.
- Báo user 1 dòng ngắn mỗi khi 1 story chuyển `Done` — để user theo dõi được và ngắt nếu thấy sai.

---

## Bước 3 — Khi mọi story `Done`

1. `go build ./...` (lần duy nhất) + `npx tsc --noEmit` nếu đụng FE.
2. `grep -rn "MUTATION\|if false &&" --include='*.go' internal/ pkg/` → phải **0**.
3. **`/bmad-retro <slug>`** — bắt buộc theo `skill/bmad/INDEX.md` §Bước dọn.
4. Báo cáo tổng + **dừng**. KHÔNG commit, KHÔNG chuyển `done/` — hai việc đó user quyết sau khi
   đọc phán quyết retro.

---

## Báo cáo cuối

```
Epic <slug>: N/M story Done
  story-01  Done   (QA 1 pass)
  story-02  Done   (QA 2 pass — 2 patch đã vá)
  story-03  Blocked → S3: cần user boot app chạy seeder RBAC
Dừng ở: story-03 (HARD STOP S3)
Chưa chạy: story-04, story-05 (phụ thuộc story-03)
Retro: chưa chạy (còn story chưa Done)
Chưa commit.
```

Bị ngắt giữa chừng (limit/lỗi) → chạy lại `/bmad-run <slug>`, nó tự resume theo Status + Checkpoint.

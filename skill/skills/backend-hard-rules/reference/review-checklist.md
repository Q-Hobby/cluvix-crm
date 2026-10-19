# Checklist nghiệm thu code vừa implement (BE + FE)

> **Đây là tài liệu, không phải slash command** (đổi 2026-09-02: `/post-review` đã gộp vào
> `/code-review` để bớt cửa review chồng lấn). Built-in `/code-review` săn bug + dọn dẹp trên
> diff, nhưng KHÔNG làm 2 việc dưới đây — cần thì đọc file này rồi làm thêm:
>
> 1. **Truy vết yêu cầu** — thay đổi nào THỪA, và quan trọng hơn: thay đổi nào **THIẾU** so với
>    plan/story. Diff không tự tố cáo phần chưa làm.
> 2. **Checklist hard-rules Cluvix** — tenant scope, company scope từ session, decimal tiền,
>    response/i18n, timezone (`../SKILL.md`).
>
> Dùng khi nghiệm thu một mảng việc đã xong, không phải mỗi lần sửa vài dòng.

**Stack:** Go (Gin/GORM/Redis) · Angular · MySQL multi-tenant

---

## Bước 0 — Scope (LUÔN làm đầu tiên)

1. `git diff HEAD~1 --stat` (hoặc `git diff --cached --stat`) → danh sách file thay đổi.
2. Đọc `skill/skills/backend-hard-rules/SKILL.md` — Hard Rules cho toàn bộ review.
3. Đọc đủ file thay đổi theo import chain — KHÔNG giả định.
4. User cung cấp task/plan → đây là ground truth. Không có → **hỏi ngay, KHÔNG suy đoán từ git diff**.

---

## Bước 1 — Requirement Traceability

- Mỗi thay đổi có map về yêu cầu? Code thừa / feature ngoài scope?
- Thay đổi nào **thiếu** so với yêu cầu?
- Scope bị creep?

---

## Bước 2 — Hard Rules Checklist (chỉ check file đã thay đổi)

**Backend (Go):**
- [ ] Check từng mục Hard Rules trên file đã thay đổi → Xem `skill/skills/backend-hard-rules/SKILL.md` (đã đọc ở Bước 0)

**Frontend (Angular):**
- [ ] Subscription: `takeUntilDestroyed` / async pipe / unsubscribe?
- [ ] `OnPush` change detection?
- [ ] Không gọi API thừa hoặc duplicate?

---

## Bước 3 — Logic & Correctness

- Business logic đúng và nhất quán với nghiệp vụ?
- Edge case bị bỏ sót: null/zero/empty, boundary, concurrent request?
- State transition hợp lệ? (status flow, soft-delete, active/inactive)
- Transaction: rollback đủ? Partial write?
- Error handling: path nào silent fail?

---

## Bước 4 — Performance

- Query N+1? (loop + query bên trong, missing preload)
- Query thừa? (fetch data không dùng)
- Missing index trên WHERE/JOIN column mới?
- Cache invalidate đúng chỗ? Stale data risk?
- Worst case với N lớn (10k, 100k)?

---

## Bước 5 — Trình bày vấn đề

```
**[Tên vắn tắt]**
- File: path/to/file.go:line
- Severity: Critical / High / Medium / Low
- Vấn đề: [mô tả cụ thể]
- Tác động: [ảnh hưởng user/data/performance]
- Fix: [cách sửa — không workaround]
```

---

## Bước 6 — Action Table

| Fix ngay được | Cần design/approval | Pass (OK) |
|---|---|---|
| ... | ... | ... |

**Verdict cuối:** PASS / PASS WITH MINOR FIXES / NEEDS REWORK

---

## Bước 7 — Verify

- `go build ./...` pass
- `npx tsc --noEmit` pass
- Không regression ở flow liên quan

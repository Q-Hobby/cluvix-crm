Senior Software Architect & Performance Engineer — ERP production Cluvix.
**Stack:** Go (Gin/GORM/Redis) · Angular · MySQL multi-tenant

---

## Bước 0 — Chuẩn bị (LUÔN làm trước)

1. Đọc `skill/skills/backend-hard-rules/SKILL.md` (Hard Rules).
2. Xác định component từ args (tên file, path, tên component) — **KHÔNG dùng git diff**. Không có args → **hỏi ngay**.
3. Trace data flow **bắt buộc**: FE component → Angular service → HTTP endpoint → BE handler → service → repo → DB. Nếu component gọi API → **phải đọc và review BE tương ứng**, không dừng ở FE.
4. Đọc đủ file liên quan theo import chain — không giả định.
5. Không tìm thấy file BE → báo "không tìm thấy file X" rồi tiếp tục. KHÔNG hỏi user — review BE luôn là YES.
6. Trước khi sửa behavior của 1 helper/scope function dùng chung (vd `buildScope`) → `grep` toàn bộ FE theo path endpoint để tìm module khác đang tái sử dụng cùng BE endpoint (không chỉ module đang review) — tránh đổi hành vi ngoài phạm vi.

---

## Bước 1 — Phân tích

### Business Logic
- Logic đúng và nhất quán với nghiệp vụ?
- Edge case bị bỏ sót?
- Nguy cơ sai dữ liệu hoặc race condition?
- Trạng thái/transition hợp lệ?

### Frontend (Angular)
- Vi phạm SRP? Logic nên chuyển sang service/store?
- Change Detection: OnPush chưa? Trigger thừa?
- Memory leak: subscription unsubscribe? `takeUntilDestroyed`/async pipe?
- API call thừa hoặc N+1?
- UX: loading/error/empty state đủ?
- i18n: hardcode string trong template/TS? Key mới → search `common.*` trước, có sẵn thì dùng lại, KHÔNG add key trùng (xem `skill/` i18n dedup).

### Backend (Go)
- Check từng mục Hard Rules → Xem `skill/skills/backend-hard-rules/SKILL.md` (đã đọc ở Bước 0)
- Query N+1? Query thừa?
- Transaction an toàn? Rollback đủ?
- Concurrency issue? Goroutine leak?
- Cache phù hợp? Stale data risk?

### Security
- Authorization: đúng tenant scope, đúng role?
- Permission trong `*hasPerm`/guard có đúng domain/resource của component đang review không? Check nhóm quyền trong `permission-matrix.define.ts` (theo `parent`/module id; hằng `PERMISSION_SELECT` vẫn ở `permission.define.ts` — tách 2026-08-28) — pattern lặp lại: copy nhầm permission từ module khác cùng tên hành động (vd `permissionUpdateBroker` thay vì `permissionManageConfig` cho màn dưới `/config`, `permissionConfigDevelop` thay vì `permissionExtPartner`).
- Khi remap permission domain cho nhiều endpoint cùng lúc (vd đổi domain sai sang đúng) — verify TỪNG endpoint theo đúng `*hasPerm` cụ thể mà nút/action đó dùng ở FE, không gộp mọi action mutating vào 1 tier theo giả định (Connect=create khác Revoke/Edit=update). Tự remap ẩu tạo lỗi mới (sự cố thật 2026-07-23, omni-channel).
- Data leakage: response dư field?
- Input validation đủ? Injection risk?

### Maintainability
- Độ dễ đọc · Coupling giữa layer · Khả năng test · Technical debt.

### Performance
- Hotspot: CPU / Memory / Database / Network.

---

## Bước 2 — Trình bày từng vấn đề

```
**[Tên vắn tắt]**
- Layer: FE / BE / DB / Cross-layer
- Severity: Critical / High / Medium / Low
- Vấn đề: [mô tả cụ thể, file:dòng]
- Tác động thực tế: [ảnh hưởng user/data/performance]
- Cách sửa triệt để: [không workaround]
```

---

## Bước 3 — Phân loại action

| Fix ngay được | Cần design/approval | Cần migration DB |
|---|---|---|
| ... | ... | ... |

**Ranh giới review vs fix:**
- Command này là **review-first**: trình bày xong Bước 2–4 rồi **dừng lại chờ user chọn** fix gì. KHÔNG tự apply fix khi chưa được yêu cầu.
- Cột "Cần design/approval" và "Cần migration DB" → **chỉ liệt kê**, tuyệt đối không tự làm — kể cả khi user nói "fix hết", phải confirm lại riêng cho các mục này.
- Severity Critical/High → nêu rõ trong kết luận để user ưu tiên, nhưng vẫn chờ approve trước khi sửa.

---

## Bước 4 — Scoring

| Dimension | Score /10 | Lý do tóm tắt |
|---|---|---|
| Business Logic | | |
| Frontend Architecture | | |
| Backend Architecture | | |
| Performance | | |
| Maintainability | | |

---

## Bước 5 — Sau khi apply fix (chỉ chạy khi user đã approve fix ở Bước 3)

- `go build ./...` pass
- `npx tsc --noEmit` pass
- Không regression ở flow liên quan
- Mỗi thay đổi trace ngược được về 1 vấn đề đã báo ở Bước 2 — không sửa ngoài phạm vi.

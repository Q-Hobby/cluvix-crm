# Cluvix Frontend Skill Index

Web app Angular 22 + PrimeNG v22 (rc.2) + Tailwind + Signals cho ERP Cluvix (BE Go / MySQL multi-tenant — repo riêng `erp-cluvix`). Trả lời tiếng Việt, code tiếng Anh, comment tiếng Việt.

**Quy tắc đọc:** Luôn đọc file này trước. Làm bất kỳ việc gì đụng code → đọc tiếp `skill/skills/frontend-hard-rules/SKILL.md` (sub-index dùng chung với repo cha) rồi file rules/prompts tương ứng.

**Cập nhật skill/:** Xem mục "Cập nhật skill/" cuối file trước khi ghi.

---

## Development Protocol

### 1. Understand First
- Yêu cầu mơ hồ → dừng lại hỏi.
- Nhiều cách hiểu → trình bày options.
- Xác định: module/route nào bị ảnh hưởng, cần endpoint BE mới không, có đụng permission/IndexedDB không.

### 2. Design (thay đổi vừa/lớn)
Bắt buộc cho: feature mới, cần API mới từ BE, cross-module, đổi cấu trúc share/base class.

**Fix nhỏ (1–3 dòng, 1 file) → bỏ qua, sang bước 3.**

- Trình plan ngắn: `1. [Bước] → verify: [check]`
- Chỉ rõ module/component bị ảnh hưởng.
- Chờ duyệt trước khi implement lớn.

### 3. Implement
- Chỉ giải quyết đúng vấn đề được yêu cầu, giải pháp đơn giản nhất, không feature suy đoán.
- Theo pattern sẵn có (BaseList/BaseForm/BaseReport, FormShareImports, share services) — tái dùng trước khi tạo abstraction mới.
- Mọi thay đổi truy ngược được về yêu cầu.

### 4. Validate
- `npx tsc --noEmit` sạch + lint pass. **Lưu ý: tsc pass ≠ ng build pass** (AOT check template) — đổi template nhiều thì build thử (xem `skill/skills/using-primeng-components/reference/fe-patterns-gotchas-build.md`).
- Verify edge case + mục tiêu plan đạt.

### ⚠ Correctness over Speed
Không tối ưu cho xong việc nhanh. Tối ưu cho đúng, maintainable, kiến trúc dài hạn. Chỉ tìm được workaround → nói rõ trước khi làm, tiếp tục tìm root cause.

---

## Hard Rules — LUÔN áp dụng, kể cả fix nhỏ

Digest các rule hay vi phạm nhất — **chi tiết + lý do: `skill/skills/frontend-hard-rules/SKILL.md` (⛔ FE Hard Rules) + `skill/skills/frontend-hard-rules/SKILL.md` (B.1–B.19)**:

- **i18n key mới**: PHẢI `npm run i18n:find -- "<value>"` trước — có rồi thì DÙNG LẠI, chưa có thì theo taxonomy `common.*`/`{module}.*`, add đủ 4 locale (en-US, vi-VN, ja-JP, fil-PH).
- **Footer dialog**: LUÔN `<app-dialog-footer [isSubmitting]="..." />` — KHÔNG tự build footer bằng `p-button` rời.
- **Tra cứu PrimeNG**: dùng MCP server `primeng` — KHÔNG WebFetch/đoán API. Component tên mới (`p-datePicker`, `p-select`...), KHÔNG tên cũ.
- **XSS**: `DOMPurify.sanitize()` trước mọi `bypassSecurityTrustHtml`; mặc định `[innerText]`. Tag mới cho `| safeHtml` phải có trong ALLOWED_TAGS, không thì bị strip câm lặng.
- **OnPush bắt buộc** + signal/computed; KHÔNG mutate object — replace reference. `@for` track theo `item.id`, KHÔNG `$index`.
- **Subscribe cleanup**: `takeUntilDestroyed(this.destroyRef)` — BaseForm/BaseList đã có sẵn `protected destroyRef`.
- **Service mới**: `@Service()` + `inject()` — KHÔNG `constructor(private x: X)`.
- **Confirm dialog**: host global duy nhất là `<p-confirmpopup>` ở app.component (neo theo `target` của event) — KHÔNG khai thêm `p-confirmdialog`/`p-confirmpopup`. Confirm phát từ chỗ KHÔNG có event/target (gate trong `onSubmit`, base class, effect) → `confirmationService.confirm()` không hiện gì → dùng `AppConfirmDialog` qua `dialogWrapperService.open` (khuôn `BaseSignalForm.closeForm()`/`offerAssignSharedRecord()`).
- **Route mới**: gate `authGuard` + permission per-route theo `src/app.routes.ts` (permission hiện enforce chủ yếu ở FE — thiếu gate là lộ menu/trang).
- **Ngày giờ**: convert qua `TimezoneService` (`dateHmsToUnix`/`toTz`) — timestamp gửi BE là unix.
- **Màu sắc/UI mới**: theo `skill/skills/using-primeng-components/reference/ui.md` (Purple `#7B3F9E` primary duy nhất) — KHÔNG tạo màu brand mới.
- **Verify**: `npx tsc --noEmit`. `ng build` ghi đè `public/` git-tracked của repo cha — **`--output-path` KHÔNG cứu được** (CLI clean `outputPath` trong `angular.json` bất kể cờ). Verify AOT bằng `ngc --noEmit`, KHÔNG chạy `ng build`. Đếm lỗi phải dùng `grep -cE "error (TS|NG)"` — lỗi template mang mã `NG8xxx`, `grep "error TS"` luôn cho 0 giả (đo 2026-09-21). Chi tiết: `skill/skills/using-primeng-components/reference/fe-patterns-gotchas-build.md`.

---

## Skills — bảng tra cứu (đọc TRƯỚC khi sửa code)

> Agent có auto-trigger (Claude): skill tự kích hoạt theo `description`.
> **Agent KHÔNG có auto-trigger (Codex, Gemini, Cursor...): bảng này là đường vào duy nhất.**
> Gotcha chi tiết + ví dụ nằm trong `reference/` của từng skill (sâu đúng 1 tầng).

| Khi task chạm... | Đọc TRƯỚC khi sửa |
|---|---|
| **Bất kỳ file nào** trong `src/` — kể cả sửa 1 dòng | `skill/skills/frontend-hard-rules/SKILL.md` |
| Form/dialog, Signal Forms, validate, convert form | `skill/skills/building-angular-signal-forms/SKILL.md` |
| Component PrimeNG, Angular API mới, màu sắc/brand, lỗi build/AOT | `skill/skills/using-primeng-components/SKILL.md` |
| List có filter, detail/report, in ấn, export/import Excel | `skill/skills/building-list-and-export-views/SKILL.md` |
| Auth/permission/guard/tenant FE, i18n key, timezone | `skill/skills/handling-fe-auth-and-i18n/SKILL.md` |
| Base classes, service dùng chung, IndexedDB, service worker, module CRM/EMR/Med | `skill/skills/working-on-fe-modules/SKILL.md` |

Làm việc trong repo cha `erp-cluvix` → skill BE ở `../skill/INDEX.md` (20 skill).

---

## BMAD (AI-Driven Agile cho task lớn)

Xem sub-index: `bmad/INDEX.md` — dùng khi làm feature mới/cần API mới/cross-module, qua vòng đời Analyst→PM→Architect→SM→Dev→QA (slash commands `/bmad-*` trong `.claude/commands/`). Fix nhỏ 1–3 dòng vẫn đi thẳng theo Development Protocol phía trên. Artifact feature đang chạy nằm ở `bmad/<feature-slug>/`; feature có phần BE → phần BE chạy BMAD ở repo cha.

---

## Quan hệ với repo cha (erp-cluvix)

- BE đối ứng: `backend/` (Go/Gin, ~1230 endpoint `/api/*`). Cần endpoint/field mới → phối hợp người giữ repo cha (hoặc làm trong repo cha theo skill của repo cha nếu có quyền).
- Path bắt đầu `backend/`, `../skill/` trong các file skill = path repo cha, chỉ truy cập được khi frontend nằm lồng trong erp-cluvix. Có repo cha → đọc qua `../` (vd `../skill/skills/working-on-ops/reference/export.md`).
- **IndexedDB master-data sync**: phía FE self-contained tại `skill/skills/working-on-fe-modules/reference/fe-indexeddb.md` (sync layers, table-registry, bump INDEXEDDB_DB_VERSION, cache service status filter); phía BE (BumpDexieTable, sentinel, thêm table BE steps) ở repo cha `../skill/skills/handling-auth-and-workspace/reference/indexeddb.md`.
- Làm frontend standalone (không có repo cha): mọi rule cần thiết đã self-contained trong `skill/`; câu hỏi về hành vi API → hỏi người giữ repo cha, KHÔNG đoán response shape.


---

## Cập nhật skill/

Sau session có bài học đáng giữ (gotcha, quyết định kiến trúc, pattern mới):
- Đúng chỗ theo bảng `skill/skills/frontend-hard-rules/SKILL.md` + `skill/skills/working-on-fe-modules/SKILL.md`; kiến thức per-module → `skill/frontend/modules/<module>/`. KHÔNG tạo file mới khi file sẵn có còn đúng chủ đề.
- Ngắn gọn, dạng rule + lý do 1 dòng; không dán log/code dài.
- Sửa file dùng chung (`skill/frontend/*`) → giữ đúng cho cả 2 repo (quy ước path repo cha ở trên).
- INDEX chỉ chứa pointer + digest — nội dung chi tiết nằm trong rules/prompts.

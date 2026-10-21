# How to Update skill/

**KHÔNG tự ý cập nhật `skill/` khi user chưa yêu cầu.**
Chỉ ghi khi user nói rõ: "lưu vào skill", "cập nhật prompt", "thêm vào skill/", hoặc tương đương.

**NHƯNG bắt buộc đề xuất:** cuối session, nếu phát hiện (a) rule trong `skill/` đã sai so với code, (b) mâu thuẫn giữa 2 file, hoặc (c) gotcha/pattern mới đáng lưu → liệt kê ngắn gọn cho user duyệt. KHÔNG im lặng bỏ qua — docs sai còn tệ hơn không có docs.

**⛔ KHÔNG ghi knowledge vào bộ nhớ riêng của agent (memory của Claude Code hay tương đương).** User
chốt 2026-08-23: họ KHÔNG đọc/sửa/kiểm soát được kho đó, nên thứ nằm ở đấy là thứ họ không quản được.
Mọi thứ đáng giữ đi vào ĐÚNG MỘT trong hai chỗ:
- **`skill/`** (hoặc `frontend/skill/`, `mobile/skill/`) — rule/pattern/gotcha tái dùng nhiều nơi.
- **`documents/<topic>/`** — việc cần NGƯỜI làm, phép đo còn treo, spec/khảo sát dài, checklist bàn giao.

Việc còn treo của một epic PHẢI ra `documents/` **trước** khi chuyển `bmad/<slug>/` vào `done/` —
`bmad/` nằm trong `.gitignore`, chuyển đi mà chưa bóc nợ ra là chôn luôn nợ.

---

## Cấu trúc — Agent Skills (2026-08-17, cập nhật bmad/ 2026-08-30)

```
skill/
├── INDEX.md                    ← router: bảng "task chạm gì → đọc skill nào" (= CLAUDE.md/AGENTS.md/GEMINI.md qua symlink)
├── HOW_TO_UPDATE.md            ← file này
├── BMAD-MOVED.md               ← lịch sử 2 lần dời BMAD, xem chi tiết trong file
├── commands/                   ← slash-command prompt (đặc thù Claude Code)
├── hooks/                      ← PreToolUse hook script (đặc thù Claude Code)
├── bmad/                       ← BMAD tooling bền (persona, executor-contract, prompts-playbook, router) — KHÔNG chứa plan/story (plan ở `bmad/` gốc erp-cluvix, ngoài `skill/`, không git theo dõi)
└── skills/
    ├── _tooling/                       ← 5 gate script (xem Step 5)
    ├── backend-hard-rules/
    │   ├── SKILL.md             ← frontmatter name+description, body <500 dòng
    │   └── reference/*.md       ← phẳng đúng 1 tầng, >100 dòng phải có mục lục
    └── <19 skill BE khác>/
```

**3 repo, mỗi repo 1 cây `skill/skills/`** (nested git repo riêng, KHÔNG symlink chéo):
- Backend + cross-cutting: `skill/skills/` (20 skill, repo gốc erp-cluvix)
- Frontend: `frontend/skill/skills/` (6 skill, repo `frontend/`)
- Mobile: `mobile/skill/skills/` (3 skill, repo `mobile/`)

Ghi bài học FE → ghi vào `frontend/skill/skills/`. Ghi bài học mobile → `mobile/skill/skills/`. KHÔNG tạo lại cấu trúc `skill/backend/`, `skill/frontend/`, `skill/mobile/`, `skill/common/`, `skill/mysql/` — cây đó đã bị gỡ hoàn toàn, thay bằng 30 skill ở trên.

**Rule:**
- Mỗi skill = 1 folder = SKILL.md + (tuỳ chọn) `reference/*.md`
- SKILL.md: frontmatter CHỈ có `name` (khớp tên thư mục, kebab-case, ≤64 ký tự) + `description` (nêu cả "làm gì" lẫn "KHI NÀO dùng", ≤1024 ký tự) — **cấm field khác** (vd `allowed-tools`) vì knowledge base phục vụ nhiều agent (Claude/Codex/Gemini), field riêng runtime làm nội dung dính chặt 1 runtime
- `reference/*.md`: phẳng đúng 1 tầng — **cấm** `reference/` trỏ vào `reference/` khác (không lồng cấp 2); file >100 dòng **bắt buộc có mục lục** ở đầu
- Nội dung chỉ đúng cho 1 runtime cụ thể (`/bmad-*`, subagent, hook Claude Code) → đánh dấu rõ ràng là đặc thù runtime đó ngay tại chỗ, không viết như thể mọi agent đều có

---

## Step 1 — Phân loại knowledge → skill nào

| Loại knowledge | Skill đích |
|---|---|
| BE rule bắt buộc (tenant scope, company scope, tiền, response/i18n, timezone, GORM, select field) | `skills/backend-hard-rules/` |
| BE gotcha/pattern khi sửa query/handler đang sai | `skills/debugging-backend-queries/` |
| BE quy trình tạo resource mới (entity/dao/repo/service/handler/router) | `skills/writing-backend-resources/` |
| Kiến trúc/module map/routing/glossary BE | `skills/navigating-backend-architecture/` |
| Test infra Go (unit/integration/httptest) | `skills/testing-backend/` |
| Migration SQL, idempotent ADD/DROP, FK, MySQL 9.7 | `skills/writing-mysql-migrations/` |
| Config module (call-center, digital-signature, print-template, control-field, AI provider) | `skills/working-on-config/` |
| Auth (JWT/2FA/device limit), workspace self-service, IndexedDB sync, AI module | `skills/handling-auth-and-workspace/` |
| `user_option` CRM global option | `skills/managing-crm-options/` |
| Nghiệp vụ CRM (customer/appointment/order/treatment/payment) | `skills/working-on-crm/` |
| Nghiệp vụ HR (user/role/RBAC/OT/payroll/chấm công/hợp đồng) | `skills/working-on-hr/` |
| Kế toán, dashboard CRM, export, import khách hàng/Lead | `skills/working-on-ops/` |
| Deploy production, setup instance, script deploy, cấu hình `.env`/storage instance | `skills/deploying-to-production/` |
| FE rule bắt buộc (XSS, OnPush, subscribe cleanup, HTTP error, token) | `frontend/skill/skills/frontend-hard-rules/` |
| FE Signal Forms (BaseSignalForm, gotcha #1-52) | `frontend/skill/skills/building-angular-signal-forms/` |
| FE list/export/report/print (BaseListV2, BaseReport, BasePrint) | `frontend/skill/skills/building-list-and-export-views/` |
| FE auth signal, permission, timezone, i18n key | `frontend/skill/skills/handling-fe-auth-and-i18n/` |
| PrimeNG v22 + Angular 22 component | `frontend/skill/skills/using-primeng-components/` |
| FE base class (BaseApp/BaseList/BaseForm), shared service, IndexedDB, pattern module CRM/Med/EMR/Config | `frontend/skill/skills/working-on-fe-modules/` |
| Mobile Flutter pattern implementation (`features/`, `packages/core_*`) | `mobile/skill/skills/building-flutter-features/` |
| Mobile kiến trúc/cấu trúc thư mục (melos monorepo) | `mobile/skill/skills/mobile-architecture-and-structure/` |
| Mobile rule bắt buộc (package boundary, contract API v1, auth/tenant, PHI, design foundation) | `mobile/skill/skills/mobile-hard-rules/` |
| Slash-command prompt (review, save-skill...) | `skill/commands/<name>.md` |
| Spec/hợp đồng code tham chiếu (IG, API contract) | cạnh code `backend/internal/modules/webapp/<x>/` (hoặc client/mobile/public tương ứng) — KHÔNG để trong `bmad/<slug>/` (transient) |
| Document tham khảo dài (spec pháp lý, assessment, khảo sát, checklist đối chiếu văn bản, hướng dẫn nghiệp vụ) | `documents/<topic>/` repo chính (gitignored) — nơi DUY NHẤT viết document; KHÔNG để `bmad/<slug>/docs/`, KHÔNG để `Plan/`. **Luôn viết `.html`** (một file tự chứa, mở bằng trình duyệt, xuất bản được thành trang chia sẻ), **KHÔNG viết `.md`** — user chốt 2026-09-02. Tiền lệ: `documents/teamchat/index.html` |

Không chắc giữa 2 skill có ranh giới gần nhau (vd `working-on-crm` vs `working-on-hr`) → đọc phần "Ranh giới với..." trong `description` của SKILL.md liên quan trước khi quyết.

---

## Step 2 — Check đã tồn tại chưa

```bash
grep -rn "<keyword>" skill frontend/skill mobile/skill --include="*.md"
```

Nếu tìm thấy → sang Step 3 (update).
Nếu không → sang Step 4 (thêm mới).

---

## Step 3 — Update (đã tồn tại)

- [ ] Tìm đúng dòng/section cần sửa
- [ ] Check nội dung mới có **mâu thuẫn** với dòng hiện tại không
- [ ] Nếu mâu thuẫn → xác định cái nào đúng, xóa cái sai
- [ ] Update in-place, không duplicate
- [ ] Giữ style ngắn gọn — không thêm giải thích "tại sao"

---

## Step 4 — Thêm mới

- [ ] Chọn đúng skill theo bảng Step 1
- [ ] SKILL.md đích có > 500 dòng body? → **BẮT BUỘC tách vào `reference/<topic>.md` trước** (giữ SKILL.md như mục lục ngắn trỏ vào reference)
- [ ] `reference/*.md` đích có > 500 dòng? → tách theo nhóm topic rõ ràng, thêm mục lục nếu file mới >100 dòng
- [ ] Thêm vào section phù hợp (không tạo section mới nếu có thể ghép)
- [ ] Reference file mới → đảm bảo có được liệt kê/link từ SKILL.md của skill đó (progressive disclosure)

---

## Step 5 — Verify trước khi done

```bash
cd /Users/KemBi/www/erp-cluvix
skill/skills/_tooling/verify-links.sh
skill/skills/_tooling/verify-paths.sh
skill/skills/_tooling/verify-frontmatter.sh
skill/skills/_tooling/verify-depth.sh
skill/skills/_tooling/verify-no-dup.sh
skill/skills/_tooling/verify-symbols.sh
skill/skills/_tooling/verify-orphan.sh
```

- [ ] Grep lại keyword — chỉ xuất hiện **1 lần** trong toàn bộ knowledge base (`skill/` + `frontend/skill/` + `mobile/skill/`)
- [ ] Đọc lại đoạn vừa thêm — có câu giải thích "tại sao" nào không? Xóa đi.
- [ ] Nội dung có nhất quán với các rule liên quan không?
- [ ] SKILL.md sau khi edit có dài hơn **500 dòng body** không? → tách ngay sang `reference/` (không để sang session sau)
- [ ] `reference/*.md` sau khi edit >100 dòng mà chưa có mục lục → thêm mục lục
- [ ] Cả 7 gate ở trên phải exit 0 — trừ `verify-no-dup.sh` được phép đỏ đúng nhóm bmad-plan.md (FE+mobile, nợ đã biết); **finding MỚI ở bất kỳ gate nào thì KHÔNG được bỏ qua**, phải xử trước khi báo xong

---

## Quy tắc viết

- **Chỉ viết rule/action** — không viết lý do, không viết context lịch sử
- **Bullet ngắn** — tối đa 1 dòng mỗi rule
- **Code block** — giữ nguyên, không rút gọn
- **CRITICAL / HIGH** — giữ label nếu rule quan trọng
- **Không tạo file mới** nếu nội dung < 20 dòng — ghép vào file hiện có
- **Trung lập agent** — không viết rule như chỉ Claude đọc được; phần đặc thù Claude Code (`/bmad-*`, subagent, hook) phải gắn nhãn rõ ràng ngay tại chỗ

---

## Ghi mô tả file / function

Sau khi làm xong 1 file/func phức tạp → thêm vào `reference/` của skill tương ứng (không tạo file codemap riêng).

**Format:**

```markdown
## Key Files

| File | Mục đích | Gọi bởi |
|------|----------|---------|
| `appointment-duplicate-dialog/` | Check + hiển thị lịch hẹn trùng | `appointment-form.component` |

### Functions đáng chú ý

`checkDuplicate(params)` — query BE `/appointment/check-duplicate`, block submit nếu có conflict.
```

**Rules:**
- Chỉ ghi file/func **không self-explanatory** từ tên
- 1 dòng mục đích + 1 dòng caller/trigger — không ghi implementation detail

---

## Khi nào tạo file `reference/` mới

Tạo file mới khi **cả 3** điều kiện sau đúng:
1. Nội dung > 30 dòng
2. Không thuộc về file nào hiện có
3. Sẽ được đọc độc lập (không phải luôn đọc kèm file khác)

Sau khi tạo → đảm bảo SKILL.md của skill đó trỏ tới file mới (progressive disclosure — agent đọc SKILL.md trước, chỉ mở `reference/` khi cần).

---

## Khi nào chạy `/skill-optimize`

Chạy soát toàn bộ knowledge base (30 skill, cả 3 repo) — phát hiện trùng lặp, link chết, drift so với code, ngưỡng dòng vượt — sau:
- Một đợt BMAD lớn (epic nhiều story) vừa xong
- Restructure module/folder ảnh hưởng path tham chiếu trong skill
- Định kỳ (gợi ý: mỗi vài tháng hoặc khi `verify-no-dup.sh`/`verify-paths.sh` bắt đầu tích luỹ finding)

> Lệnh `/skill-optimize` sẽ được tạo ở story-24 của epic `skill-optimization` — nếu chưa tồn tại khi cần, chạy tay 5 gate ở Step 5 trên toàn bộ 3 cây skill thay thế.

# Check-up Psychiatry (Khám sàng lọc tâm thần)

> ⚠ **(đã bỏ, D15/CRM standalone)** File này mô tả module `internal/modules/client/customer-b/` — verify
> 2026-09-28: `internal/modules/client/` KHÔNG tồn tại trong repo CRM standalone này (chỉ còn
> `internal/modules/webapp/`). Mọi mention `company_id`/`companyActiveId`/`GetCompanyActiveID` bên
> dưới cũng đã bỏ cùng company (D15). Giữ nguyên làm lịch sử/tham khảo, KHÔNG áp dụng.

## Mục lục

- Mục tiêu nghiệp vụ
- 6 bảng DB (`psych_quiz_*` prefix — cùng nhóm khi `SHOW TABLES`)
- Constants — `pkg/define/psych_quiz.go`
- Routes admin (JWT auth)
- Routes client (PartnerAuth bắt buộc — 2026-07-08)
- Tenant scope
- Scoring engine (submit.go)
- Token + expiry
- ZNS
- Preload với ORDER BY (bắt buộc cho sort_order)
- FE — cấu trúc
- Gotchas

---


**Trạng thái: ✅ Production**
Module: `internal/modules/client/customer-b/handler/` (client, module khách `customer-b`, PartnerAuth — `quiz.go` theo token, `quiz_list.go`/`quiz_session.go`/`quiz_history.go` tự phục vụ)

---

## Mục tiêu nghiệp vụ

Admin tạo bài trắc nghiệm tâm thần (dynamic, nhiều câu hỏi + đáp án + thang điểm).
Tạo session cho từng KH → KH làm bài qua Web/ZMA/QR → hệ thống tự tính điểm → BS xem kết quả trong HIS.

---

## 6 bảng DB (`psych_quiz_*` prefix — cùng nhóm khi `SHOW TABLES`)

| Bảng | Mục đích | Có `company_id` |
|------|----------|----------------|
| `psych_quiz` | Template quiz (admin tạo) | ✅ `company_id` |
| `psych_quiz_question` | Câu hỏi (FK: quiz_id) | ❌ |
| `psych_quiz_option` | Đáp án single-choice (FK: question_id) | ❌ |
| `psych_quiz_session` | 1 lần KH làm bài, có UUID token | ❌ (scope qua customer) |
| `psych_quiz_session_answer` | Câu trả lời (FK: session_id + question_id) | ❌ |
| `psych_quiz_type` | Loại trắc nghiệm (danh mục, admin CRUD trong Config) | ✅ `company_id` |

`psych_quiz_session` KHÔNG có `company_id` — scope qua customer subquery (giống dental).
`psych_quiz.quiz_type_id` — FK nullable tới `psych_quiz_type.id`, `ON DELETE SET NULL` (xoá loại không kéo theo xoá quiz, chỉ mất phân loại).

---

## Constants — `pkg/define/psych_quiz.go`

```go
PsychQuizStatusDraft  = 1  // Nháp
PsychQuizStatusActive = 2  // Đang dùng

PsychQuestionTypeSingle = 1  // Chọn 1 đáp án
PsychQuestionTypeText   = 2  // Tự luận (score = 0)

PsychSessionPending   = 1  // Chờ làm bài
PsychSessionCompleted = 2  // Đã nộp
PsychSessionExpired   = 3  // Hết hạn

PsychChannelInPerson = 1  // Trực tiếp
PsychChannelZMA      = 2  // Zalo Mini App
PsychChannelWeb      = 3  // Web browser
```

---

## Routes admin (JWT auth)

```
GET    /crm/psych-quiz           → ListQuiz
GET    /crm/psych-quiz/:id       → GetQuiz (có preload Questions.Options ORDER BY sort_order)
POST   /crm/psych-quiz           → CreateQuiz
PUT    /crm/psych-quiz/:id       → UpdateQuiz (diff-by-id: update tại chỗ, insert mới, xoá cái bị bỏ — KHÔNG delete-all-recreate)
DELETE /crm/psych-quiz/:id       → DeleteQuiz

GET    /crm/psych-session        → ListSession
GET    /crm/psych-session/:id    → GetSession (có answers JOIN)
POST   /crm/psych-session        → CreateSession (sinh UUID token, set expires_at)
PUT    /crm/psych-session/:id/note  → UpdateSessionNote
DELETE /crm/psych-session/:id    → DeleteSession (chỉ khi status=pending)
POST   /crm/psych-session/:id/send-zns  → SendSessionZNS
```

## Routes client (PartnerAuth bắt buộc — 2026-07-08)

```
GET    /client/customer-b/quiz              → ListQuiz (list quiz active, tự phục vụ — 2026-07-10; filter tuỳ chọn ?quiz_type_id=<id> — 2026-07-11)
GET    /client/customer-b/quiz/history      → ListHistory (lịch sử session theo customer — 2026-07-10)
GET    /client/customer-b/quiz/:token       → GetQuiz (load quiz + questions + options, không trả answers; nếu completed có thêm max_score — 2026-07-11)
POST   /client/customer-b/quiz/:token       → SubmitQuiz (lưu answers, tính điểm, cập nhật session; response có thêm max_score — 2026-07-11)
POST   /client/customer-b/quiz/:token/session → CreateSession (khách tự tạo session theo quiz_id, idempotent qua GET_LOCK — 2026-07-10; param path mượn tên "token", giá trị thực là quiz_id, xem `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md` §Gin wildcard)
GET    /client/customer-b/quiz-type         → ListQuizType (danh mục loại có ≥1 quiz active — 2026-07-11; route RIÊNG ngoài /client/customer-b/quiz, tránh wildcard /quiz/:token nuốt mất)
```

`max_score` = Σ điểm cao nhất mỗi câu single-choice trong quiz (câu tự luận = 0) — không có ở nhánh `pending` của GetQuiz, không có ở `/quiz/history`.

Đăng ký tại: `internal/modules/client/customer-b/route.go` (module khách `customer-b`, đăng ký thẳng trong `client/route.go`; group `/client/customer-b/quiz` + `/client/customer-b/quiz-type`, token `/client/customer-b/oauth/token`). Handler: `internal/modules/client/customer-b/handler/`.

> **Đã dời từ `internal/modules/public/quiz/` (no-auth) sang đây theo yêu cầu tái sử dụng logic PartnerAuth.** Module `internal/modules/public/` cũ đã bị xóa hoàn toàn. Đây là breaking change có chủ đích — trang public mở thẳng bằng trình duyệt không đăng nhập không còn gọi được API này nữa, phải qua BFF của đối tác giữ Bearer token hộ. Quiz được scope theo `company_id` của partner (`psych_quiz.company_id = companyActiveId`), token của company khác trả 404 giống token không tồn tại.

---

## Tenant scope

```go
// Quiz → có company_id → dùng quizCompanyScope
func (h *Handler) quizCompanyScope(c *gin.Context) *gorm.DB {
    companyID := middleware.GetCompanyActiveID(c)
    return h.DB.Table("psych_quiz").
        Where("psych_quiz.company_id = ? OR psych_quiz.company_id IS NULL", companyID)
}

// Session → scope qua customer subquery (KHÔNG có company_id trên session)
func (h *Handler) sessionCustomerScope(c *gin.Context, q *gorm.DB) *gorm.DB {
    sub := tenant.ScopedDBNullBranch(c, h.DB, "customer").Table("customer").Select("id")
    return q.Where("psych_quiz_session.customer_id IN (?)", sub)
}
```

---

## Scoring engine (submit.go)

```go
// 1. Load option scores (IN optionIDs)
// 2. Sum scores từ body.Answers
// 3. Unmarshal quiz.Thresholds (JSON) → find bucket: score >= t.Min && (t.Max == nil || score <= *t.Max)
// 4. Transaction: INSERT session_answers + UPDATE session (status, total_score, threshold_label, is_normal)
```

`thresholds` lưu dạng JSON trong `psych_quiz.thresholds` — `max: null` = không giới hạn trên:
```json
[{"min":0,"max":100,"label":"Bình thường","is_normal":1},
 {"min":101,"max":null,"label":"Bất thường","is_normal":0}]
```

Validate bắt buộc (FE + BE độc lập, xem `quiz_upsert.go` `validateThresholds` — 2026-07-11): mức thấp nhất `min=0`, các mức nối liền nhau không hở khoảng/không chồng lấn, mức cao nhất PHẢI `max=null` — đảm bảo mọi điểm số đạt được luôn khớp đúng 1 mức, không rơi vào "Không xác định".

---

## Token + expiry

- Token: UUID v4, sinh tại `CreateSession`
- `expires_at`: BIGINT Unix — FE gửi `expires_days`, **FE service convert** → `expires_at = now/1000 + days*86400`
- Public GET: nếu `now > expires_at` → auto-update status=3 + trả 410 Gone
- Public GET: nếu status=completed → trả 200 với kết quả cũ (KH có thể xem lại)
- Public POST: nếu status≠pending → trả 409 Conflict

---

## ZNS

- Trigger key: `psych_quiz.session_created`
- ERP field `psych_quiz.token_url` = full URL: `APP_URL/quiz/<token>` (env `APP_URL`, fallback `https://app.example.com`)
- Route: `POST /crm/psych-session/:id/send-zns` body `{trigger_key?}` (default: session_created)

---

## Preload với ORDER BY (bắt buộc cho sort_order)

```go
h.DB.Preload("Questions", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).
    Preload("Questions.Options", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC") }).
    Where("id = ?", id).First(&result)
```

---

## FE — cấu trúc

```
core/models/crm/psych-quiz.model.ts          ← PsychQuizModel, PsychSessionModel, PsychThreshold
core/models/crm/psych-quiz-type.model.ts     ← PsychQuizTypeModel
core/services/crm/psych-quiz.service.ts      ← BaseApiService<PsychQuizModel> + session endpoints
core/services/crm/psych-quiz-type.service.ts ← BaseApiService<PsychQuizTypeModel>
core/services/crm/psych-session.service.ts   ← BaseApiService<PsychSessionModel>, có listByCustomer()

modules/config/config-derpartment/psych-quiz/     ← quiz template + loại trắc nghiệm (admin CRUD, route /config/psych-quiz)
  list/psych-quiz-list.ts/.html                    ← quiz list
  form/psych-quiz-form.ts/.html                    ← quiz builder (nested FormArray: questions → options + thresholds)
  quiz-type/list/psych-quiz-type-list.ts/.html      ← danh mục loại trắc nghiệm
  quiz-type/form/psych-quiz-type-form.ts/.html
  psych-quiz.routes.ts                              ← '' → list, 'quiz-type' → quiz-type list

modules/crm/psych-quiz/session/       ← session (dùng chung, KHÔNG còn trang riêng trong Config)
  form/psych-session-form.ts/.html    ← tạo session; LUÔN tự chọn khách hàng qua autocomplete (đã bỏ chế độ khoá field — 2026-07-11)
  detail/psych-session-detail.ts/.html ← xem kết quả, ghi chú BS, QR, in
  list/customer-detail/psych-session-of-customer.ts/.html  ← tab "Thăm khám" trong customer-detail, chỉ hiện khi department_type = PSYCHIATRY, READ-ONLY (chỉ xem, không tạo — 2026-07-11)
  list/psych-session-list/psych-session-list.ts/.html  ← danh sách TOÀN BỘ session mọi khách hàng, route /crm/psych-quiz/sessions, có tạo/filter khách hàng-quiz-status (2026-07-11)
```

> FE public quiz page (`modules/public/psych-quiz/`) đã xóa 2026-07-10.

---

## Gotchas

- `expires_days` vs `expires_at`: FE service `createSession()` tự convert — component không cần biết
- `qr-code` selector: dùng `QrCodeComponent` từ `ng-qrcode` — PHẢI import vào `imports[]` của component
- Session không có `company_id` → khi admin xem session phải scope qua customer, không qua company trực tiếp
- Con reference cha bằng ID nhưng không có FK (vd `session_answer.question_id/option_id` không FK tới `psych_quiz_question/option`) → sửa cha KHÔNG được delete-all-rồi-tạo-lại (đổi ID vô cớ làm đứt tham chiếu lịch sử của các session đã hoàn thành). Dùng diff-by-id (update tại chỗ/insert mới/xoá có chọn lọc) + snapshot text (`question_text`/`option_text`) tại thời điểm ghi answer, không JOIN sống vào bảng cha.

# Gotchas Data Integrity

## Mục lục

- source_id trong financial metrics — dùng `osu.source_id`, KHÔNG `customer.source_id` (CRITICAL)
- PATTERN: Masked credentials — secret field không lộ ra ngoài
- `codegen.Generate()` — bắt buộc truyền `&companyIDUint`
- SeedConfigCodeForBranch — chạy lại trên MỌI branch Create/Update, không chỉ Create (CRITICAL)
- Bảng con `order_service_used`/`order_treatment_plan` — không có FK constraint, cleanup thủ công (HIGH)
- Trong 1 transaction — KHÔNG query phụ trợ qua connection khác (CRITICAL)
- `LAST_INSERT_ID()` — PHẢI cùng kết nối với lệnh INSERT (CRITICAL)
- `INSERT … ON DUPLICATE KEY UPDATE` trúng dòng đã có — khoá supremum, chặn INSERT toàn bảng (CRITICAL)
- Lock order NHẤT QUÁN cha→con trong state machine đa-actor (CRITICAL)
- Đọc field decimal từ `map[string]interface{}` — dùng `ToDecimal`, không type-assert cứng
- Operator phủ định trên cột nullable — PHẢI OR IS NULL (CRITICAL)
- Cắt chuỗi trước khi ghi cột MySQL — theo BIÊN RUNE, đếm KÝ TỰ (CRITICAL)

---


Transaction, lock, FK, decimal, credential, quy tắc domain. Tách từ `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md`.
Hợp đồng handler → `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md`.

## source_id trong financial metrics — dùng `osu.source_id`, KHÔNG `customer.source_id` (CRITICAL)

Mọi subquery tính financial metrics (totalOrderPlan, totalRevenue, totalPayment, totalRefundServiceUsed) **phải** GROUP BY và filter theo `osu.source_id`, không phải `customer.source_id`.

```sql
-- ❌ SAI — source từ customer, không phải từ dịch vụ
SELECT customer.source_id, SUM(osu.totalEnd) AS totalOrderPlan
FROM order_service_used osu
LEFT JOIN customer ON customer.id = osu.customer_id
WHERE customer.source_id IS NOT NULL ...
GROUP BY customer.source_id

-- ✅ ĐÚNG
SELECT osu.source_id, SUM(osu.totalEnd) AS totalOrderPlan
FROM order_service_used osu
LEFT JOIN customer ON customer.id = osu.customer_id  -- giữ join nếu cần scope filter
WHERE osu.source_id IS NOT NULL ...
GROUP BY osu.source_id
```

**Ngoại lệ — giữ `customer.source_id`:** deposit (`payment_for = PaymentForDeposit`) và refund deposit (`PaymentForRefundDeposit`) vì không có osu link.

**Payment joins osu:** subquery totalPayment/totalRefundServiceUsed từ `payment` phải thêm `INNER JOIN order_service_used osu ON osu.id = p.serviceUsed_id` khi đổi sang osu.source_id.

**Files đã fix (2026-06-10):**
- `crm/report/handler/crm.go` — `dimensionStatistics` (param `useOsuFinancial=true` cho SourceStatistics), `GroupSourceStatistics`
- `accountant/report/handler/report_payment_by_source.go`
- `accountant/report/handler/report_payment_by_group_source.go`

---

## PATTERN: Masked credentials — secret field không lộ ra ngoài

Dùng khi field nhạy cảm (API key, password) lưu DB nhưng GET response phải che.

```go
// 1. GET: mask trước khi trả
func maskSecret(field, value string) string {
    if !secretFields[field] || value == "" { return value }
    visible := min(8, len(value))
    return value[:visible] + strings.Repeat("*", len(value)-visible)
}

// 2. UPDATE: nếu FE gửi lại masked → giữ nguyên, không overwrite
func isMaskedVersion(field, input, original string) bool {
    return secretFields[field] && input == maskSecret(field, original)
}
// Trong upsert loop:
if isMaskedVersion(k, incoming[k], existing[k]) {
    merged[k] = existing[k]
} else {
    merged[k] = incoming[k]
}
```

**FE không cần xử lý gì** — load masked value vào form, submit as-is, BE tự detect.

Dùng trong: `config/digital_signature` (`credentials_enc`), `config/ai_config` (`api_key`).

**KHÔNG dùng `has_credentials` làm điều kiện bỏ qua field** — phải check `isMaskedVersion` per field.

---

## `codegen.Generate()` — bắt buộc truyền `&companyIDUint`

`nil` cho companyID → code không có tenant scope → trùng code giữa các company.

```go
// ❌ NEVER
usage.Code = codegen.Generate(tx, "stock_usages", nil, nil)

// ✅ ALWAYS
companyIDUint := uint(usage.CompanyId)
usage.Code = codegen.Generate(tx, "stock_usages", &companyIDUint, nil)
```

- `Generate` trả `""` cả khi chưa seed config lẫn khi bộ đếm lỗi. Caller không có nhánh xử lý `""` ⇒ dùng `codegen.GenerateErr` và trả lỗi (rollback); `GenerateErr` chỉ báo lỗi bộ đếm, chưa seed config vẫn là `("", nil)`.


→ Xem `skills/writing-mysql-migrations/SKILL.md` § "UNIQUE KEY + NULL column"

---

## SeedConfigCodeForBranch — chạy lại trên MỌI branch Create/Update, không chỉ Create (CRITICAL)

`backend/internal/modules/webapp/config/branch/handler/handler.go` gọi hàm này ở CUỐI cả `CreateBranch`
lẫn `UpdateBranch` (sửa tên/địa chỉ/ảnh... cũng trigger), tự INSERT row `config_code` thiếu cho
13 bảng codegen theo cặp `(company_id, branch_id)` — dùng DB default (`include_branch_code=1,
monthly_reset=1, serial_length=2`), không biết config company-scoped đã tuỳ biến riêng.

Bảng nào cần company-scoped (không branch-scoped, vd `service`) PHẢI check tồn tại theo
`company_id` (bỏ điều kiện `branch_id`) trong hàm này + set tường minh field tuỳ biến khi insert
— nếu không, edit 1 branch bất kỳ sẽ tái tạo row branch-scoped mặc định, ghi đè hiệu lực config
company-scoped ngay lập tức.

---

## Bảng con `order_service_used`/`order_treatment_plan` — không có FK constraint, cleanup thủ công (HIGH)

`order_commission_*`, `broker_commission`, `order_revenue_*` v.v. KHÔNG có `FOREIGN KEY` trong DB — mọi cleanup khi xóa parent phải tự viết `DELETE` tường minh, không tự động cascade.

Khi thêm bảng con mới kiểu này, phải cover đủ:
- `order_treatment_plan.go` `Delete()` — nhánh theo `order_service_used_id IN (svcIDs)` VÀ nhánh theo `order_treatment_plan_id`
- `order_treatment_plan.go` `handleServices()` — orphan-service branch (`deletableIDs`) khi xóa 1 dịch vụ lẻ trong đơn (không xóa cả đơn)
- Nếu bảng con key theo `payment_id` (derived per-payment, vd `order_commission_*`/`order_revenue_*`) → thêm cleanup ở `payment_crud.go` `deletePayment()`

Thiếu 1 trong các điểm trên → orphan data khi xóa đơn/dịch vụ/payment.

## Trong 1 transaction — KHÔNG query phụ trợ qua connection khác (CRITICAL)

Trong `tx := db.Begin()`/`db.Transaction(...)`, KHÔNG gọi query "chọn ứng viên"/lookup phụ qua `s.DB` (connection khác pool) rồi dùng kết quả tiếp trong cùng tx — pool nhỏ/bận khiến tx đang giữ 1 connection chờ connection thứ 2 từ CÙNG pool → deadlock/pool-exhaustion (verified `SetMaxOpenConns(1)` treo thật, xem §Testing "GET_LOCK không test được qua Suite" — cùng họ triệu chứng, khác nguyên nhân). Tách bước "chọn"/lookup ra NGOÀI transaction; tx chỉ giữ lock + re-check + update. Precedent: `omnichat-assignment` push-assign-hook.

**Lý do THỨ HAI, độc lập với pool — ảnh chụp REPEATABLE READ.** MySQL chốt ảnh chụp của tx ở lần đọc **nhất quán (non-locking) ĐẦU TIÊN** — không phải ở `BEGIN`, và không phải ở lần đọc có khoá. Đặt một `SELECT` thường (guard tồn-tại/authz) làm câu đầu trong tx ⇒ ảnh chụp chốt TRƯỚC khi giành khoá ⇒ tx sau xếp hàng đúng như thiết kế nhưng khi tới lượt lại đọc bảng bằng ảnh CŨ, không thấy dòng tx trước vừa commit ⇒ cưỡng chế trần/quota sai, **im lặng**: khoá vẫn chạy, chỉ là không còn bảo vệ được gì. Câu ĐẦU TIÊN trong tx phải là chính lệnh `SELECT ... FOR UPDATE`. Đo thật 2026-09-04: 8/8 lượt ghim đồng thời lọt qua trần 5 khi guard nằm trong tx. Precedent: `teamchat/service/pin.go`, `omnichat/service/pin.go`.

**Ngoại lệ hợp lệ duy nhất đã chốt — bộ đếm phải tách khỏi tx** (không được giữ khoá tới commit), khuôn
`(*CustomerCoder).nextSerial` + `upsertSerialOwnConn` (`pkg/codegen/customer_code.go`). Bắt buộc đủ 5 lưới: (1) pool có trần và đã kín
(`sqlDB.Stats().InUse >= MaxOpenConnections`) ⇒ KHÔNG xin thêm; (2) chờ lấy kết nối có hạn
(`context.WithTimeout` quanh `sqlDB.Conn`); (3) mọi lỗi ⇒ lùi về làm trong tx; (4) chờ KHOÁ có hạn trên
kết nối riêng: `SET SESSION innodb_lock_wait_timeout = N`, trả về `DEFAULT` trước khi `Close` — trả không
được ⇒ huỷ kết nối (`conn.Raw` trả `driver.ErrBadConn`); (5) đã lùi về tx 1 lần thì các lượt sau cùng
request đếm luôn trong tx (tx đang giữ khoá dòng đếm — mở kết nối riêng là chờ chính mình). Row cha của FK mà kết nối
riêng ghi vào PHẢI đã COMMIT — nếu do chính tx caller vừa tạo, kết nối riêng chờ khoá của chính caller.

## `LAST_INSERT_ID()` — PHẢI cùng kết nối với lệnh INSERT (CRITICAL)

`LAST_INSERT_ID()` tính theo từng kết nối. INSERT/UPSERT và `SELECT LAST_INSERT_ID()` phải chạy trên CÙNG
1 kết nối: trong `tx`, trong `db.Transaction(...)` (đang trong tx thì là SAVEPOINT), hoặc cùng một
`*sql.Conn`. Hai lệnh GORM rời trên `h.DB`/`s.DB` có thể lấy 2 kết nối khác nhau trong pool ⇒ đọc ra 0
hoặc id của request khác — chỉ lộ khi có tải đồng thời. Khuôn: `nextSerial` (`pkg/codegen/codegen.go`).

## `INSERT … ON DUPLICATE KEY UPDATE` trúng dòng đã có — khoá supremum, chặn INSERT toàn bảng (CRITICAL)

- Trúng dòng **đã có**, InnoDB vẫn giữ khoá X trên supremum của PRIMARY (thử chèn id auto-increment mới trước) tới hết tx ⇒ chặn MỌI INSERT khác vào bảng, kể cả dòng không liên quan.
- `INSERT IGNORE` trúng trùng giữ khoá y hệt — đổi sang nó không được gì.
- Bộ đếm/upsert nóng trong tx: `UPDATE … WHERE <unique key>` trước, `RowsAffected = 0` mới INSERT. Khuôn: `bumpSerial` (`pkg/codegen/codegen.go`).
- Kiểm chứng: `performance_schema.data_locks`, tìm `LOCK_DATA = supremum pseudo-record` đang `GRANTED` cho tx giữ và `WAITING` cho tx bị chặn.

## Lock order NHẤT QUÁN cha→con trong state machine đa-actor (CRITICAL)

Nhiều actor cùng lúc thao tác trong transaction khoá ≥2 row liên quan (aggregate cha + row con của nó): PHẢI khoá theo THỨ TỰ NHẤT QUÁN cha→con — `SELECT ... FOR UPDATE` row **cha (aggregate) TRƯỚC**, rồi mới CAS/update row con. Khoá con trước rồi mới cha → 2 actor thao tác 2 con anh-em (vd 2 người duyệt 2 task cùng 1 step) tạo chu trình chờ chéo → **MySQL 1213 deadlock**. Lock cha đầu serialize toàn bộ advance: actor thứ 2 chờ tới khi actor 1 commit, đọc lại state, CAS con của mình trượt (đã bị đóng do quorum) → báo "việc đã xử lý". Precedent: `approval` instance→task (`FOR UPDATE approval_instance` trước CAS `approval_task`).

## Đọc field decimal từ `map[string]interface{}` — dùng `ToDecimal`, không type-assert cứng

- Sau `ParseDecimalFieldsForTables`, field vẫn có thể KHÔNG phải `decimal.Decimal` nếu cache `decimalColumnMap` chưa init (race lúc bootstrap/cache stale) — assertion `v["field"].(decimal.Decimal)` fail âm thầm (`ok=false`), giá trị bị coi như thiếu/0.
- Dùng `helpers.ToDecimal(v["field"])` thay vì assertion trực tiếp — handle mọi kiểu input (kể cả raw bytes), không silent-wrong. Đã cắn: `CreatePaymentOrderFromPayment` tính tổng tiền đơn.

## Operator phủ định trên cột nullable — PHẢI OR IS NULL (CRITICAL)

- SQL three-valued logic: `col != ?` / `NOT IN` / `NOT LIKE` trên cột nullable âm thầm loại record NULL khỏi kết quả "khác X".
- Raw SQL: `(x.deposit_type IS NULL OR x.deposit_type NOT IN (...))` — guard nằm TRONG cùng cặp ngoặc với NOT IN.
- `pkg/query.ApplyFilter` operator `neq` ĐÃ tự guard `(col != ? OR col IS NULL)` (2026-07-24, toàn cục) — thêm operator phủ định mới (`nin`/`not_like`) vào filter builder PHẢI giữ guard này.
- Verify sau sweep: `grep -rn "NOT IN" backend/internal/ | grep -v "IS NULL"` → 0 kết quả với cột nullable.
- Luật "bọc ngoặc mệnh đề OR" áp cho **`Raw`/`Exec`**, KHÔNG phải `Where` của builder: GORM tự bọc một `Where` chứa OR khi nối bằng AND (đo bằng `db.ToSQL`, 2026-09-01).
- Hệ quả: mutation "bỏ ngoặc trong `Where`" **không thể đỏ** — nó là no-op ngữ nghĩa. Đừng viết test cho nó rồi tưởng có lưới; và đừng mô tả nó như lỗi thật trong comment.
- Nghi ngờ SQL sinh ra khác điều mình nghĩ ⇒ `db.ToSQL(func(tx *gorm.DB) *gorm.DB { ... })` in ra câu thật, rẻ hơn suy luận.

## Cắt chuỗi trước khi ghi cột MySQL — theo BIÊN RUNE, đếm KÝ TỰ (CRITICAL)

- `VARCHAR(n)` của MySQL đếm **n ký tự** (utf8mb4), KHÔNG phải n byte.
- `s[:n]` cắt theo byte → tiếng Việt/emoji bị cắt giữa rune → chuỗi UTF-8 hỏng → MySQL trả **1366 Incorrect string value** và vứt **NGUYÊN dòng**.
- Cắt bằng `for i := range s` (i luôn là biên rune) + đếm rune; đo bằng `utf8.RuneCountInString`, kiểm bằng `utf8.ValidString`.
- Test phải dùng ký tự **3–4 byte** (`ầ`, `🙂`) với độ dài **lẻ** — ký tự 2 byte + ngưỡng chẵn cho test xanh giả vì cắt vô tình rơi đúng biên.

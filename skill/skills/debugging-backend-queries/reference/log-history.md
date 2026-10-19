# Audit Trail — Log History

## Mục lục

- Tổng quan
- Schema — 5 bảng đồng nhất
- API — `history.LogChanges()`
- Tracked fields (từ `config.go`)
  - log_customer
  - log_appointment
  - log_payment
  - log_treatment
  - log_order_service_used
- Pattern xóa parent — BẮT BUỘC xóa log trước
- Read API — `GET /history/log?_fieldName=<type>`
- Thêm log table mới — checklist

---


## Tổng quan

5 bảng `log_*` ghi lại mọi thay đổi (update) trên các entity CRM quan trọng. Tất cả đi qua một hàm duy nhất `history.LogChanges()`.

**`company_id` snapshot lúc ghi log** (2026-09-22) trên `log_customer`, `log_appointment`, `log_payment`, `log_treatment`, `log_order_service_used` (+ `log_partners` từ trước) — đã đăng ký `tenantTables`, đọc bằng `ScopedDBNullBranch(c, q, "<bảng>", "<alias>")`. KHÔNG scope qua JOIN bảng cha: dòng log xoá có fk NULL ⇒ JOIN rỗng. Ghi log **tự điền**: `history.insertLogRow` tra `company_id` của bản ghi gốc theo `logEntityTables` khi caller không truyền `extra` — thêm bảng log mới có company_id thì PHẢI thêm vào map đó (`log_partners` tự truyền).

⚠ **KHÔNG tạo index `(company_id)` trên các bảng log này** (đã gỡ ở `20260922133410`) — lý do + số đo: `perf-list-endpoints.md` §"Đừng bỏ chỉ mục một cột `(company_id)`", mục Ngoại lệ NullBranch.

---

## Schema — 5 bảng đồng nhất

| Table | FK column | FK table | ON DELETE |
|---|---|---|---|
| `log_customer` | `customer_id` | `customer` | **SET NULL** (2026-09-22) |
| `log_appointment` | `appointment_id` | `appointment` | **SET NULL** (2026-09-22) |
| `log_payment` | `payment_id` | `payment` | **SET NULL** |
| `log_treatment` | `treatment_id` | `treatment_course` | **SET NULL** (2026-09-22) |
| `log_order_service_used` | `order_service_used_id` (nullable từ 2026-09-22) | `order_service_used` | **SET NULL** (2026-09-22) |

Cột chung mỗi bảng:

```sql
id           INT PK AUTO_INCREMENT
<fk_column>  INT FK → parent (ON DELETE RESTRICT, ON UPDATE CASCADE)
action_type  TINYINT  -- 1=create, 2=update, 3=delete
changed_fields JSON   -- diff {field: {from, to}}
changed_by   INT      -- user.id
changed_at   INT      -- unix timestamp
```

**Hệ quả RESTRICT:** Trước khi xóa parent, PHẢI xóa log trước (xem phần Delete bên dưới).
**SET NULL** (5 bảng CRM trên + `log_partners`, `log_user`): gọi `history.LogDeleteSnapshot(tx, "<log>", id, history.<X>DeleteFields, userID)` TRƯỚC khi xoá parent, cùng tx → dòng action_type=3 sống sót với fk NULL. `<X>DeleteFields` = `id`, `code`, `customer_id` + field theo dõi (`withDeleteIdentity`) — màn lịch sử đọc lại cột ID (`COALESCE(fk, snapshot id)`) và khách (`COALESCE(entity.customer_id, snapshot customer_id)`) từ đó; FE hiện "Dữ liệu đã bị xóa" thay vì liệt kê `giá trị → null`.

---

## API — `history.LogChanges()`

**Import:** `history "erpCluvix/internal/modules/webapp/history/log/service"`

```go
history.LogChanges(
    db,              // *gorm.DB — dùng tx nếu trong transaction
    "log_customer",  // table name (whitelist: 5 bảng trên)
    "customer_id",   // fkColumn — phải khớp với whitelist
    entityID,        // int — ID của parent record
    oldData,         // map[string]interface{} — snapshot TRƯỚC update
    newData,         // map[string]interface{} — snapshot SAU update (hoặc filtered map)
    includeFields,   // []string — chỉ track các field này
    skipNullFields,  // []string — bỏ qua nếu giá trị cũ là nil/empty
    userID,          // int
)
```

**Behavior:**
- So sánh `oldData` vs `newData` chỉ trên `includeFields`
- Field nằm trong `skipNullFields` mà oldVal nil/empty → bỏ qua (tránh log "vừa tạo chưa có giá trị")
- Nếu không có field nào thay đổi → không insert (no-op)
- Numeric so sánh float với tolerance `1e-9` (tránh false diff do type conversion)
- Whitelist `allowedLogTables` chống SQL injection — truyền sai table/fkColumn → error

**Snapshot helper:**

```go
old := history.SnapshotRow(db, "customer", id, history.CustomerIncludeFields)
// ... update ...
history.LogChanges(db, "log_customer", "customer_id", id, old, newData,
    history.CustomerIncludeFields, history.CustomerSkipNullFields, userID)
```

---

## Tracked fields (từ `config.go`)

### log_customer
```go
IncludeFields:  fullname, nickname, phone, category, branch_id,
                source_id, marketing_id, payment_user_id, type_service_id, created_day
SkipNullFields: branch_id, fullname, nickname, phone, type_service_id
```

### log_appointment
```go
IncludeFields:  appointment_date, checkin_date, checkout_date, revenue_id,
                type_service_id, campain_event_id, bookType,
                doctorAppointment_id, doctor_id, content, contentPrivate,
                branch_id, appointment_type_id, service_id, source_id, marketing_id
SkipNullFields: checkin_date, checkout_date, type_service_id
```

### log_payment
```go
IncludeFields:  branch_id, payment_time, payment_of_date,
                amount, note, method, card_type_id, bank_code
SkipNullFields: (none)
```

### log_treatment
```go
IncludeFields:  type_id, category_id, doctor_id, branch_id, created_time,
                percent_success_new, treatment_timeEnd, note, noteNext
SkipNullFields: note, noteNext, treatment_timeEnd
```

### log_order_service_used
```go
IncludeFields:  service_id, quantity, unit_price,
                service_discount, service_discount_type, discount, discount_other,
                expires_at, note, branch_id, created_day,
                appointment_id, source_id, listRevenues, listDoctors, listDirectSales
SkipNullFields: appointment_id, source_id, expires_at, note, listRevenues, listDoctors, listDirectSales
```

---

## Pattern xóa parent — BẮT BUỘC xóa log trước

ON DELETE RESTRICT đồng nghĩa MySQL block xóa parent nếu còn log. Mọi handler/service xóa parent phải làm trong transaction theo thứ tự:

```go
tx.Exec("DELETE FROM log_appointment WHERE appointment_id = ?", id)
tx.Delete(&appt)
```

**Ngoại lệ `customer` (FK SET NULL):** `DeleteWithCleanupTx` (gọi từ `crmService.DeleteCustomerWithPayments`) vẫn xoá các dòng log CŨ (fk NULL thì không biết của ai → rác), rồi `history.LogDelete(tx, "log_customer", …, history.CustomerDeleteFields, …, {"company_id"})` NGAY TRƯỚC `DELETE customer`, cùng transaction. Thứ tự sai (LogDelete trước bước dọn log) ⇒ chính dòng log xoá bị dọn mất. `logCustomerQuery` lấy tên/mã từ snapshot khi `c.id` NULL.

**Trạng thái hiện tại:**

| Parent | Xử lý trong | Đã có |
|---|---|---|
| `customer` | `crm/service/customer_delete.go` `DeleteCustomerWithPayments` → `DeleteWithCleanupTx` — giữ 1 dòng log xoá | ✅ |
| `appointment` | `AppointmentService.Delete`; lịch hẹn kế tiếp khi xoá buổi điều trị (`deleteNextAppointment`, bọc SAVEPOINT — DELETE fail thì hoàn cả log) | ✅ log xoá |
| `payment` | `deletePayment` (giữ log cũ); payment theo `parent_id` khi xoá đơn | ✅ log xoá |
| `treatment_course` | `TreatmentCourseService.beforeDelete` | ✅ log xoá |
| `order_service_used` | xoá đơn (`OrderTreatmentPlanService.Delete`) + bỏ dòng khi sửa đơn (`handleServices`) — `logDeleteOSUs` | ✅ log xoá |

Thứ tự bắt buộc ở mọi chỗ: `DELETE FROM log_x WHERE x_id = ?` (dọn log cũ — trừ payment) → `LogDeleteSnapshot` → DELETE parent. **Xoá khách hàng** vẫn dọn sạch log của lịch hẹn/thanh toán/buổi/dòng dịch vụ của khách (chỉ giữ 1 dòng log xoá khách — user chốt 2026-09-22). Dọn theo `fk IN (…)` KHÔNG bắt được dòng log XOÁ cũ (fk đã NULL) ⇒ `DeleteWithCleanupTx` quét thêm theo snapshot `$.customer_id.from` (`childDeleteLogTables`, quét toàn bảng log — chấp nhận vì xoá khách hiếm). Log thanh toán: `deleteCustomerPayments` dọn log cũ của mọi phiếu trước `deletePayment`, rồi dọn luôn dòng log xoá phiếu vừa ghi (khớp snapshot id).

**Test neo hành vi** (integration, bật lại `foreign_key_checks` sau seed — Suite tắt nên SET NULL không chạy): `crm/service/log_delete_integration_test.go` (6 đường xoá + nhánh SAVEPOINT), `crm/customer/service/delete_log_integration_test.go`, `crm/service/customer_delete_integration_test.go` (bút toán đảo + kỳ khoá sổ huỷ cả lần xoá), `crm/customer/handler/delete_customer_integration_test.go` (422 kèm mã phiếu), `history/log/handler/list_log_integration_test.go` (scope snapshot, vế company trong ON, `customer_id` = `c.id`). Đã chạy mutation 8 biến thể (đảo thứ tự dọn/ghi, bỏ RollbackTo, bỏ vế ON…) — đều đỏ.

**Lưu ý appointment:** Ngoài `log_appointment`, appointment còn bị block bởi `zalo_zns_log` và `order_treatment_plan` (RESTRICT). Thứ tự delete đúng:

```go
tx.Exec("DELETE FROM log_appointment WHERE appointment_id = ?", id)
tx.Exec("DELETE FROM zalo_zns_log WHERE appointment_id = ?", id)
tx.Exec("UPDATE order_service_used SET appointment_id = NULL WHERE appointment_id = ?", id)
tx.Exec("UPDATE order_treatment_plan SET appointment_id = NULL WHERE appointment_id = ?", id)
tx.Exec("UPDATE treatment_course SET appointment_id = NULL WHERE appointment_id = ?", id)
tx.Delete(&appt)
```

---

## Read API — `GET /history/log?_fieldName=<type>`

Handler: `internal/modules/webapp/history/log/handler/handler.go`

`_fieldName` nhận: `customer` | `appointment` | `payment` | `treatment`

Query pattern — mỗi loại JOIN 2 bước để lấy customer:

| `_fieldName` | Join path |
|---|---|
| `customer` | `log_customer` → `customer` trực tiếp |
| `appointment` | `log_appointment` → `appointment` → `customer` |
| `payment` | `log_payment` → `payment` → `customer` |
| `treatment` | `log_treatment` → `treatment_course` → `customer` |

Response: `changed_fields` được parse từ JSON string → object trước khi trả về. Customer fields nest vào object `{id, fullname, nickname}`.

---

## Thêm log table mới — checklist

1. Tạo migration với schema chuẩn (fk_column, action_type, changed_fields, changed_by, changed_at)
2. Thêm vào `allowedLogTables` trong `service.go`
3. Thêm `IncludeFields` + `SkipNullFields` vào `config.go`
4. Gọi `history.LogChanges()` sau mỗi update của parent
5. Thêm case vào `ListLog` handler + viết query JOIN
6. Xoá cứng parent: FK SET NULL + `LogDelete` + cột `company_id` snapshot (giữ audit, mẫu `log_customer`/`log_partners`) — hoặc RESTRICT + xoá log trước (mất audit)

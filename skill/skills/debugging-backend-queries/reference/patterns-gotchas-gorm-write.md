# Gotchas GORM v2 — write

## Mục lục

- `Create(&struct)` với field có tag `default:` — MỌI zero-value bị OMIT khỏi INSERT (CRITICAL)
- `decimal.Decimal` gán thẳng vào `map[string]any` cho `.Table().Create()/.Updates()` — không cần convert tay
- `clause.OnConflict{DoNothing: true}` batch insert — PHẢI dùng typed model, KHÔNG `Table().Create(map)` (HIGH)
- Reload sau upsert — thêm `ORDER BY id ASC` khi WHERE có thể khớp nhiều row (HIGH)
- GORM v2 — `Select(cols)` tường minh: field KHÔNG trong list luôn zero/nil dù DB có data (CRITICAL)
- Tái dùng `*Service` khác module qua constructor(tx) — GORM tự lồng transaction (SAVEPOINT)
- `created_by`/`updated_by` — cột nullable NHƯNG có FK constraint thật, userID=0 ≠ NULL
- Rename struct field → grep riêng `Omit("FieldName")`/whitelist `map[string]bool` (CRITICAL)
- FK `ON DELETE RESTRICT` — mỗi bảng con phải dọn HOẶC chặn, ở `beforeDelete` (CRITICAL)
- `Create(&struct)` có field relation mang PK → CƯỚP bản ghi con của record khác (CRITICAL)
- `INSERT IGNORE` vào bảng xoá MỀM → thêm lại là no-op IM LẶNG (CRITICAL)
- Xoá MỀM cha không kích `ON DELETE SET NULL` → con giữ id cha đã chết (HIGH)
- Handler UPDATE qua `merge*()` bỏ qua nil/`""` → user KHÔNG xoá được dữ liệu đã nhập (CRITICAL)

---


Tách từ `skill/skills/debugging-backend-queries/reference/patterns-gotchas-gorm.md` (2026-08-05, file gốc vượt 350 dòng) — gotcha phần đọc/query giữ nguyên ở file gốc, phần này chỉ Create/Update/reuse-transaction/rename-field.

## `Create(&struct)` với field có tag `default:` — MỌI zero-value bị OMIT khỏi INSERT (CRITICAL)

GORM bỏ cột có `gorm:"default:..."` khỏi câu INSERT khi Go set zero-value, để MySQL tự áp `DEFAULT` → cột ra giá trị mặc định dù code set khác. **KHÔNG chỉ `bool`/`default:true`**: áp cho mọi kiểu — `int` với `default:7` mà set `0`, `string` với `default:'x'` mà set `""`.

Fix, theo thứ tự ưu tiên:
1. **Bỏ `default:` khỏi struct tag Go** khi tầng app LUÔN set giá trị tường minh (giữ `DEFAULT` ở DDL cho đường INSERT không qua GORM). Rẻ nhất, không đổi call site.
2. Insert qua `map[string]any{...}` (ghi tường minh mọi cột) + `clause.OnConflict{DoUpdates: ...}` nếu cần upsert.

Triệu chứng: FE gửi đúng, BE nhận đúng, DB lưu SAI — không lỗi, không log. Ca thật: `gcal_subscription.window_past_days` `default:7`, user chọn cửa sổ `0` ngày (chỉ lịch từ hôm nay) bị lưu thành 7 (2026-09-06, bắt bởi HTTP test khẳng định giá trị đã lưu — test chỉ khẳng định status 200 sẽ KHÔNG thấy). Precedent bool: `omnichat-assignment` availability-online-helper.

## `decimal.Decimal` gán thẳng vào `map[string]any` cho `.Table().Create()/.Updates()` — không cần convert tay

`decimal.Decimal` implement `driver.Valuer` (`Value()` trả string) — gán thẳng field tiền vào `map[string]any{"field": d}` rồi `.Table().Create()/.Updates()` bind đúng cột `decimal(20,2)`, không cần `.String()`/format thủ công.

## `clause.OnConflict{DoNothing: true}` batch insert — PHẢI dùng typed model, KHÔNG `Table().Create(map)` (HIGH)

`tx.Table("name").Create([]map[string]any{...})` với `OnConflict{DoNothing:true}` khiến GORM sinh `ON DUPLICATE KEY UPDATE` RỖNG (không có schema field để chọn cột no-op) → MySQL lỗi 1064. Dùng typed model slice (`tx.Create([]dao.SomeEntity{...})`) — GORM đủ thông tin PK/schema để dựng no-op update hợp lệ. Precedent: `omnichat-search` bulk insert.

## Reload sau upsert — thêm `ORDER BY id ASC` khi WHERE có thể khớp nhiều row (HIGH)

Khi reload record sau upsert có khả năng khớp nhiều row (UNIQUE có cột nullable, hoặc race 2 writer), luôn thêm `.Order("id ASC")` vào query reload — GORM `First()` không có ORDER BY trả về row tùy ý khi ≥2 row khớp WHERE, không đảm bảo lấy đúng row gốc/cũ nhất. Precedent: `omnichat-core` zalo-adapter reload sau upsert.

## GORM v2 — `Select(cols)` tường minh: field KHÔNG trong list luôn zero/nil dù DB có data (CRITICAL)

Struct dest khai báo field với `gorm:"column:x"` nhưng `Select("a, b")` không liệt kê `x`
→ GORM chỉ scan đúng cột được chọn, field `x` giữ nguyên zero-value/nil bất kể DB có giá trị gì.

```go
// ❌ struct có field CommissionDoctorConsultantSnap nhưng Select không liệt kê → luôn nil
var su struct {
    TotalEnd                      decimal.Decimal
    CommissionDoctorConsultantSnap *decimal.Decimal `gorm:"column:commission_doctor_consultant_snap"`
}
tx.Table("order_service_used").Select("listDoctors, COALESCE(totalEnd,0) as totalEnd").Take(&su)
// su.CommissionDoctorConsultantSnap luôn nil dù DB có giá trị thật

// ✅ liệt kê đủ cột cần dùng trong Select
tx.Table("order_service_used").
    Select("listDoctors, COALESCE(totalEnd,0) as totalEnd, commission_doctor_consultant_snap").
    Take(&su)
```

Precedent: `crm/service/payment_commission.go` `handleDoctorConsultantCommission` — thiếu cột snapshot khiến code luôn rơi fallback đọc rate hiện tại thay vì rate khóa lúc lên đơn.

## Tái dùng `*Service` khác module qua constructor(tx) — GORM tự lồng transaction (SAVEPOINT)

Service có constructor kiểu `NewXxxService(db *gorm.DB) *XxxService` (field `DB` nhận thẳng `*gorm.DB`) → module khác gọi được bằng cách truyền `tx` (transaction đang mở) làm tham số, KHÔNG cần refactor gì thêm. Bên trong service dù có gọi `s.DB.Transaction(...)` riêng, GORM v2 tự chạy như SAVEPOINT lồng trong `tx` ngoài — giữ nguyên tính atomic của transaction gốc. Áp dụng khi muốn tái dùng nguyên luồng Create/Update thật (kèm side-effect) từ 1 module khác thay vì viết lại logic. Precedent: `client/customer-a/order` gọi `crmServices.NewOrderPlanService(tx)`.

## `created_by`/`updated_by` — cột nullable NHƯNG có FK constraint thật, userID=0 ≠ NULL

Mọi `BaseModel`/`BaseModelWithCode` hook (`internal/models/base.go`) set blame qua `tx.Get("userID")` — nếu code gọi truyền `userID=0` (chưa resolve được ai) thay vì thật sự KHÔNG set, GORM insert giá trị `0` (không phải NULL) vào cột có FK → lỗi 1452 ngay lúc INSERT. Dùng helper `blameID(tx)` (trả `(nil, false)` khi userID<=0) thay vì tự inline `tx.Get("userID")`.

## Rename struct field → grep riêng `Omit("FieldName")`/whitelist `map[string]bool` (CRITICAL)

`go build` không bắt lỗi vì đây là string literal, không phải reference field:
- `.Omit("OldFieldName", ...)` — field bị loại khỏi Omit âm thầm, cột nặng bị kéo lại vào query
- `map[string]bool{"old_column": true}` (whitelist update) — field mới bị reject âm thầm khi FE gửi

Rename field → `grep -rn "OldFieldName\|old_column"` toàn repo, không chỉ tin `go build` pass.

## FK `ON DELETE RESTRICT` — mỗi bảng con phải dọn HOẶC chặn, ở `beforeDelete` (CRITICAL)

Mỗi bảng con có FK `RESTRICT` tới bảng cha PHẢI chọn 1 trong 2 tại **`beforeDelete`**: xoá/unlink nó, HOẶC chặn xoá bằng `errs.NewLocalizedError("<key>", args...)` kèm message dịch được. Không để lỗi 1451 nổi lên client.

Cleanup/hoàn tác đặt ở `afterDelete` là **dead code**: `tx.Delete(row)` fail trước → rollback → hàm không bao giờ chạy tới, và record không xoá được vĩnh viễn.

Thêm bảng con RESTRICT mới → phải cập nhật `beforeDelete` của bảng cha. `go build` không bắt được.

Liệt kê bảng con + rule:
```sql
SELECT k.TABLE_NAME, k.COLUMN_NAME, rc.DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
JOIN information_schema.KEY_COLUMN_USAGE k
  ON k.CONSTRAINT_NAME = rc.CONSTRAINT_NAME AND k.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
WHERE rc.CONSTRAINT_SCHEMA = DATABASE() AND k.REFERENCED_TABLE_NAME = '<bảng cha>';
```

## `Create(&struct)` có field relation mang PK → CƯỚP bản ghi con của record khác (CRITICAL)

`Create(&item)` tự save association. Child có PK ≠ 0 → `onConflictOption` (`callbacks/associations.go`) sinh `ON DUPLICATE KEY UPDATE <fk> = VALUES(<fk>)` → **re-point child đang thuộc record khác sang record mới**.

Handler bind JSON vào struct có field relation → PHẢI `item.<Relation> = nil` cùng chỗ reset `ID`/`Code`/`Version`. Nghiêm trọng nhất ở luồng **CLONE**: payload mang nguyên id con của bản gốc, clone xong bản gốc mất sạch con (ca thật: `service.ServiceMaterial`).

Cùng gốc, chiều update: key relation trong `Updates(map)` không tra được field (relation không nằm trong `FieldsByDBName`) → GORM dùng key làm **TÊN CỘT** → `Error 1054 Unknown column`.

## Handler UPDATE qua `merge*()` bỏ qua nil/`""` → user KHÔNG xoá được dữ liệu đã nhập (CRITICAL)

Mọi handler UPDATE đi qua hàm `merge*(dst, src)` kiểu `if src.X != nil { dst.X = src.X }` / `if src.X != "" { ... }` đều có lỗ **im lặng**: user xoá trắng 1 ô rồi Lưu → BE trả 200, DB giữ nguyên giá trị cũ. Không lỗi, không cảnh báo — chỉ ghi đè được, không xoá được. Gốc: `ShouldBindJSON` không phân biệt `null` với key vắng mặt (đều ra nil/`""`), nên merge buộc phải bỏ qua rỗng để khỏi xoá oan payload partial.

Điều kiện xoá phải là **key CÓ MẶT trong raw body** với giá trị `null` hoặc `""` — KHÔNG phải "giá trị rỗng sau unmarshal". Nhờ vậy client gửi payload partial (mobile BFF truyền map rỗng) không bị xoá oan:

```go
bodyBytes, _ := io.ReadAll(c.Request.Body)
var rawMap map[string]json.RawMessage
json.Unmarshal(bodyBytes, &rawMap)
explicitClears := map[string]bool{}
for k, v := range rawMap {
    if s := string(v); s == "null" || s == `""` { explicitClears[k] = true }
}
```

Apply SAU merge qua `map[string]func(*Entity)` (xem `checkinClearableFields`, `crm/appointment/service/service.go`); field có cascade (`checkin_date`) giữ nhánh xử lý riêng. Precedent: `CustomerCheckin` — trước khi vá, mọi sinh hiệu nhập sai là vĩnh viễn sai vì `explicitNulls` chỉ áp cho `checkin_date`/`checkout_date`.

Còn CHƯA rà cùng lớp lỗi: `config_call_center/handler/helpers.go`, `omni_channel/handler/livechat.go`, `crm/service/order_treatment_plan.go`.

## `INSERT IGNORE` vào bảng xoá MỀM → thêm lại là no-op IM LẶNG (CRITICAL)

Bảng xoá **mềm** (`left_at`/`deleted_at`): dòng cũ **vẫn tồn tại** ⇒ `INSERT IGNORE` bỏ qua, trả `nil`, KHÔNG ghi gì. Thêm lại một người đã rời báo thành công mà không có gì xảy ra.
Mọi đường "thêm lại" phải `UPDATE … SET left_at = NULL` **TRƯỚC** `INSERT IGNORE` — bổ sung, không phải thay thế.
Test "đã là thành viên chưa" BẮT BUỘC lọc `left_at IS NULL`: đếm số dòng trần cho kết quả giống nhau ở cả hai trạng thái ⇒ test xanh vô nghĩa.
Có **N đường** cùng làm một việc ⇒ lập bảng đối chiếu N đường; không đường nào tự thấy mình thiếu.

## Xoá MỀM cha không kích `ON DELETE SET NULL` → con giữ id cha đã chết (HIGH)

`ON DELETE SET NULL/CASCADE` chỉ chạy khi DELETE thật. Cha xoá mềm (`deleted_at`) hoặc huỷ (`status=cancelled`) ⇒ con vẫn trỏ id cha chết.
Logic "gắn lại con cho cha mới" phải coi cha chết như NULL — chỉ lọc `parent_id IS NULL` là bỏ sót im lặng:
`LEFT JOIN cha ON cha.id = con.parent_id WHERE con.parent_id IS NULL OR cha.id IS NULL OR cha.deleted_at IS NOT NULL OR cha.status = 'cancelled'`.
Khuôn: `episodelink.adoptOrphanDocuments`.
